//go:build linux && (386 || arm || mips || mipsle)

package main

import (
	"os"
	"os/exec"
)

func (e *AudioEngine) PlayNative(runForever bool) error {
	cmd := exec.Command("aplay", "-B", "20000", "-F", "5000", "-f", "S16_LE", "-c", "2", "-r", "48000")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return err
	}

	var buf [blockSize * 4]byte
	for {
		active := e.RenderBlock(buf[:], runForever)
		if !active && !runForever {
			break
		}
		_, err := stdin.Write(buf[:])
		if err != nil {
			break
		}
	}
	stdin.Close()
	return cmd.Wait()
}
