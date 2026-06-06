package streamer

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

func TestStreamerHandshake(t *testing.T) {
	// Start a mock local TCP listener on an ephemeral port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start mock Icecast listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Run mock Icecast server loop in background
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read request headers until double CRLF
		reader := bufio.NewReader(conn)
		var headers []string
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			headers = append(headers, line)
		}

		// Validate headers
		var hasSourceLine, hasAuthHeader bool
		for _, h := range headers {
			if strings.HasPrefix(h, "SOURCE /live") {
				hasSourceLine = true
			}
			if strings.HasPrefix(h, "Authorization: Basic") {
				hasAuthHeader = true
			}
		}

		if !hasSourceLine || !hasAuthHeader {
			conn.Write([]byte("HTTP/1.0 401 Unauthorized\r\n\r\n"))
			return
		}

		// Acknowledge connection with ICY 200 OK
		conn.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))

		// Read incoming MP3 payload data
		buf := make([]byte, 256)
		_, _ = conn.Read(buf)
	}()

	// Instantiate and connect our streamer to the mock server
	cfg := Config{
		Host:        serverAddr,
		MountPoint:  "/live",
		Username:    "source",
		Password:    "hackme",
		StreamName:  "Test Channel",
		Description: "Testing Icecast handshake",
		Genre:       "Test",
		URL:         "http://localhost",
		Public:      false,
		Bitrate:     128,
		SampleRate:  44100,
		Channels:    2,
	}

	client, err := Connect(cfg)
	if err != nil {
		t.Fatalf("Failed to establish stream connection: %v", err)
	}
	defer client.Close()

	// Write simulated MP3 frames
	payload := []byte("SIMULATED-MP3-FRAME-BYTES-HERE")
	n, err := client.Write(payload)
	if err != nil {
		t.Fatalf("Failed writing payload: %v", err)
	}
	if n != len(payload) {
		t.Errorf("Bytes written mismatch: expected %d, got %d", len(payload), n)
	}

	if client.BytesSent() != uint64(len(payload)) {
		t.Errorf("BytesSent statistics mismatch: expected %d, got %d", len(payload), client.BytesSent())
	}
}
