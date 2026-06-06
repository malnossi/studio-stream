package encoder

import (
	"fmt"
	"io"
	"unsafe"

	"github.com/gen2brain/aac-go"
)

// AACEncoder wraps the gen2brain/aac-go encoder to output ADTS frames.
type AACEncoder struct {
	enc    *aac.Encoder
	writer io.Writer
	intBuf []int16
	pw     *io.PipeWriter
}

// NewAACEncoder creates a new AAC encoder.
func NewAACEncoder(w io.Writer, channels int, sampleRate int, bitrate int) (*AACEncoder, error) {
	opts := &aac.Options{
		SampleRate:  sampleRate,
		NumChannels: channels,
		BitRate:     bitrate * 1000,
	}

	enc, err := aac.NewEncoder(w, opts)
	if err != nil {
		return nil, fmt.Errorf("aac init error: %w", err)
	}

	pr, pw := io.Pipe()

	// Run the encoder blockingly in a background goroutine.
	// It will read from 'pr' until 'pw' is closed.
	go func() {
		_ = enc.Encode(pr)
	}()

	return &AACEncoder{
		enc:    enc,
		writer: w,
		intBuf: make([]int16, 0),
		pw:     pw,
	}, nil
}

// WritePCM converts float32 to int16 PCM and encodes to AAC.
func (e *AACEncoder) WritePCM(pcm []float32) error {
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

	if len(byteBuf) > 0 {
		if _, err := e.pw.Write(byteBuf); err != nil {
			return err
		}
	}
	return nil
}

// Flush flushes the encoder.
func (e *AACEncoder) Flush() error {
	return nil
}

// Close closes the encoder.
func (e *AACEncoder) Close() {
	e.pw.Close()
	e.enc.Close()
}
