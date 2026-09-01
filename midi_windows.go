//go:build windows

package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

var (
	modWinmm              = syscall.NewLazyDLL("winmm.dll")
	procMidiInGetNumDevs  = modWinmm.NewProc("midiInGetNumDevs")
	procMidiInGetDevCapsW = modWinmm.NewProc("midiInGetDevCapsW")
	procMidiInOpen        = modWinmm.NewProc("midiInOpen")
	procMidiInStart       = modWinmm.NewProc("midiInStart")
	procMidiInStop        = modWinmm.NewProc("midiInStop")
	procMidiInClose       = modWinmm.NewProc("midiInClose")
)

const (
	mimData          = 0x3C3
	mimMoreData      = 0x3CC
	callbackFunction = 0x00030000
)

type midiInCapsW struct {
	mid           uint16
	pid           uint16
	driverVersion uint32
	pname         [32]uint16
	support       uint32
}

func findDefaultMidiDevice() string {
	r, _, _ := procMidiInGetNumDevs.Call()
	numDevs := int(r)
	if numDevs > 0 {
		return "0"
	}
	return ""
}

func startMidiListener(devPath string, engine *AudioEngine, tuning float64, harmonics []float64, attack, decay, sustain, release float64, gain float64, panMode string) error {
	r, _, _ := procMidiInGetNumDevs.Call()
	numDevs := int(r)
	if numDevs == 0 {
		return fmt.Errorf("no MIDI input devices found on Windows")
	}

	devID := 0
	if devPath != "" && devPath != "auto" {
		if id, err := strconv.Atoi(devPath); err == nil && id >= 0 && id < numDevs {
			devID = id
		}
	}

	var caps midiInCapsW
	procMidiInGetDevCapsW.Call(uintptr(devID), uintptr(unsafe.Pointer(&caps)), unsafe.Sizeof(caps))
	devName := syscall.UTF16ToString(caps.pname[:])
	fmt.Fprintf(os.Stderr, "Listening for MIDI events on Windows device #%d: %s...\n", devID, devName)

	cb := syscall.NewCallback(func(hMidiIn, wMsg, dwInstance, dwParam1, dwParam2 uintptr) uintptr {
		if wMsg == mimData || wMsg == mimMoreData {
			status := byte(dwParam1 & 0xFF)
			note := byte((dwParam1 >> 8) & 0xFF)
			vel := byte((dwParam1 >> 16) & 0xFF)

			statusType := status & 0xF0
			if statusType == 0x90 { // Note On
				if vel > 0 {
					engine.AddMidiNote(note, vel, tuning, harmonics, attack, decay, sustain, release, gain, panMode)
				} else {
					engine.ReleaseMidiNote(note)
				}
			} else if statusType == 0x80 { // Note Off
				engine.ReleaseMidiNote(note)
			}
		}
		return 0
	})

	var hMidiIn uintptr
	res, _, _ := procMidiInOpen.Call(
		uintptr(unsafe.Pointer(&hMidiIn)),
		uintptr(devID),
		cb,
		0,
		callbackFunction,
	)
	if res != 0 {
		return fmt.Errorf("midiInOpen failed with error code: %d", res)
	}

	resStart, _, _ := procMidiInStart.Call(hMidiIn)
	if resStart != 0 {
		procMidiInClose.Call(hMidiIn)
		return fmt.Errorf("midiInStart failed with error code: %d", resStart)
	}

	return nil
}
