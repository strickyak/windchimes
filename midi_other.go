//go:build !linux && !windows && !darwin

package main

import (
	"fmt"
)

func findDefaultMidiDevice() string {
	return ""
}

func startMidiListener(devPath string, engine *AudioEngine, tuning float64, harmonics []float64, attack, decay, sustain, release float64, gain float64, panMode string) error {
	return fmt.Errorf("MIDI input is not supported on this platform")
}
