//go:build windows || darwin

package main

import (
	"fmt"
	"io"
	"time"

	"github.com/ebitengine/oto/v3"
)

type AudioReader struct {
	engine    *AudioEngine
	isPlaying func() bool
	buf       [blockSize * 4]byte
	bufOffset int
	bufLen    int
}

func (r *AudioReader) Read(p []byte) (int, error) {
	total := 0
	for total < len(p) {
		if r.bufLen > 0 {
			n := copy(p[total:], r.buf[r.bufOffset:r.bufOffset+r.bufLen])
			r.bufOffset += n
			r.bufLen -= n
			total += n
			if r.bufLen == 0 {
				r.bufOffset = 0
			}
			continue
		}

		active := r.engine.RenderBlock(r.buf[:], r.isPlaying)
		if !active {
			if total > 0 {
				return total, nil
			}
			return 0, io.EOF
		}
		r.bufOffset = 0
		r.bufLen = len(r.buf)
	}
	return total, nil
}

func (e *AudioEngine) PlayNative(isPlaying func() bool) error {
	op := &oto.NewContextOptions{
		SampleRate:   int(sampleRate),
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   20 * time.Millisecond,
	}

	otoCtx, readyChan, err := oto.NewContext(op)
	if err != nil {
		return fmt.Errorf("failed to initialize native audio: %w", err)
	}
	<-readyChan

	reader := &AudioReader{
		engine:    e,
		isPlaying: isPlaying,
	}

	player := otoCtx.NewPlayer(reader)
	player.SetBufferSize(int(sampleRate) * 4 * 20 / 1000)
	player.Play()

	for player.IsPlaying() {
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}
