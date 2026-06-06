package encoder

import (
	"fmt"
	"io"
)

// Encoder defines the interface for audio encoders.
type Encoder interface {
	WritePCM(pcm []float32) error
	Flush() error
	Close()
}

// NewEncoder creates a new audio encoder based on the specified codec.
func NewEncoder(codec string, w io.Writer, channels int, sampleRate int, bitrate int) (Encoder, error) {
	switch codec {
	case "mp3", "":
		return NewLameEncoder(w, channels, sampleRate, bitrate)
	case "aac":
		return NewAACEncoder(w, channels, sampleRate, bitrate)
	case "opus":
		return NewOpusEncoder(w, channels, sampleRate, bitrate)
	case "vorbis":
		return NewVorbisEncoder(w, channels, sampleRate, bitrate)
	default:
		return nil, fmt.Errorf("unsupported codec: %s", codec)
	}
}
