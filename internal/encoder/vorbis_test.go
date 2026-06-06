package encoder

import (
	"bytes"
	"testing"
)

func TestVorbisEncoderPanic(t *testing.T) {
	var buf bytes.Buffer

	enc, err := NewEncoder("vorbis", &buf, 2, 44100, 128)
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}
	defer enc.Close()

	pcm := make([]float32, 2048)
	err = enc.WritePCM(pcm)
	if err != nil {
		t.Fatalf("Failed to write PCM: %v", err)
	}
	err = enc.Flush()
	if err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Encoder did not output any bytes")
	} else {
		t.Logf("Successfully encoded %d bytes", buf.Len())
	}
}
