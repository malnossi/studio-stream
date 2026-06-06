package encoder

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/hraban/opus"
	"mccoy.space/g/ogg"
)

type OpusEncoder struct {
	opusEnc    *opus.Encoder
	oggEnc     *ogg.Encoder
	granule    int64
	pcmBuf     []float32
	channels   int
	sampleRate int
	frameSize  int
}

func NewOpusEncoder(w io.Writer, channels int, sampleRate int, bitrate int) (*OpusEncoder, error) {
	// Opus works best at 48000, but supports 8, 12, 16, 24, 48kHz.
	// For simplicity, we assume sampleRate is one of these, usually 48000.
	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("opus error: %w", err)
	}
	enc.SetBitrate(bitrate * 1000)

	oggEnc := ogg.NewEncoder(1, w)

	opusHead := make([]byte, 19)
	copy(opusHead[0:8], []byte("OpusHead"))
	opusHead[8] = 1                              // version
	opusHead[9] = byte(channels)
	binary.LittleEndian.PutUint16(opusHead[10:12], 384) // preskip
	binary.LittleEndian.PutUint32(opusHead[12:16], uint32(sampleRate))
	binary.LittleEndian.PutUint16(opusHead[16:18], 0) // output gain
	opusHead[18] = 0                             // mapping family

	if err := oggEnc.EncodeBOS(0, [][]byte{opusHead}); err != nil {
		return nil, fmt.Errorf("ogg bos error: %w", err)
	}

	vendor := "StudioStream"
	opusTags := make([]byte, 8+4+len(vendor)+4)
	copy(opusTags[0:8], []byte("OpusTags"))
	binary.LittleEndian.PutUint32(opusTags[8:12], uint32(len(vendor)))
	copy(opusTags[12:12+len(vendor)], []byte(vendor))
	binary.LittleEndian.PutUint32(opusTags[12+len(vendor):], 0)

	if err := oggEnc.Encode(0, [][]byte{opusTags}); err != nil {
		return nil, fmt.Errorf("ogg tags error: %w", err)
	}

	// Frame size of 20ms
	frameSize := (sampleRate * 20) / 1000

	return &OpusEncoder{
		opusEnc:    enc,
		oggEnc:     oggEnc,
		channels:   channels,
		sampleRate: sampleRate,
		pcmBuf:     make([]float32, 0, frameSize*channels*2),
		frameSize:  frameSize,
	}, nil
}

func (e *OpusEncoder) WritePCM(pcm []float32) error {
	e.pcmBuf = append(e.pcmBuf, pcm...)

	samplesPerFrame := e.frameSize * e.channels
	for len(e.pcmBuf) >= samplesPerFrame {
		frame := e.pcmBuf[:samplesPerFrame]

		// Encode 20ms frame
		out := make([]byte, 4000) // max opus frame size
		n, err := e.opusEnc.EncodeFloat32(frame, out)
		if err != nil {
			return err
		}

		// Update granule: Opus granule is always counted at 48kHz.
		// A 20ms frame is exactly 960 samples at 48kHz.
		e.granule += 960

		if err := e.oggEnc.Encode(e.granule, [][]byte{out[:n]}); err != nil {
			return err
		}

		e.pcmBuf = e.pcmBuf[samplesPerFrame:]
	}
	return nil
}

func (e *OpusEncoder) Flush() error {
	return nil
}

func (e *OpusEncoder) Close() {
	if len(e.pcmBuf) > 0 {
		// Zero pad the last frame and write
		samplesPerFrame := e.frameSize * e.channels
		padded := make([]float32, samplesPerFrame)
		copy(padded, e.pcmBuf)
		
		out := make([]byte, 4000)
		if n, err := e.opusEnc.EncodeFloat32(padded, out); err == nil {
			e.granule += 960
			e.oggEnc.EncodeEOS(e.granule, [][]byte{out[:n]})
		}
	} else {
		e.oggEnc.EncodeEOS(e.granule, [][]byte{})
	}
}
