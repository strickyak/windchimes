package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"gitlab.com/gomidi/midi/v2/smf"
)

type midiSchedEvent struct {
	microSec int64
	channel  uint8
	note     byte
	vel      byte
	isNoteOn bool
	program  uint8
	pan      float32
	volume   float32
}

func playMidiFile(
	filename string,
	engine *AudioEngine,
	speed float64,
	tuning float64,
	fallbackHarmonics []float64,
	fallbackAttack, fallbackDecay, fallbackSustain, fallbackRelease, fallbackGain float64,
	fallbackPanMode string,
	midiAuto bool,
	onDone func(),
) error {
	var events []midiSchedEvent

	// Track channel states during parsing
	var channelProgram [16]uint8
	var channelPan [16]float32
	var channelVol [16]float32
	var channelExpr [16]float32

	for ch := 0; ch < 16; ch++ {
		channelProgram[ch] = 0 // Default Acoustic Piano (or set by ProgramChange)
		channelPan[ch] = 0.5   // Center pan
		channelVol[ch] = 1.0   // Full volume
		channelExpr[ch] = 1.0  // Full expression
	}

	reader := smf.ReadTracks(filename)
	reader.Do(func(ev smf.TrackEvent) {
		var ch, prog, ctrl, val, key, vel uint8

		if ev.Message.GetProgramChange(&ch, &prog) {
			if ch < 16 {
				channelProgram[ch] = prog
			}
		} else if ev.Message.GetControlChange(&ch, &ctrl, &val) {
			if ch < 16 {
				switch ctrl {
				case 10: // Pan (0 = left, 64 = center, 127 = right)
					channelPan[ch] = float32(val) / 127.0
				case 7: // Main Volume
					channelVol[ch] = float32(val) / 127.0
				case 11: // Expression
					channelExpr[ch] = float32(val) / 127.0
				}
			}
		} else if ev.Message.GetNoteStart(&ch, &key, &vel) {
			chIdx := ch % 16
			events = append(events, midiSchedEvent{
				microSec: ev.AbsMicroSeconds,
				channel:  ch,
				note:     key,
				vel:      vel,
				isNoteOn: true,
				program:  channelProgram[chIdx],
				pan:      channelPan[chIdx],
				volume:   channelVol[chIdx] * channelExpr[chIdx],
			})
		} else if ev.Message.GetNoteEnd(&ch, &key) {
			events = append(events, midiSchedEvent{
				microSec: ev.AbsMicroSeconds,
				channel:  ch,
				note:     key,
				vel:      0,
				isNoteOn: false,
			})
		}
	})

	if err := reader.Error(); err != nil {
		return fmt.Errorf("error reading MIDI file %s: %w", filename, err)
	}

	if len(events) == 0 {
		return fmt.Errorf("no note events found in MIDI file %s", filename)
	}

	// Sort events by microsecond timestamp
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].microSec == events[j].microSec {
			// Process note offs before note ons at the same timestamp
			return !events[i].isNoteOn && events[j].isNoteOn
		}
		return events[i].microSec < events[j].microSec
	})

	go func() {
		if onDone != nil {
			defer onDone()
		}

		fmt.Fprintf(os.Stderr, "Playing MIDI file %s (%d events, speed: %.1f%%, auto-voices: %v)...\n",
			filename, len(events), speed, midiAuto)

		startTime := time.Now()
		speedFactor := 100.0 / speed

		for _, ev := range events {
			targetDelay := time.Duration(float64(ev.microSec) * speedFactor * float64(time.Microsecond))
			targetTime := startTime.Add(targetDelay)

			sleepDur := time.Until(targetTime)
			if sleepDur > 0 {
				time.Sleep(sleepDur)
			}

			if ev.isNoteOn {
				if midiAuto {
					preset := GetGMPreset(ev.program)
					pan := ev.pan
					velGain := (float32(ev.vel) / 127.0) * ev.volume * preset.Gain * float32(fallbackGain)

					leftGain := velGain * (1.0 - pan)
					rightGain := velGain * pan

					engine.ReleaseMidiChannelNote(ev.channel, ev.note)
					v := newVoiceAdvanced(
						ev.channel,
						ev.note,
						tuning,
						preset.Harmonics,
						preset.Attack,
						preset.Decay,
						preset.Sustain,
						preset.Release,
						leftGain,
						rightGain,
						preset.Chorus,
						preset.VibratoRate,
						preset.VibratoDepth,
					)
					engine.AddVoice(v)
				} else {
					engine.AddMidiNote(ev.note, ev.vel, tuning, fallbackHarmonics, fallbackAttack, fallbackDecay, fallbackSustain, fallbackRelease, fallbackGain, fallbackPanMode)
				}
			} else {
				engine.ReleaseMidiChannelNote(ev.channel, ev.note)
			}
		}

		// Allow release tail to finish
		time.Sleep(600 * time.Millisecond)
		fmt.Fprintf(os.Stderr, "Finished playing MIDI file: %s\n", filename)
	}()

	return nil
}
