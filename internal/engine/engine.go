package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"studio-stream/internal/audio"
	"studio-stream/internal/encoder"
	"studio-stream/internal/streamer"
)

// EngineState represents the current state of the streaming engine.
type EngineState string

const (
	StateDisconnected EngineState = "disconnected"
	StateConnecting   EngineState = "connecting"
	StateConnected    EngineState = "connected"
	StateReconnecting EngineState = "reconnecting"
)

// EngineStats represents real-time metrics of the active stream.
type EngineStats struct {
	State     EngineState `json:"state"`
	BytesSent uint64      `json:"bytesSent"`
	Uptime    float64     `json:"uptime"` // in seconds
}

// Engine orchestrates audio capture, encoding, and streaming.
type Engine struct {
	state         EngineState
	stateMu       sync.RWMutex
	recorder      *audio.Recorder
	encoder       encoder.Encoder
	streamer      *streamer.Streamer
	pcmChan       chan *[]float32
	pcmPool       *sync.Pool
	cancelFunc    context.CancelFunc
	recWg         sync.WaitGroup
	encWg         sync.WaitGroup
	config        streamer.Config
	devIndex      int
	onStateChange func(EngineState, string)
	startTime     time.Time
}

// NewEngine creates and returns a new Engine instance.
func NewEngine(onStateChange func(EngineState, string)) *Engine {
	return &Engine{
		state:         StateDisconnected,
		onStateChange: onStateChange,
		pcmPool: &sync.Pool{
			New: func() interface{} {
				// Allocate a buffer large enough for stereo 2048 frames (4096 samples)
				buf := make([]float32, 4096)
				return &buf
			},
		},
	}
}

// StartStream begins capturing audio and streaming it to Icecast.
func (e *Engine) StartStream(devIndex int, cfg streamer.Config) error {
	e.stateMu.Lock()
	if e.state != StateDisconnected {
		e.stateMu.Unlock()
		return fmt.Errorf("engine is already running (state: %s)", e.state)
	}
	// Opus only supports 48000 Hz in this app context. Force override to prevent streaming errors.
	if cfg.Codec == "opus" && cfg.SampleRate != 48000 {
		cfg.SampleRate = 48000
	}
	e.state = StateConnecting
	e.config = cfg
	e.devIndex = devIndex
	e.stateMu.Unlock()

	e.onStateChange(StateConnecting, "Connecting to Icecast server...")

	// Connect to Icecast
	streamClient, err := streamer.Connect(cfg)
	if err != nil {
		e.stateMu.Lock()
		e.state = StateDisconnected
		e.stateMu.Unlock()
		e.onStateChange(StateDisconnected, fmt.Sprintf("Icecast connection failed: %v", err))
		return err
	}

	// Initialize encoder
	enc, err := encoder.NewEncoder(cfg.Codec, streamClient, cfg.Channels, cfg.SampleRate, cfg.Bitrate)
	if err != nil {
		streamClient.Close()
		e.stateMu.Lock()
		e.state = StateDisconnected
		e.stateMu.Unlock()
		e.onStateChange(StateDisconnected, fmt.Sprintf("LAME encoder setup failed: %v", err))
		return err
	}

	// Create pcm transmission channel (size 100 serves as a safety buffer)
	e.pcmChan = make(chan *[]float32, 100)
	framesPerBuffer := 1024

	rec, err := audio.NewRecorder(devIndex, cfg.Channels, cfg.SampleRate, framesPerBuffer, e.pcmChan, e.pcmPool)
	if err != nil {
		enc.Close()
		streamClient.Close()
		e.stateMu.Lock()
		e.state = StateDisconnected
		e.stateMu.Unlock()
		e.onStateChange(StateDisconnected, fmt.Sprintf("Audio device setup failed: %v", err))
		return err
	}

	e.stateMu.Lock()
	e.streamer = streamClient
	e.encoder = enc
	e.recorder = rec
	e.state = StateConnected
	e.startTime = time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	e.cancelFunc = cancel
	e.stateMu.Unlock()

	e.onStateChange(StateConnected, "Connected. Broadcasting live!")

	// Start recorder and encoder routines
	e.recWg.Add(1)
	go func() {
		defer e.recWg.Done()
		if err := rec.Start(ctx); err != nil {
			e.handleDisconnect(ctx, err.Error())
		}
	}()

	e.encWg.Add(1)
	go e.encoderLoop(ctx)

	return nil
}

// StopStream halts the audio stream and releases all resources cleanly.
func (e *Engine) StopStream() {
	e.stateMu.Lock()
	if e.state == StateDisconnected {
		e.stateMu.Unlock()
		return
	}

	// Terminate background routines via context cancellation
	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	// Transition state to avoid new entries
	e.state = StateDisconnected
	e.stateMu.Unlock()

	e.onStateChange(StateDisconnected, "Stopping stream and disconnecting...")

	// 1. Wait for audio recorder to stop sending data to pcmChan
	e.recWg.Wait()

	// 2. Safe to close the channel because no more writers are active
	if e.pcmChan != nil {
		close(e.pcmChan)
	}

	// 3. Wait for the encoder loop to finish processing/draining the channel
	e.encWg.Wait()

	// Clean up structs
	e.stateMu.Lock()
	if e.encoder != nil {
		e.encoder.Flush()
		e.encoder.Close()
		e.encoder = nil
	}
	if e.streamer != nil {
		e.streamer.Close()
		e.streamer = nil
	}
	e.recorder = nil

	// Recycle any lingering frames in channel
	if e.pcmChan != nil {
		for pcm := range e.pcmChan {
			e.pcmPool.Put(pcm)
		}
		e.pcmChan = nil
	}
	e.startTime = time.Time{}
	e.stateMu.Unlock()

	e.onStateChange(StateDisconnected, "Disconnected")
}

// GetLevels fetches the real-time input volume RMS levels.
func (e *Engine) GetLevels() (float64, float64) {
	e.stateMu.RLock()
	rec := e.recorder
	e.stateMu.RUnlock()

	if rec != nil {
		return rec.GetLevels()
	}
	return 0, 0
}

// GetStats yields the current connection status and network statistics.
func (e *Engine) GetStats() EngineStats {
	e.stateMu.RLock()
	defer e.stateMu.RUnlock()

	var bytes uint64
	if e.streamer != nil {
		bytes = e.streamer.BytesSent()
	}

	var uptime float64
	if e.state == StateConnected && !e.startTime.IsZero() {
		uptime = time.Since(e.startTime).Seconds()
	}

	return EngineStats{
		State:     e.state,
		BytesSent: bytes,
		Uptime:    uptime,
	}
}

func (e *Engine) encoderLoop(ctx context.Context) {
	defer e.encWg.Done()

	for pcm := range e.pcmChan {
		if ctx.Err() != nil {
			e.pcmPool.Put(pcm)
			return
		}

		e.stateMu.RLock()
		enc := e.encoder
		currentState := e.state
		e.stateMu.RUnlock()

		// Discard audio blocks when reconnecting to keep the broadcast live
		if currentState == StateReconnecting || enc == nil {
			e.pcmPool.Put(pcm)
			continue
		}

		err := enc.WritePCM(*pcm)
		e.pcmPool.Put(pcm)

		if err != nil {
			e.handleDisconnect(ctx, err.Error())
		}
	}
}

func (e *Engine) handleDisconnect(ctx context.Context, errMsg string) {
	e.stateMu.Lock()
	if e.state == StateReconnecting || e.state == StateDisconnected {
		e.stateMu.Unlock()
		return
	}
	e.state = StateReconnecting
	e.stateMu.Unlock()

	e.onStateChange(StateReconnecting, fmt.Sprintf("Stream error: %s. Reconnecting...", errMsg))

	go e.reconnectLoop(ctx)
}

func (e *Engine) reconnectLoop(ctx context.Context) {
	backoff := 2 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			e.stateMu.RLock()
			currentState := e.state
			e.stateMu.RUnlock()

			if currentState == StateDisconnected {
				return
			}

			e.onStateChange(StateReconnecting, "Attempting to reconnect to Icecast...")

			newStreamer, err := streamer.Connect(e.config)
			if err != nil {
				e.onStateChange(StateReconnecting, fmt.Sprintf("Reconnection failed: %v. Retrying...", err))
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			newEnc, err := encoder.NewEncoder(e.config.Codec, newStreamer, e.config.Channels, e.config.SampleRate, e.config.Bitrate)
			if err != nil {
				newStreamer.Close()
				e.onStateChange(StateReconnecting, fmt.Sprintf("Failed to initialize LAME encoder: %v. Retrying...", err))
				continue
			}

			// Connection restored! Swap encoder and streamer.
			e.stateMu.Lock()
			select {
			case <-ctx.Done():
				newEnc.Close()
				newStreamer.Close()
				e.stateMu.Unlock()
				return
			default:
			}

			if e.streamer != nil {
				e.streamer.Close()
			}
			if e.encoder != nil {
				e.encoder.Close()
			}

			e.streamer = newStreamer
			e.encoder = newEnc
			e.state = StateConnected
			e.stateMu.Unlock()

			e.onStateChange(StateConnected, "Reconnected successfully. Broadcasting live!")
			return
		}
	}
}
