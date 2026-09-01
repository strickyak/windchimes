//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func findDefaultMidiDevice() string {
	matches, err := filepath.Glob("/dev/snd/midi*")
	if err == nil && len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func startMidiListener(devPath string, engine *AudioEngine, tuning float64, harmonics []float64, attack, decay, sustain, release float64, gain float64, panMode string) error {
	f, err := os.Open(devPath)
	if err != nil {
		return fmt.Errorf("failed to open MIDI device %s: %w", devPath, err)
	}

	go func() {
		defer f.Close()
		fmt.Fprintf(os.Stderr, "Listening for MIDI events on Linux device %s...\n", devPath)

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
						if vel > 0 {
							engine.AddMidiNote(note, vel, tuning, harmonics, attack, decay, sustain, release, gain, panMode)
						} else {
							// Note On with velocity 0 is Note Off
							engine.ReleaseMidiNote(note)
						}
					} else if statusType == 0x80 { // Note Off
						note := dataBuf[0]
						engine.ReleaseMidiNote(note)
					}

					dataCount = 0
				}
			}
		}
	}()

	return nil
}
