package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const sampleRate = 48000.0
const twoPi = 2.0 * math.Pi
const chanBufSize = 128

// SineWave produces a channel of 48000 float32 samples per second
// for a given frequency sine wave ranging -1.0 to +1.0.
func SineWave(freq float64) <-chan float32 {
	out := make(chan float32, chanBufSize)
	go func() {
		var phase float64
		phaseIncrement := twoPi * freq / sampleRate
		for {
			out <- float32(math.Sin(phase))
			phase += phaseIncrement
			if phase >= twoPi {
				phase -= twoPi
			}
		}
	}()
	return out
}

// HarmonicWave produces a channel of 48000 float32 samples per second
// for a given fundamental frequency, combining multiple harmonics based on
// the provided relative amplitudes. The amplitudes are normalized to sum to 1.0.
func HarmonicWave(freq float64, harmonics []float64) <-chan float32 {
	out := make(chan float32, chanBufSize)

	// Normalize harmonics so their sum is 1.0
	var sum float64
	for _, h := range harmonics {
		sum += h
	}
	normalized := make([]float64, len(harmonics))
	if sum > 0 {
		for i, h := range harmonics {
			normalized[i] = h / sum
		}
	}

	go func() {
		var phase float64
		phaseIncrement := twoPi * freq / sampleRate
		for {
			var sampleVal float64
			for i, amp := range normalized {
				harmonicFactor := float64(i + 1)
				sampleVal += amp * math.Sin(phase*harmonicFactor)
			}
			out <- float32(sampleVal)

			phase += phaseIncrement
			if phase >= twoPi {
				phase -= twoPi
			}
		}
	}()
	return out
}

// Envelope applies an ADSR envelope to an input channel.
func Envelope(in <-chan float32, attack, decay, sustainLevel, sustainTime, release float64) <-chan float32 {
	out := make(chan float32, chanBufSize)
	go func() {
		defer close(out)

		attackSamples := int(attack * sampleRate)
		decaySamples := int(decay * sampleRate)
		sustainSamples := int(sustainTime * sampleRate)
		releaseSamples := int(release * sampleRate)

		processPhase := func(samples int, startLevel, endLevel float64) {
			for i := 0; i < samples; i++ {
				val, ok := <-in
				if !ok {
					return
				}
				t := float64(i) / float64(samples)
				multiplier := startLevel + t*(endLevel-startLevel)
				out <- val * float32(multiplier)
			}
		}

		// A, D, S, R phases
		processPhase(attackSamples, 0.0, 1.0)
		processPhase(decaySamples, 1.0, sustainLevel)
		processPhase(sustainSamples, sustainLevel, sustainLevel)
		processPhase(releaseSamples, sustainLevel, 0.0)
	}()
	return out
}

// InteractiveHarmonicNote produces a channel of audio samples with an ADSR envelope
// that sustains until stopFlag is set to true (key-up event), at which point it releases.
func InteractiveHarmonicNote(freq float64, harmonics []float64, attack, decay, sustainLevel, release float64, stopFlag *atomic.Bool) <-chan float32 {
	out := make(chan float32, chanBufSize)
	go func() {
		defer close(out)

		// Normalize harmonics
		var sum float64
		for _, h := range harmonics {
			sum += h
		}
		normalized := make([]float64, len(harmonics))
		if sum > 0 {
			for i, h := range harmonics {
				normalized[i] = h / sum
			}
		}

		attackSamples := int(attack * sampleRate)
		decaySamples := int(decay * sampleRate)
		releaseSamples := int(release * sampleRate)
		if attackSamples < 1 {
			attackSamples = 1
		}
		if decaySamples < 1 {
			decaySamples = 1
		}
		if releaseSamples < 1 {
			releaseSamples = 1
		}

		var phase float64
		phaseIncrement := twoPi * freq / sampleRate

		nextWaveSample := func() float32 {
			var sampleVal float64
			for i, amp := range normalized {
				harmonicFactor := float64(i + 1)
				sampleVal += amp * math.Sin(phase*harmonicFactor)
			}
			phase += phaseIncrement
			if phase >= twoPi {
				phase -= twoPi
			}
			return float32(sampleVal)
		}

		currentLevel := 0.0
		released := false

		// Attack phase
		for i := 0; i < attackSamples; i++ {
			if stopFlag != nil && stopFlag.Load() {
				released = true
				break
			}
			sample := nextWaveSample()
			t := float64(i) / float64(attackSamples)
			currentLevel = t
			out <- sample * float32(currentLevel)
		}

		// Decay phase
		if !released {
			for i := 0; i < decaySamples; i++ {
				if stopFlag != nil && stopFlag.Load() {
					released = true
					break
				}
				sample := nextWaveSample()
				t := float64(i) / float64(decaySamples)
				currentLevel = 1.0 + t*(sustainLevel-1.0)
				out <- sample * float32(currentLevel)
			}
		}

		// Sustain phase: holds sustainLevel until stopFlag becomes true
		if !released {
			currentLevel = sustainLevel
			for {
				if stopFlag != nil && stopFlag.Load() {
					break
				}
				sample := nextWaveSample()
				out <- sample * float32(currentLevel)
			}
		}

		// Release phase: smooth ramp from currentLevel down to 0.0
		startReleaseLevel := currentLevel
		for i := 0; i < releaseSamples; i++ {
			sample := nextWaveSample()
			t := float64(i) / float64(releaseSamples)
			level := startReleaseLevel * (1.0 - t)
			out <- sample * float32(level)
		}
	}()
	return out
}

func findDefaultMidiDevice() string {
	matches, err := filepath.Glob("/dev/snd/midi*")
	if err == nil && len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func startMidiListener(devPath string, addChan chan<- MixInput, tuning float64, harmonics []float64, attack, decay, sustain, release float64, gain float64, panMode string) error {
	f, err := os.Open(devPath)
	if err != nil {
		return fmt.Errorf("failed to open MIDI device %s: %w", devPath, err)
	}

	go func() {
		defer f.Close()
		fmt.Fprintf(os.Stderr, "Listening for MIDI events on %s...\n", devPath)

		var mu sync.Mutex
		activeVoices := make(map[byte]*atomic.Bool)

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		buf := make([]byte, 256)
		var runningStatus byte
		var expectedData int
		var dataBuf [2]byte
		var dataCount int

		for {
			n, err := f.Read(buf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "MIDI read error: %v\n", err)
				return
			}

			for i := 0; i < n; i++ {
				b := buf[i]

				// Real-time messages (0xF8 - 0xFF) can arrive at any time and do not reset running status
				if b >= 0xF8 {
					continue
				}

				// Status byte
				if b >= 0x80 {
					statusType := b & 0xF0
					if b >= 0xF0 {
						// System Common message
						runningStatus = 0
						if b == 0xF1 || b == 0xF3 {
							expectedData = 1
							runningStatus = b
						} else if b == 0xF2 {
							expectedData = 2
							runningStatus = b
						} else {
							expectedData = 0
						}
					} else {
						// Channel message
						runningStatus = b
						if statusType == 0xC0 || statusType == 0xD0 {
							expectedData = 1
						} else {
							expectedData = 2
						}
					}
					dataCount = 0
					continue
				}

				// Data byte
				if runningStatus == 0 {
					continue
				}

				if dataCount < len(dataBuf) {
					dataBuf[dataCount] = b
					dataCount++
				}

				if dataCount == expectedData {
					statusType := runningStatus & 0xF0

					if statusType == 0x90 { // Note On
						note := dataBuf[0]
						vel := dataBuf[1]

						mu.Lock()
						if oldStop, exists := activeVoices[note]; exists {
							oldStop.Store(true)
							delete(activeVoices, note)
						}

						if vel > 0 {
							stopFlag := new(atomic.Bool)
							activeVoices[note] = stopFlag
							mu.Unlock()

							// Frequency calculation: A4 (note 69) = tuning Hz
							semitones := float64(int(note) - 69)
							freq := tuning * math.Pow(2.0, semitones/12.0)

							voiceChan := InteractiveHarmonicNote(freq, harmonics, attack, decay, sustain, release, stopFlag)

							var pan float32
							switch panMode {
							case "center":
								pan = 0.5
							case "random":
								pan = rng.Float32()
							default: // "spread"
								// Pan based on note range (approx 21 to 108)
								pan = float32(int(note)-21) / float32(108-21)
								if pan < 0.15 {
									pan = 0.15
								} else if pan > 0.85 {
									pan = 0.85
								}
							}

							velGain := (float32(vel) / 127.0) * float32(gain)
							leftGain := velGain * (1.0 - pan)
							rightGain := velGain * pan

							addChan <- MixInput{C: voiceChan, L: leftGain, R: rightGain}
						} else {
							mu.Unlock()
						}
					} else if statusType == 0x80 { // Note Off
						note := dataBuf[0]
						mu.Lock()
						if stopFlag, exists := activeVoices[note]; exists {
							stopFlag.Store(true)
							delete(activeVoices, note)
						}
						mu.Unlock()
					}

					dataCount = 0
				}
			}
		}
	}()

	return nil
}

// MixInput combines an audio channel with left and right volume multipliers.
type MixInput struct {
	C <-chan float32
	L float32
	R float32
}

// Mixer consumes a list of channels and optionally an addChan for dynamically adding new channels.
// It sums them and produces a Left and a Right output channel.
// If runForever is true, the mixer will continue outputting silence when there are no active inputs,
// waiting for new inputs from addChan.
// The mixer loop continues as long as there are active inputs, runForever is true, or addChan is not closed.
func Mixer(inputs []MixInput, addChan <-chan MixInput, runForever bool) (<-chan float32, <-chan float32) {
	left := make(chan float32, chanBufSize)
	right := make(chan float32, chanBufSize)

	go func() {
		defer close(left)
		defer close(right)

		activeInputs := make([]MixInput, len(inputs))
		copy(activeInputs, inputs)

		for len(activeInputs) > 0 || runForever || addChan != nil {
			if addChan != nil {
				for {
					select {
					case newIn, ok := <-addChan:
						if ok {
							activeInputs = append(activeInputs, newIn)
							continue
						} else {
							addChan = nil
						}
					default:
					}
					break
				}
			}

			if len(activeInputs) == 0 {
				left <- 0
				right <- 0
				continue
			}

			var sumL float32
			var sumR float32
			var nextActive []MixInput

			for _, in := range activeInputs {
				val, ok := <-in.C
				if ok {
					sumL += val * in.L
					sumR += val * in.R
					nextActive = append(nextActive, in)
				}
			}
			activeInputs = nextActive

			if len(activeInputs) > 0 || runForever {
				left <- sumL
				right <- sumR
			}
		}
	}()
	return left, right
}

// Output consumes Left and Right channels with 48000 float32 samples each
// and outputs that as stream PCM format (stereo signed 16-bit little-endian)
// with no header, directly to stdout.
func Output(left <-chan float32, right <-chan float32, gain float32) {
	var buf [4]byte
	for {
		l, lOk := <-left
		r, rOk := <-right

		if !lOk && !rOk {
			break // Both channels closed, we are done
		}

		l *= gain
		r *= gain

		// Hard clipping to [-1.0, 1.0] to prevent integer overflow wrapping
		if l > 1.0 {
			l = 1.0
		} else if l < -1.0 {
			l = -1.0
		}
		if r > 1.0 {
			r = 1.0
		} else if r < -1.0 {
			r = -1.0
		}

		// Convert float32 [-1.0, 1.0] to int16 [-32767, 32767]
		l16 := int16(l * 32767.0)
		r16 := int16(r * 32767.0)

		// Pack as stereo 16-bit little-endian
		binary.LittleEndian.PutUint16(buf[0:2], uint16(l16))
		binary.LittleEndian.PutUint16(buf[2:4], uint16(r16))

		// Write to stdout
		os.Stdout.Write(buf[:])
	}
}

func parseFractionalFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "+") {
		parts := strings.SplitN(s, "+", 2)
		a, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			return 0, err
		}

		fracStr := strings.TrimSpace(parts[1])
		if strings.Contains(fracStr, "/") {
			fParts := strings.SplitN(fracStr, "/", 2)
			b, err := strconv.ParseFloat(strings.TrimSpace(fParts[0]), 64)
			if err != nil {
				return 0, err
			}
			c, err := strconv.ParseFloat(strings.TrimSpace(fParts[1]), 64)
			if err != nil {
				return 0, err
			}
			if c == 0 {
				return 0, fmt.Errorf("division by zero in fraction")
			}
			return a + b/c, nil
		}

		b, err := strconv.ParseFloat(fracStr, 64)
		if err != nil {
			return 0, err
		}
		return a + b, nil
	}

	if strings.Contains(s, "/") {
		fParts := strings.SplitN(s, "/", 2)
		b, err := strconv.ParseFloat(strings.TrimSpace(fParts[0]), 64)
		if err != nil {
			return 0, err
		}
		c, err := strconv.ParseFloat(strings.TrimSpace(fParts[1]), 64)
		if err != nil {
			return 0, err
		}
		if c == 0 {
			return 0, fmt.Errorf("division by zero in fraction")
		}
		return b / c, nil
	}

	return strconv.ParseFloat(s, 64)
}

type SongNote struct {
	Note     string
	Duration float64
}

func parseSong(filename string) (map[int][]SongNote, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	channels := make(map[int][]SongNote)
	lines := strings.Split(string(data), "\n")
	reNote := regexp.MustCompile(`\(\s*([A-Ga-g][#b]*\d+|rest)\s*,\s*([0-9.]+)\s*\)`)
	rePrefix := regexp.MustCompile(`^\s*(\d+)\s*:`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		chID := 0
		if m := rePrefix.FindStringSubmatch(line); m != nil {
			chID, _ = strconv.Atoi(m[1])
		}

		matches := reNote.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			dur, err := strconv.ParseFloat(m[2], 64)
			if err != nil {
				return nil, err
			}
			channels[chID] = append(channels[chID], SongNote{
				Note:     m[1],
				Duration: dur,
			})
		}
	}
	return channels, nil
}

func noteToFreq(note string, tuning float64) (float64, error) {
	re := regexp.MustCompile(`^([A-Ga-g])([#b]*)(\d+)$`)
	m := re.FindStringSubmatch(note)
	if m == nil {
		return 0, fmt.Errorf("invalid note format: %s", note)
	}

	letter := m[1][0]
	if letter >= 'a' && letter <= 'z' {
		letter -= 'a' - 'A'
	}

	stdBase := 0.0
	switch letter {
	case 'C': stdBase = 0
	case 'D': stdBase = 2
	case 'E': stdBase = 4
	case 'F': stdBase = 5
	case 'G': stdBase = 7
	case 'A': stdBase = 9
	case 'B': stdBase = 11
	}
	for i := 0; i < len(m[2]); i++ {
		if m[2][i] == '#' {
			stdBase += 1.0
		} else if m[2][i] == 'b' {
			stdBase -= 1.0
		}
	}

	octave, err := strconv.ParseFloat(m[3], 64)
	if err != nil {
		return 0, err
	}

	semitones := (octave - 4.0)*12.0 + (stdBase - 9.0)
	return tuning * math.Pow(2.0, semitones/12.0), nil
}

func RenderChannel(notes []SongNote, attack, decay, sustain, release float64, tuning float64, speed float64, harmonics []float64) <-chan float32 {
	out := make(chan float32, chanBufSize)
	go func() {
		defer close(out)

		var activeEnvelopes []<-chan float32
		noteIdx := 0
		samplesUntilNextNote := 0

		for noteIdx < len(notes) || samplesUntilNextNote > 0 || len(activeEnvelopes) > 0 {
			if samplesUntilNextNote <= 0 && noteIdx < len(notes) {
				sn := notes[noteIdx]
				noteIdx++

				actualDuration := sn.Duration * (100.0 / speed)
				samplesUntilNextNote = int(actualDuration * sampleRate)

				if strings.ToLower(sn.Note) != "rest" {
					freq, err := noteToFreq(sn.Note, tuning)
					if err == nil {
						sTime := actualDuration - attack - decay
						if sTime < 0 {
							sTime = 0
						}
						wave := HarmonicWave(freq, harmonics)
						env := Envelope(wave, attack, decay, sustain, sTime, release)
						activeEnvelopes = append(activeEnvelopes, env)
					} else {
						fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
					}
				}
			}

			var sum float32 = 0
			if len(activeEnvelopes) > 0 {
				var nextActive []<-chan float32
				for _, env := range activeEnvelopes {
					v, ok := <-env
					if ok {
						sum += v
						nextActive = append(nextActive, env)
					}
				}
				activeEnvelopes = nextActive
			}

			out <- sum
			if samplesUntilNextNote > 0 {
				samplesUntilNextNote--
			}
		}
	}()
	return out
}

func parseEvenScale(scaleStr string) ([]float64, error) {
	parts := strings.Split(scaleStr, ",")
	var bases []float64

	octaveOffset := 0.0
	var prevLetter byte = 0

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) == 0 {
			continue
		}

		letter := p[0]
		if letter >= 'a' && letter <= 'z' {
			letter -= 'a' - 'A'
		}

		if prevLetter != 0 && prevLetter > letter {
			octaveOffset += 12.0
		}
		prevLetter = letter

		base := 0.0
		switch letter {
		case 'A':
			base = 0
		case 'B':
			base = 2
		case 'C':
			base = 3
		case 'D':
			base = 5
		case 'E':
			base = 7
		case 'F':
			base = 8
		case 'G':
			base = 10
		default:
			return nil, fmt.Errorf("invalid note letter: %c", letter)
		}

		for i := 1; i < len(p); i++ {
			if p[i] == '#' {
				base += 1.0
			} else if p[i] == 'b' {
				base -= 1.0
			}
		}

		bases = append(bases, base+octaveOffset)
	}
	return bases, nil
}

func main() {
	modeFlag := flag.String("mode", "harm", "Wave mode: 'sine', 'harm', 'wind3', 'song', or 'midi'")
	attackFlag := flag.Float64("a", 0.1, "Attack time in seconds")
	decayFlag := flag.Float64("d", 0.2, "Decay time in seconds")
	sustainFlag := flag.Float64("s", 0.5, "Sustain level (proportion 0.0 to 1.0)")
	releaseFlag := flag.Float64("r", 0.5, "Release time in seconds")
	timeFlag := flag.Float64("t", 1.3, "Total time for attack, decay, and sustain in seconds")
	harmonicFlag := flag.String("h", "8,4,2,1", "Comma-separated float64 harmonic vector")
	ogFlag := flag.Float64("og", 0.3, "Output gain multiplier")
	tuningFlag := flag.Float64("tuning", 440.0, "Frequency of A4 in Hz")
	poissonFlag := flag.String("p", "3", "Arrival time mean and standard deviation (m,d). If d is omitted, defaults to m/3.")
	octavesFlag := flag.String("octaves", "3,4,5", "Comma-separated list of octaves to use in wind3 mode")
	justFlag := flag.String("just", "", "Just tuning ratios, e.g. '1/1,6/5,4/3,3/2,9/5' or 'pentatonic'")
	evenFlag := flag.String("even", "A,C,D,E,G", "Even-tempered scale notes (e.g. A,C,D,E,G or C,E,G,Bb)")
	songFileFlag := flag.String("songfile", "", "Song file to play")
	songSpeedFlag := flag.Float64("songspeed", 100.0, "Speed to play the song as a percentage (100 is normal)")
	midiFlag := flag.String("midi", "", "MIDI raw device path (e.g. '/dev/snd/midiC1D0' or 'auto')")
	midiAttackFlag := flag.Float64("midia", 0.01, "MIDI keyboard attack time in seconds")
	midiDecayFlag := flag.Float64("midid", 0.1, "MIDI keyboard decay time in seconds")
	midiSustainFlag := flag.Float64("midis", 0.8, "MIDI keyboard sustain level (0.0 to 1.0)")
	midiReleaseFlag := flag.Float64("midir", 0.3, "MIDI keyboard release time in seconds")
	midiGainFlag := flag.Float64("midigain", 0.5, "MIDI note volume multiplier")
	midiHarmonicsFlag := flag.String("midih", "", "MIDI harmonics vector (defaults to -h if empty)")
	midiPanFlag := flag.String("midipan", "spread", "MIDI stereo panning mode ('spread', 'center', or 'random')")

	flag.Parse()

	sustainTime := *timeFlag - *attackFlag - *decayFlag
	if sustainTime < 0 {
		sustainTime = 0
	}

	// Parse harmonic vector (used by harm and wind3)
	hStrs := strings.Split(*harmonicFlag, ",")
	var harmonics []float64
	for _, hs := range hStrs {
		h, err := strconv.ParseFloat(strings.TrimSpace(hs), 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid harmonic vector: %v\n", err)
			os.Exit(1)
		}
		harmonics = append(harmonics, h)
	}

	// Parse MIDI harmonics if specified, otherwise default to harmonics
	var midiHarmonics []float64
	if *midiHarmonicsFlag != "" {
		mhStrs := strings.Split(*midiHarmonicsFlag, ",")
		for _, hs := range mhStrs {
			h, err := strconv.ParseFloat(strings.TrimSpace(hs), 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid MIDI harmonic vector: %v\n", err)
				os.Exit(1)
			}
			midiHarmonics = append(midiHarmonics, h)
		}
	} else {
		midiHarmonics = harmonics
	}

	// Parse poisson arrival (used by wind3)
	pStrs := strings.Split(*poissonFlag, ",")
	var meanArrival, stdDevArrival float64
	if len(pStrs) > 0 {
		var err error
		meanArrival, err = strconv.ParseFloat(strings.TrimSpace(pStrs[0]), 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid mean arrival time: %v\n", err)
			os.Exit(1)
		}
		if len(pStrs) > 1 {
			stdDevArrival, err = strconv.ParseFloat(strings.TrimSpace(pStrs[1]), 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid standard deviation for arrival time: %v\n", err)
				os.Exit(1)
			}
		} else {
			stdDevArrival = meanArrival / 3.0
		}
	}

	// Parse octaves (used by wind3)
	oStrs := strings.Split(*octavesFlag, ",")
	var octaves []float64
	for _, osStr := range oStrs {
		o, err := parseFractionalFloat(osStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid octave: %v\n", err)
			os.Exit(1)
		}
		octaves = append(octaves, o)
	}
	if len(octaves) == 0 {
		fmt.Fprintf(os.Stderr, "Must provide at least one octave\n")
		os.Exit(1)
	}

	// Parse just tuning ratios
	var justRatios []float64
	if *justFlag != "" {
		jStr := *justFlag
		if jStr == "pentatonic" {
			jStr = "1/1,6/5,4/3,3/2,9/5"
		}
		parts := strings.Split(jStr, ",")
		for _, part := range parts {
			rParts := strings.Split(strings.TrimSpace(part), "/")
			if len(rParts) != 2 {
				fmt.Fprintf(os.Stderr, "Invalid just ratio format (expected num/den): %s\n", part)
				os.Exit(1)
			}
			num, err1 := strconv.ParseFloat(strings.TrimSpace(rParts[0]), 64)
			den, err2 := strconv.ParseFloat(strings.TrimSpace(rParts[1]), 64)
			if err1 != nil || err2 != nil || den == 0 {
				fmt.Fprintf(os.Stderr, "Invalid just ratio numbers: %s\n", part)
				os.Exit(1)
			}
			justRatios = append(justRatios, num/den)
		}
	}

	// Parse even-tempered scale
	evenBases, err := parseEvenScale(*evenFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid even scale: %v\n", err)
		os.Exit(1)
	}
	if len(evenBases) == 0 {
		fmt.Fprintf(os.Stderr, "Must provide at least one note in even scale\n")
		os.Exit(1)
	}

	var initialInputs []MixInput
	var addChan chan MixInput
	runForever := false

	if *modeFlag == "sine" {
		wave := SineWave(1000.0)
		initialInputs = append(initialInputs, MixInput{Envelope(wave, *attackFlag, *decayFlag, *sustainFlag, sustainTime, *releaseFlag), 1.0, 1.0})
	} else if *modeFlag == "harm" {
		wave := HarmonicWave(1000.0, harmonics)
		initialInputs = append(initialInputs, MixInput{Envelope(wave, *attackFlag, *decayFlag, *sustainFlag, sustainTime, *releaseFlag), 1.0, 1.0})
	} else if *modeFlag == "wind3" {
		runForever = true
		addChan = make(chan MixInput, 100)

		// Start a generator goroutine
		go func() {
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			for {
				o := octaves[rng.Intn(len(octaves))]

				var freq float64
				if len(justRatios) > 0 {
					r := justRatios[rng.Intn(len(justRatios))]
					freq = *tuningFlag * math.Pow(2.0, o-4.0) * r
				} else {
					b := evenBases[rng.Intn(len(evenBases))]
					semitones := (o-4.0)*12.0 + b
					freq = *tuningFlag * math.Pow(2.0, semitones/12.0)
				}

				wave := HarmonicWave(freq, harmonics)
				enveloped := Envelope(wave, *attackFlag, *decayFlag, *sustainFlag, sustainTime, *releaseFlag)

				// Random Right Percentage [0.0, 1.0)
				rightPct := rng.Float32()
				leftPct := 1.0 - rightPct

				addChan <- MixInput{enveloped, leftPct, rightPct}

				sleepSecs := rng.NormFloat64()*stdDevArrival + meanArrival
				if sleepSecs < 0.01 {
					sleepSecs = 0.01
				}
				time.Sleep(time.Duration(sleepSecs * float64(time.Second)))
			}
		}()
	} else if *modeFlag == "midi" {
		runForever = true
		if *midiFlag == "" {
			*midiFlag = "auto"
		}
	} else if *modeFlag == "song" {
		if *songFileFlag == "" {
			fmt.Fprintf(os.Stderr, "Must provide --songfile in song mode\n")
			os.Exit(1)
		}
		channels, err := parseSong(*songFileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing song: %v\n", err)
			os.Exit(1)
		}

		for chID, notes := range channels {
			seqChan := RenderChannel(notes, *attackFlag, *decayFlag, *sustainFlag, *releaseFlag, *tuningFlag, *songSpeedFlag, harmonics)

			rightPct := float32(chID%4)*0.2 + 0.2 // pan 0.2, 0.4, 0.6, 0.8
			if len(channels) == 1 {
				rightPct = 0.5
			}
			leftPct := 1.0 - rightPct

			initialInputs = append(initialInputs, MixInput{seqChan, leftPct, rightPct})
		}
	} else {
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *modeFlag)
		os.Exit(1)
	}

	// Start MIDI listener if requested
	if *midiFlag != "" {
		runForever = true
		if addChan == nil {
			addChan = make(chan MixInput, 100)
		}

		midiDev := *midiFlag
		if midiDev == "auto" {
			midiDev = findDefaultMidiDevice()
			if midiDev == "" {
				fmt.Fprintf(os.Stderr, "No MIDI device found under /dev/snd/midi*\n")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Auto-detected MIDI device: %s\n", midiDev)
		}

		err := startMidiListener(midiDev, addChan, *tuningFlag, midiHarmonics, *midiAttackFlag, *midiDecayFlag, *midiSustainFlag, *midiReleaseFlag, *midiGainFlag, *midiPanFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting MIDI listener: %v\n", err)
			os.Exit(1)
		}
	}

	// 3. Mix it
	left, right := Mixer(initialInputs, addChan, runForever)

	// 4. Output to stdout
	Output(left, right, float32(*ogFlag))
}
