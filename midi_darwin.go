//go:build darwin

package main

import (
	"fmt"
	"os"
	"strconv"
	"unsafe"

	"github.com/ebitengine/purego"
)

type midipacket struct {
	timeStamp uint64
	length    uint16
	data      [256]byte
}

type midipacketlist struct {
	numPackets uint32
	packet     [1]midipacket
}

func findDefaultMidiDevice() string {
	return "0"
}

func startMidiListener(devPath string, engine *AudioEngine, tuning float64, harmonics []float64, attack, decay, sustain, release float64, gain float64, panMode string) error {
	coreMIDI, err := purego.Dlopen("/System/Library/Frameworks/CoreMIDI.framework/CoreMIDI", purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("failed to open CoreMIDI.framework: %w", err)
	}

	coreFoundation, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("failed to open CoreFoundation.framework: %w", err)
	}

	var cfStringCreateWithCString func(alloc uintptr, cStr *byte, encoding uint32) uintptr
	purego.RegisterLibFunc(&cfStringCreateWithCString, coreFoundation, "CFStringCreateWithCString")

	var midiGetNumberOfSources func() int
	purego.RegisterLibFunc(&midiGetNumberOfSources, coreMIDI, "MIDIGetNumberOfSources")

	var midiGetSource func(item int) uintptr
	purego.RegisterLibFunc(&midiGetSource, coreMIDI, "MIDIGetSource")

	var midiClientCreate func(name uintptr, notifyProc uintptr, notifyRefCon uintptr, outClient *uintptr) int32
	purego.RegisterLibFunc(&midiClientCreate, coreMIDI, "MIDIClientCreate")

	var midiInputPortCreate func(client uintptr, name uintptr, readProc uintptr, refCon uintptr, outPort *uintptr) int32
	purego.RegisterLibFunc(&midiInputPortCreate, coreMIDI, "MIDIInputPortCreate")

	var midiPortConnectSource func(port uintptr, source uintptr, connRefCon uintptr) int32
	purego.RegisterLibFunc(&midiPortConnectSource, coreMIDI, "MIDIPortConnectSource")

	numSources := midiGetNumberOfSources()
	if numSources == 0 {
		return fmt.Errorf("no MIDI input sources found on macOS")
	}

	sourceIdx := 0
	if devPath != "" && devPath != "auto" {
		if id, err := strconv.Atoi(devPath); err == nil && id >= 0 && id < numSources {
			sourceIdx = id
		}
	}

	source := midiGetSource(sourceIdx)
	if source == 0 {
		return fmt.Errorf("failed to get MIDI source %d", sourceIdx)
	}

	cClientName := []byte("Windchimes\x00")
	cfClientName := cfStringCreateWithCString(0, &cClientName[0], 0x08000100 /* kCFStringEncodingUTF8 */)

	cPortName := []byte("WindchimesIn\x00")
	cfPortName := cfStringCreateWithCString(0, &cPortName[0], 0x08000100)

	var client uintptr
	status := midiClientCreate(cfClientName, 0, 0, &client)
	if status != 0 {
		return fmt.Errorf("MIDIClientCreate failed: %d", status)
	}

	readCallback := purego.NewCallback(func(pktList *midipacketlist, readRefCon, srcConnRefCon uintptr) uintptr {
		if pktList == nil {
			return 0
		}

		numPkts := int(pktList.numPackets)
		pktPtr := uintptr(unsafe.Pointer(&pktList.packet[0]))

		for i := 0; i < numPkts; i++ {
			pkt := (*midipacket)(unsafe.Pointer(pktPtr))
			length := int(pkt.length)

			for j := 0; j < length; {
				b := pkt.data[j]
				if b >= 0xF8 {
					j++
					continue
				}
				statusType := b & 0xF0
				if statusType == 0x90 && j+2 < length { // Note On
					note := pkt.data[j+1]
					vel := pkt.data[j+2]
					if vel > 0 {
						engine.AddMidiNote(note, vel, tuning, harmonics, attack, decay, sustain, release, gain, panMode)
					} else {
						engine.ReleaseMidiNote(note)
					}
					j += 3
				} else if statusType == 0x80 && j+2 < length { // Note Off
					note := pkt.data[j+1]
					engine.ReleaseMidiNote(note)
					j += 3
				} else if (statusType == 0xC0 || statusType == 0xD0) && j+1 < length {
					j += 2
				} else if (statusType == 0xA0 || statusType == 0xB0 || statusType == 0xE0) && j+2 < length {
					j += 3
				} else {
					j++
				}
			}

			// Advance to next MIDIPacket (aligned)
			pktPtr += unsafe.Offsetof(pkt.data) + uintptr(length)
			if pktPtr%4 != 0 {
				pktPtr += 4 - (pktPtr % 4)
			}
		}
		return 0
	})

	var port uintptr
	status = midiInputPortCreate(client, cfPortName, readCallback, 0, &port)
	if status != 0 {
		return fmt.Errorf("MIDIInputPortCreate failed: %d", status)
	}

	status = midiPortConnectSource(port, source, 0)
	if status != 0 {
		return fmt.Errorf("MIDIPortConnectSource failed: %d", status)
	}

	fmt.Fprintf(os.Stderr, "Listening for MIDI events on macOS CoreMIDI source #%d...\n", sourceIdx)
	return nil
}
