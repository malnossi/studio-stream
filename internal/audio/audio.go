package audio

import (
	"context"
	"fmt"
	"iter"
	"log"
	"math"
	"sync"
	"sync/atomic"

	"github.com/gordonklaus/portaudio"
)

// Device represents an audio input device.
type Device struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

// Initialize initializes PortAudio.
func Initialize() error {
	return portaudio.Initialize()
}

// Terminate terminates PortAudio.
func Terminate() error {
	return portaudio.Terminate()
}

// GetInputDevices lists all available input devices with at least 1 input channel.
func GetInputDevices() (iter.Seq[Device], error) {
	devices, err := portaudio.Devices()
	if err != nil {
		return nil, err
	}
	return func(yield func(Device) bool) {
		for i, dev := range devices {
			if dev.MaxInputChannels > 0 {
				if !yield(Device{
					Index: i,
					Name:  dev.Name,
				}) {
					return
				}
			}
		}
	}, nil
}

// Recorder captures raw audio PCM samples from an input device.
type Recorder struct {
	stream             *portaudio.Stream
	inputBuf           []float32
	pcmChan            chan *[]float32
	pool               *sync.Pool
	framesPerBuffer    int
	inputChannels      int // Physical channels captured from PortAudio
	streamChannels     int // Target stream channels required by LAME/Icecast
	rmsLeft            atomic.Uint64
	rmsRight           atomic.Uint64
	sampleRate         int
	consecutiveSilence int
	silenceWarned      bool
}

// NewRecorder configures and instantiates a new Recorder.
func NewRecorder(deviceIndex int, streamChannels int, sampleRate int, framesPerBuffer int, pcmChan chan *[]float32, pool *sync.Pool) (*Recorder, error) {
	devices, err := portaudio.Devices()
	if err != nil {
		return nil, err
	}
	if deviceIndex < 0 || deviceIndex >= len(devices) {
		return nil, fmt.Errorf("invalid device index: %d", deviceIndex)
	}
	device := devices[deviceIndex]

	// Clamp requested channels to what the physical device supports.
	inputChannels := min(streamChannels, device.MaxInputChannels)

	r := &Recorder{
		inputBuf:        make([]float32, framesPerBuffer*inputChannels),
		pcmChan:         pcmChan,
		pool:            pool,
		framesPerBuffer: framesPerBuffer,
		inputChannels:   inputChannels,
		streamChannels:  streamChannels,
		sampleRate:      sampleRate,
	}

	// Request low latency parameters for input; no output device
	params := portaudio.LowLatencyParameters(device, nil)
	params.Input.Channels = inputChannels
	params.SampleRate = float64(sampleRate)
	params.FramesPerBuffer = framesPerBuffer

	stream, err := portaudio.OpenStream(params, r.inputBuf)
	if err != nil {
		return nil, err
	}
	r.stream = stream
	return r, nil
}

// Start starts capturing audio samples, maps channels, and sends them to pcmChan until cancelled.
func (r *Recorder) Start(ctx context.Context) error {
	if err := r.stream.Start(); err != nil {
		return err
	}
	defer r.stream.Stop()
	defer r.stream.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			err := r.stream.Read()
			if err != nil {
				return fmt.Errorf("portaudio stream read error: %w", err)
			}

			// Calculate real-time RMS levels based on captured physical inputChannels
			r.computeRMS(r.inputBuf)

			// Get a slice from the sync.Pool
			outSlicePtr := r.pool.Get().(*[]float32)
			requiredLen := r.framesPerBuffer * r.streamChannels
			if cap(*outSlicePtr) < requiredLen {
				*outSlicePtr = make([]float32, requiredLen)
			} else {
				*outSlicePtr = (*outSlicePtr)[:requiredLen]
			}
			outSlice := *outSlicePtr

			// Perform channel mapping
			if r.inputChannels == r.streamChannels {
				copy(outSlice, r.inputBuf)
			} else if r.inputChannels == 1 && r.streamChannels == 2 {
				// Mono to Stereo duplication
				for i := 0; i < r.framesPerBuffer; i++ {
					val := r.inputBuf[i]
					outSlice[i*2] = val
					outSlice[i*2+1] = val
				}
			} else if r.inputChannels == 2 && r.streamChannels == 1 {
				// Stereo to Mono downmix
				for i := 0; i < r.framesPerBuffer; i++ {
					outSlice[i] = (r.inputBuf[i*2] + r.inputBuf[i*2+1]) * 0.5
				}
			} else {
				// Fallback generic mapping (multi-channel pad or truncate)
				if r.inputChannels > r.streamChannels {
					// Truncate: copy first streamChannels from each frame
					for i := 0; i < r.framesPerBuffer; i++ {
						for c := 0; c < r.streamChannels; c++ {
							outSlice[i*r.streamChannels+c] = r.inputBuf[i*r.inputChannels+c]
						}
					}
				} else {
					// Pad: copy all input channels, fill remainder with zero
					for i := 0; i < r.framesPerBuffer; i++ {
						for c := 0; c < r.streamChannels; c++ {
							if c < r.inputChannels {
								outSlice[i*r.streamChannels+c] = r.inputBuf[i*r.inputChannels+c]
							} else {
								outSlice[i*r.streamChannels+c] = 0
							}
						}
					}
				}
			}

			select {
			case <-ctx.Done():
				r.pool.Put(outSlicePtr)
				return nil
			case r.pcmChan <- outSlicePtr:
				// sent successfully
			default:
				// Channel is full; drop buffer to preserve real-time stream characteristics
				r.pool.Put(outSlicePtr)
			}
		}
	}
}

// GetLevels returns the current root-mean-square levels for left and right channels.
func (r *Recorder) GetLevels() (float64, float64) {
	left := math.Float64frombits(r.rmsLeft.Load())
	right := math.Float64frombits(r.rmsRight.Load())
	return left, right
}

func (r *Recorder) computeRMS(buf []float32) {
	if len(buf) == 0 {
		return
	}
	var sumLeft, sumRight float64
	var rmsL, rmsR float64

	if r.inputChannels == 1 {
		for i := 0; i < len(buf); i++ {
			val := float64(buf[i])
			sumLeft += val * val
		}
		rmsL = math.Sqrt(sumLeft / float64(len(buf)))
		rmsR = rmsL
	} else {
		samplesPerChannel := len(buf) / r.inputChannels
		for i := 0; i < samplesPerChannel; i++ {
			lVal := float64(buf[i*r.inputChannels])
			rVal := float64(buf[i*r.inputChannels+1])
			sumLeft += lVal * lVal
			sumRight += rVal * rVal
		}
		rmsL = math.Sqrt(sumLeft / float64(samplesPerChannel))
		rmsR = math.Sqrt(sumRight / float64(samplesPerChannel))
	}

	r.rmsLeft.Store(math.Float64bits(rmsL))
	r.rmsRight.Store(math.Float64bits(rmsR))

	// Check if captured audio is completely silent (RMS is exactly 0)
	isSilent := (rmsL == 0.0 && rmsR == 0.0)

	if isSilent {
		r.consecutiveSilence++
		// If silence persists for ~3 seconds, log a warning
		// 3 seconds is: 3 * sampleRate / framesPerBuffer
		if r.sampleRate > 0 && r.framesPerBuffer > 0 {
			limit := 3 * r.sampleRate / r.framesPerBuffer
			if r.consecutiveSilence >= limit && !r.silenceWarned {
				log.Printf("[DIAGNOSTIC] Warning: Captured audio is completely silent (all zeros) for over 3 seconds. " +
					"This typically indicates a macOS microphone permissions restriction. " +
					"Please verify that your terminal/application is granted Microphone access in System Settings -> Privacy & Security -> Microphone.")
				r.silenceWarned = true
			}
		}
	} else {
		r.consecutiveSilence = 0
		if r.silenceWarned {
			log.Printf("[DIAGNOSTIC] Info: Audio signal detected. Silence warning cleared.")
			r.silenceWarned = false
		}
	}
}
