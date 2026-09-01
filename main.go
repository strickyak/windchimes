package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const sampleRate = 48000.0
const twoPi = 2.0 * math.Pi
const blockSize = 128 // 128 stereo frames = 2.67ms per block for ultra-low latency

const (
	stateAttack = iota
	stateDecay
	stateSustain
	stateRelease
	stateDone
)

func normalizeHarmonics(harmonics []float64) []float64 {
	var sum float64
	for _, h := range harmonics {
		sum += h
	}
	normalized := make([]float64, len(harmonics))
	if sum > 0 {
		for i, h := range harmonics {
			normalized[i] = h / sum
		}
	} else if len(harmonics) > 0 {
		normalized[0] = 1.0
	}
	return normalized
}

func clampInt16(val int32) int16 {
	if val > 32767 {
		return 32767
	}
	if val < -32767 {
		return -32767
	}
	return int16(val)
}

type Voice struct {
	freq           float64
	phase          float64
	phaseIncrement float64
	harmonics      []float64

	attackSamples  int
	decaySamples   int
	sustainSamples int // > 0 for fixed duration; 0 for interactive sustain until released
	releaseSamples int
	sustainLevel   float64

	state             int
	stateSample       int
	currentLevel      float64
	releaseStartLevel float64
	released          bool

	leftGain  float32
	rightGain float32

	midiNote byte
	isMidi   bool
}

func newVoice(freq float64, harmonics []float64, attack, decay, sustainLevel, sustainTime, release float64, leftGain, rightGain float32, midiNote byte, isMidi bool) *Voice {
	attackSamples := int(attack * sampleRate)
	decaySamples := int(decay * sampleRate)
	releaseSamples := int(release * sampleRate)
	sustainSamples := 0
	if sustainTime > 0 {
		sustainSamples = int(sustainTime * sampleRate)
	}

	if attackSamples < 1 {
		attackSamples = 1
	}
	if decaySamples < 1 {
		decaySamples = 1
	}
	if releaseSamples < 1 {
		releaseSamples = 1
	}

	return &Voice{
		freq:           freq,
		phaseIncrement: twoPi * freq / sampleRate,
		harmonics:      normalizeHarmonics(harmonics),
		attackSamples:  attackSamples,
		decaySamples:   decaySamples,
		sustainSamples: sustainSamples,
		releaseSamples: releaseSamples,
		sustainLevel:   sustainLevel,
		state:          stateAttack,
		leftGain:       leftGain,
		rightGain:      rightGain,
		midiNote:       midiNote,
		isMidi:         isMidi,
	}
}

func (v *Voice) NextSample() (float32, bool) {
	switch v.state {
	case stateAttack:
		if v.released {
			v.state = stateRelease
			v.stateSample = 0
			v.releaseStartLevel = v.currentLevel
		} else {
			if v.attackSamples > 0 {
				v.currentLevel = float64(v.stateSample) / float64(v.attackSamples)
			} else {
				v.currentLevel = 1.0
			}
			v.stateSample++
			if v.stateSample >= v.attackSamples {
				v.state = stateDecay
				v.stateSample = 0
				v.currentLevel = 1.0
			}
		}

	case stateDecay:
		if v.released {
			v.state = stateRelease
			v.stateSample = 0
			v.releaseStartLevel = v.currentLevel
		} else {
			if v.decaySamples > 0 {
				t := float64(v.stateSample) / float64(v.decaySamples)
				v.currentLevel = 1.0 + t*(v.sustainLevel-1.0)
			} else {
				v.currentLevel = v.sustainLevel
			}
			v.stateSample++
			if v.stateSample >= v.decaySamples {
				v.state = stateSustain
				v.stateSample = 0
				v.currentLevel = v.sustainLevel
			}
		}

	case stateSustain:
		if v.released {
			v.state = stateRelease
			v.stateSample = 0
			v.releaseStartLevel = v.currentLevel
		} else if !v.isMidi {
			v.currentLevel = v.sustainLevel
			if v.sustainSamples > 0 {
				v.stateSample++
				if v.stateSample >= v.sustainSamples {
					v.state = stateRelease
					v.stateSample = 0
					v.releaseStartLevel = v.currentLevel
				}
			} else {
				v.state = stateRelease
				v.stateSample = 0
				v.releaseStartLevel = v.currentLevel
			}
		} else {
			v.currentLevel = v.sustainLevel
		}

	case stateRelease:
		if v.releaseSamples > 0 {
			t := float64(v.stateSample) / float64(v.releaseSamples)
			v.currentLevel = v.releaseStartLevel * (1.0 - t)
			if v.currentLevel < 0 {
				v.currentLevel = 0
			}
		} else {
			v.currentLevel = 0
		}
		v.stateSample++
		if v.stateSample >= v.releaseSamples {
			v.state = stateDone
			return 0, false
		}

	case stateDone:
		return 0, false
	}

	var waveVal float64
	for i, amp := range v.harmonics {
		waveVal += amp * math.Sin(v.phase*float64(i+1))
	}
	v.phase += v.phaseIncrement
	if v.phase >= twoPi {
		v.phase -= twoPi
	}

	return float32(waveVal * v.currentLevel), true
}

type AudioEngine struct {
	mu     sync.Mutex
	voices []*Voice
	gain   float32
}

func NewAudioEngine(gain float32) *AudioEngine {
	return &AudioEngine{
		gain: gain,
	}
}

func (e *AudioEngine) AddVoice(v *Voice) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.voices = append(e.voices, v)
}

func (e *AudioEngine) AddMidiNote(note byte, vel byte, tuning float64, harmonics []float64, attack, decay, sustain, release, gain float64, panMode string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Release any currently sustaining instance of this note
	for _, v := range e.voices {
		if v.isMidi && v.midiNote == note && !v.released {
			v.released = true
		}
	}

	if vel == 0 {
		return
	}

	semitones := float64(int(note) - 69)
	freq := tuning * math.Pow(2.0, semitones/12.0)

	var pan float32
	switch panMode {
	case "center":
		pan = 0.5
	case "random":
		pan = rand.Float32()
	default: // "spread"
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

	v := newVoice(freq, harmonics, attack, decay, sustain, 0, release, leftGain, rightGain, note, true)
	e.voices = append(e.voices, v)
}

func (e *AudioEngine) ReleaseMidiNote(note byte) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, v := range e.voices {
		if v.isMidi && v.midiNote == note && !v.released {
			v.released = true
		}
	}
}

func (e *AudioEngine) RenderBlock(buf []byte, runForever bool) bool {
	for i := range buf {
		buf[i] = 0
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.voices) == 0 {
		return runForever
	}

	var remainingVoices []*Voice
	voiceScratch := make([]float32, blockSize)

	for _, v := range e.voices {
		active := false
		for i := 0; i < blockSize; i++ {
			sample, ok := v.NextSample()
			if ok {
				active = true
				voiceScratch[i] = sample
			} else {
				voiceScratch[i] = 0
			}
		}

		if active {
			remainingVoices = append(remainingVoices, v)
			lScale := v.leftGain * e.gain * 32767.0
			rScale := v.rightGain * e.gain * 32767.0

			for i := 0; i < blockSize; i++ {
				s := voiceScratch[i]
				if s == 0 {
					continue
				}
				offset := i * 4
				curL := int16(binary.LittleEndian.Uint16(buf[offset : offset+2]))
				curR := int16(binary.LittleEndian.Uint16(buf[offset+2 : offset+4]))

				newL := clampInt16(int32(curL) + int32(s*lScale))
				newR := clampInt16(int32(curR) + int32(s*rScale))

				binary.LittleEndian.PutUint16(buf[offset:offset+2], uint16(newL))
				binary.LittleEndian.PutUint16(buf[offset+2:offset+4], uint16(newR))
			}
		}
	}
	e.voices = remainingVoices
	return len(e.voices) > 0 || runForever
}

func (e *AudioEngine) RenderLoopToStdout(runForever bool) {
	var buf [blockSize * 4]byte
	for {
		active := e.RenderBlock(buf[:], runForever)
		if !active && !runForever {
			break
		}
		_, err := os.Stdout.Write(buf[:])
		if err != nil {
			break
		}
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
	outFlag := flag.String("out", "speakers", "Audio output target: 'speakers' (native direct playback) or 'stdout' (PCM byte stream)")

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

	engine := NewAudioEngine(float32(*ogFlag))
	runForever := false

	if *modeFlag == "sine" {
		v := newVoice(1000.0, []float64{1.0}, *attackFlag, *decayFlag, *sustainFlag, sustainTime, *releaseFlag, 1.0, 1.0, 0, false)
		engine.AddVoice(v)
	} else if *modeFlag == "harm" {
		v := newVoice(1000.0, harmonics, *attackFlag, *decayFlag, *sustainFlag, sustainTime, *releaseFlag, 1.0, 1.0, 0, false)
		engine.AddVoice(v)
	} else if *modeFlag == "wind3" {
		runForever = true

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

				rightPct := rng.Float32()
				leftPct := 1.0 - rightPct

				v := newVoice(freq, harmonics, *attackFlag, *decayFlag, *sustainFlag, sustainTime, *releaseFlag, leftPct, rightPct, 0, false)
				engine.AddVoice(v)

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
			chNotes := notes
			chNum := chID
			go func() {
				rightPct := float32(chNum%4)*0.2 + 0.2
				if len(channels) == 1 {
					rightPct = 0.5
				}
				leftPct := 1.0 - rightPct

				for _, sn := range chNotes {
					actualDur := sn.Duration * (100.0 / *songSpeedFlag)
					if strings.ToLower(sn.Note) != "rest" {
						freq, err := noteToFreq(sn.Note, *tuningFlag)
						if err == nil {
							sTime := actualDur - *attackFlag - *decayFlag
							if sTime < 0 {
								sTime = 0
							}
							v := newVoice(freq, harmonics, *attackFlag, *decayFlag, *sustainFlag, sTime, *releaseFlag, leftPct, rightPct, 0, false)
							engine.AddVoice(v)
						}
					}
					time.Sleep(time.Duration(actualDur * float64(time.Second)))
				}
			}()
		}
	} else {
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *modeFlag)
		os.Exit(1)
	}

	// Start MIDI listener if requested
	if *midiFlag != "" {
		runForever = true
		midiDev := *midiFlag
		if midiDev == "auto" {
			midiDev = findDefaultMidiDevice()
			if midiDev == "" {
				fmt.Fprintf(os.Stderr, "No MIDI device found under /dev/snd/midi*\n")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Auto-detected MIDI device: %s\n", midiDev)
		}

		err := startMidiListener(midiDev, engine, *tuningFlag, midiHarmonics, *midiAttackFlag, *midiDecayFlag, *midiSustainFlag, *midiReleaseFlag, *midiGainFlag, *midiPanFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting MIDI listener: %v\n", err)
			os.Exit(1)
		}
	}

	if *outFlag == "stdout" {
		engine.RenderLoopToStdout(runForever)
	} else {
		err := engine.PlayNative(runForever)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Audio playback error: %v\nFalling back to stdout...\n", err)
			engine.RenderLoopToStdout(runForever)
		}
	}
}
