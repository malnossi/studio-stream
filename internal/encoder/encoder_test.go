package encoder

import (
	"bytes"
	"testing"
)

func TestEncoder(t *testing.T) {
	var buf bytes.Buffer

	// Initialize LAME encoder (2 channels, 44.1kHz, 128kbps)
	enc, err := NewEncoder("mp3", &buf, 2, 44100, 128)
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}
	defer enc.Close()

	// Write 1024 frames of silent stereo audio (2048 float32 samples)
	pcm := make([]float32, 2048)
	err = enc.WritePCM(pcm)
	if err != nil {
		t.Fatalf("Failed to write PCM: %v", err)
	}

	// Flush internal encoder buffers
	err = enc.Flush()
	if err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Verify that MP3 frames were indeed output
	if buf.Len() == 0 {
		t.Error("Encoder did not output any MP3 bytes")
	} else {
		t.Logf("Successfully encoded %d bytes of MP3 data", buf.Len())
	}
}
