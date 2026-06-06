package streamer

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Config holds the details for establishing a connection to an Icecast server.
type Config struct {
	Host        string // e.g. "localhost:8000"
	UseTLS      bool   // whether to connect via HTTPS/TLS
	MountPoint  string // e.g. "/live"
	Username    string // e.g. "source"
	Password    string // password for source client
	StreamName  string
	Description string
	Genre       string
	URL         string
	Public      bool
	Bitrate     int    // informative kbps
	SampleRate  int    // informative Hz
	Channels    int    // informative channels count
	Codec       string // "mp3", "aac", "opus", "vorbis"
}

// Streamer manages the Icecast TCP connection and implements io.WriteCloser.
type Streamer struct {
	conn      net.Conn
	mu        sync.Mutex
	bytesSent uint64
}

// Connect dials the Icecast server and performs the ICY/SOURCE handshake.
func Connect(cfg Config) (*Streamer, error) {
	// Normalize mount point to start with /
	mount := cfg.MountPoint
	if !strings.HasPrefix(mount, "/") {
		mount = "/" + mount
	}

	// Default username for Icecast source client is "source"
	username := cfg.Username
	if username == "" {
		username = "source"
	}

	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + cfg.Password))

	pub := 0
	if cfg.Public {
		pub = 1
	}

	contentType := "audio/mpeg"
	switch cfg.Codec {
	case "aac":
		contentType = "audio/aac"
	case "opus", "vorbis":
		contentType = "application/ogg"
	}

	// Connect to Icecast port
	var conn net.Conn
	var err error
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	if cfg.UseTLS {
		// Import "crypto/tls" at top
		conn, err = tls.DialWithDialer(dialer, "tcp", cfg.Host, nil)
	} else {
		conn, err = dialer.Dial("tcp", cfg.Host)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	audioInfo := fmt.Sprintf("samplerate=%d;channels=%d;bitrate=%d", cfg.SampleRate, cfg.Channels, cfg.Bitrate)

	// Format HTTP SOURCE handshake
	req := fmt.Sprintf(
		"SOURCE %s HTTP/1.0\r\n"+
			"Authorization: Basic %s\r\n"+
			"User-Agent: StudioStream/1.0\r\n"+
			"Content-Type: %s\r\n"+
			"Ice-Name: %s\r\n"+
			"Ice-Public: %d\r\n"+
			"Ice-Description: %s\r\n"+
			"Ice-Genre: %s\r\n"+
			"Ice-URL: %s\r\n"+
			"Ice-Audio-Info: %s\r\n"+
			"\r\n",
		mount,
		auth,
		contentType,
		cfg.StreamName,
		pub,
		cfg.Description,
		cfg.Genre,
		cfg.URL,
		audioInfo,
	)

	// Set deadline for handshake write
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		conn.Close()
		return nil, err
	}

	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send handshake request: %w", err)
	}

	// Read response
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		conn.Close()
		return nil, err
	}

	respBuf := make([]byte, 1024)
	n, err := conn.Read(respBuf)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read handshake response: %w", err)
	}

	// Reset deadlines for infinite streaming loop
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, err
	}

	respStr := string(respBuf[:n])
	if !strings.Contains(respStr, "200 OK") && !strings.Contains(respStr, "ICY 200 OK") {
		conn.Close()
		lines := strings.Split(respStr, "\r\n")
		statusLine := "handshake failed"
		if len(lines) > 0 {
			statusLine = lines[0]
		}
		return nil, fmt.Errorf("server handshake failed: %s", statusLine)
	}

	return &Streamer{
		conn: conn,
	}, nil
}

// Write writes encoded MP3 chunks to the TCP socket with a write deadline.
func (s *Streamer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return 0, fmt.Errorf("not connected")
	}

	// Add 5-second write deadline to detect hung network sockets
	if err := s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return 0, err
	}

	n, err := s.conn.Write(p)
	s.bytesSent += uint64(n)
	if err != nil {
		return n, fmt.Errorf("socket write error: %w", err)
	}
	return n, nil
}

// BytesSent returns the total number of bytes sent through the connection.
func (s *Streamer) BytesSent() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bytesSent
}

// Close closes the TCP connection cleanly.
func (s *Streamer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		err := s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}
