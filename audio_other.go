//go:build !linux && !windows && !darwin

package main

import (
	"fmt"
)

func (e *AudioEngine) PlayNative(runForever bool) error {
	return fmt.Errorf("native audio playback is not supported on this platform, please use -out stdout")
}
