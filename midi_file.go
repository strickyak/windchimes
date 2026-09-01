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
	note     byte
	vel      byte
	isNoteOn bool
}

func playMidiFile(
	filename string,
	engine *AudioEngine,
	speed float64,
	tuning float64,
	harmonics []float64,
	attack, decay, sustain, release, gain float64,
	panMode string,
	onDone func(),
) error {
	var events []midiSchedEvent

	reader := smf.ReadTracks(filename)
	reader.Do(func(ev smf.TrackEvent) {
		var ch, key, vel uint8
		if ev.Message.GetNoteStart(&ch, &key, &vel) {
			events = append(events, midiSchedEvent{
				microSec: ev.AbsMicroSeconds,
				note:     key,
				vel:      vel,
				isNoteOn: true,
			})
		} else if ev.Message.GetNoteEnd(&ch, &key) {
			events = append(events, midiSchedEvent{
				microSec: ev.AbsMicroSeconds,
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

		fmt.Fprintf(os.Stderr, "Playing MIDI file %s (%d events, speed: %.1f%%)...\n", filename, len(events), speed)

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
				engine.AddMidiNote(ev.note, ev.vel, tuning, harmonics, attack, decay, sustain, release, gain, panMode)
			} else {
				engine.ReleaseMidiNote(ev.note)
			}
		}

		// Allow release tail to finish
		time.Sleep(time.Duration(release * float64(time.Second)))
		fmt.Fprintf(os.Stderr, "Finished playing MIDI file: %s\n", filename)
	}()

	return nil
}
