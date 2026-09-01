//go:build linux && (amd64 || arm64 || riscv64)

package main

import (
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"github.com/ebitengine/purego"
)

func (e *AudioEngine) PlayNative(isPlaying func() bool) error {
	err := e.playALSA(isPlaying)
	if err == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Direct ALSA load failed (%v), falling back to background aplay...\n", err)
	return e.playAplayFallback(isPlaying)
}

func (e *AudioEngine) playALSA(isPlaying func() bool) error {
	asound, err := purego.Dlopen("libasound.so.2", purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("could not dlopen libasound.so.2: %w", err)
	}

	var snd_pcm_open func(pcmOut *uintptr, name *byte, stream int, mode int) int
	purego.RegisterLibFunc(&snd_pcm_open, asound, "snd_pcm_open")

	var snd_pcm_set_params func(pcm uintptr, format int, access int, channels int, rate int, soft_resample int, latency_us int) int
	purego.RegisterLibFunc(&snd_pcm_set_params, asound, "snd_pcm_set_params")

	var snd_pcm_writei func(pcm uintptr, buffer unsafe.Pointer, size int) int
	purego.RegisterLibFunc(&snd_pcm_writei, asound, "snd_pcm_writei")

	var snd_pcm_recover func(pcm uintptr, err int, silent int) int
	purego.RegisterLibFunc(&snd_pcm_recover, asound, "snd_pcm_recover")

	var snd_pcm_close func(pcm uintptr) int
	purego.RegisterLibFunc(&snd_pcm_close, asound, "snd_pcm_close")

	cDefault := []byte("default\x00")
	var pcm uintptr
	ret := snd_pcm_open(&pcm, &cDefault[0], 0, 0)
	if ret < 0 {
		return fmt.Errorf("snd_pcm_open failed with code: %d", ret)
	}
	defer snd_pcm_close(pcm)

	// SND_PCM_FORMAT_S16_LE = 2, SND_PCM_ACCESS_RW_INTERLEAVED = 3
	// rate = 48000, channels = 2, soft_resample = 1, latency_us = 20000 (20ms)
	ret = snd_pcm_set_params(pcm, 2, 3, 2, int(sampleRate), 1, 20000)
	if ret < 0 {
		return fmt.Errorf("snd_pcm_set_params failed with code: %d", ret)
	}

	var buf [blockSize * 4]byte
	for {
		active := e.RenderBlock(buf[:], isPlaying)
		if !active {
			break
		}

		frames := blockSize
		ptr := unsafe.Pointer(&buf[0])
		for frames > 0 {
			n := snd_pcm_writei(pcm, ptr, frames)
			if n < 0 {
				n = snd_pcm_recover(pcm, n, 1)
			}
			if n < 0 {
				return fmt.Errorf("snd_pcm_writei error: %d", n)
			}
			if n > 0 {
				frames -= n
				ptr = unsafe.Pointer(uintptr(ptr) + uintptr(n*4))
			}
		}
	}

	return nil
}

func (e *AudioEngine) playAplayFallback(isPlaying func() bool) error {
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
		active := e.RenderBlock(buf[:], isPlaying)
		if !active {
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
