package encoder

import (
	"fmt"
	"io"
	"unsafe"

	"github.com/viert/go-lame"
)

// LameEncoder wraps the viert/go-lame MP3 encoder.
type LameEncoder struct {
	lameEnc  *lame.Encoder
	intBuf   []int16
	channels int
}

// NewLameEncoder creates a LAME encoder writing to the given writer.
func NewLameEncoder(w io.Writer, channels int, sampleRate int, bitrate int) (*LameEncoder, error) {
	enc := lame.NewEncoder(w)

	if err := enc.SetNumChannels(channels); err != nil {
		enc.Close()
		return nil, fmt.Errorf("lame set channels error: %w", err)
	}
	if err := enc.SetInSamplerate(sampleRate); err != nil {
		enc.Close()
		return nil, fmt.Errorf("lame set sample rate error: %w", err)
	}
	if err := enc.SetBrate(bitrate); err != nil {
		enc.Close()
		return nil, fmt.Errorf("lame set bitrate error: %w", err)
	}
	if err := enc.SetQuality(5); err != nil {
		enc.Close()
		return nil, fmt.Errorf("lame set quality error: %w", err)
	}

	return &LameEncoder{
		lameEnc:  enc,
		intBuf:   make([]int16, 0),
		channels: channels,
	}, nil
}

// WritePCM converts float32 audio samples to 16-bit PCM and encodes them.
func (e *LameEncoder) WritePCM(pcm []float32) error {
	if len(pcm) == 0 {
		return nil
	}

	requiredSize := len(pcm)
	if cap(e.intBuf) < requiredSize {
		e.intBuf = make([]int16, requiredSize)
	} else {
		e.intBuf = e.intBuf[:requiredSize]
	}

	for i, val := range pcm {
		e.intBuf[i] = int16(max(-1.0, min(1.0, val)) * 32767.0)
	}

	var byteBuf []byte
	if len(e.intBuf) > 0 {
		byteBuf = unsafe.Slice((*byte)(unsafe.Pointer(&e.intBuf[0])), len(e.intBuf)*2)
	}

	_, err := e.lameEnc.Write(byteBuf)
	return err
}

// Flush flushes the encoder buffers to the underlying writer.
func (e *LameEncoder) Flush() error {
	_, err := e.lameEnc.Flush()
	return err
}

// Close flushes and releases CGo LAME encoder resources.
func (e *LameEncoder) Close() {
	e.lameEnc.Close()
}
