package encoder

/*
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"io"
	"time"
	"unsafe"

	ogg "github.com/tryphon/go-ogg"
	vorbis "github.com/tryphon/go-vorbis"
	"github.com/tryphon/go-vorbis/vorbisenc"
)

type VorbisEncoder struct {
	w   io.Writer
	oss ogg.StreamState
	og  ogg.Page
	op  ogg.Packet

	vi *vorbis.Info
	vc *vorbis.Comment
	vd *vorbis.DspState
	vb *vorbis.Block

	channels int
	eos      bool
}

func NewVorbisEncoder(w io.Writer, channels int, sampleRate int, bitrate int) (*VorbisEncoder, error) {
	e := &VorbisEncoder{
		w:        w,
		channels: channels,
		vi:       (*vorbis.Info)(C.malloc(C.size_t(unsafe.Sizeof(vorbis.Info{})))),
		vc:       (*vorbis.Comment)(C.malloc(C.size_t(unsafe.Sizeof(vorbis.Comment{})))),
		vd:       (*vorbis.DspState)(C.malloc(C.size_t(unsafe.Sizeof(vorbis.DspState{})))),
		vb:       (*vorbis.Block)(C.malloc(C.size_t(unsafe.Sizeof(vorbis.Block{})))),
	}

	e.vi.Init()
	// Quality roughly 0.4 for 128kbps stereo. Bitrate parameter is in kbps, let's map it roughly to quality
	// quality range is -0.1 to 1.0. 128kbps ~ 0.4.
	// Since init is required, let's use Init (ABR mode).
	ret := vorbisenc.Init(e.vi, int32(channels), int32(sampleRate), -1, int32(bitrate*1000), -1)
	if ret != 0 {
		return nil, fmt.Errorf("vorbis init failed with code %d", ret)
	}

	e.vc.Init()
	e.vc.AddTag("ENCODER", "StudioStream")

	vorbis.AnalysisInit(e.vd, e.vi)
	e.vb.Init(e.vd)

	// e.oss.Init(rand.Int31()) will just use the global rand.
	// Actually we should create a new rand source, or just use time.Now().UnixNano().
	e.oss.Init(int32(time.Now().UnixNano()))

	var (
		header     ogg.Packet
		headerComm ogg.Packet
		headerCode ogg.Packet
	)
	vorbis.AnalysisHeaderOut(e.vd, e.vc, &header, &headerComm, &headerCode)
	e.oss.PacketIn(&header)
	e.oss.PacketIn(&headerComm)
	e.oss.PacketIn(&headerCode)

	for {
		if !e.oss.Flush(&e.og) {
			break
		}
		w.Write(e.og.Header)
		w.Write(e.og.Body)
	}

	return e, nil
}

func (e *VorbisEncoder) WritePCM(pcm []float32) error {
	if len(pcm) == 0 {
		return nil
	}

	frames := len(pcm) / e.channels
	buffer := vorbis.AnalysisBuffer(e.vd, frames)

	// de-interleave
	for c := 0; c < e.channels; c++ {
		for i := 0; i < frames; i++ {
			buffer[c][i] = pcm[i*e.channels+c]
		}
	}

	vorbis.AnalysisWrote(e.vd, frames)

	for vorbis.AnalysisBlockOut(e.vd, e.vb) == 1 {
		vorbis.Analysis(e.vb, nil)
		vorbis.BitrateAddBlock(e.vb)

		for vorbis.BitrateFlushPacket(e.vd, &e.op) != 0 {
			e.oss.PacketIn(&e.op)

			for !e.eos {
				if !e.oss.PageOut(&e.og) {
					break
				}
				e.w.Write(e.og.Header)
				e.w.Write(e.og.Body)
				if e.og.Eos() {
					e.eos = true
				}
			}
		}
	}

	return nil
}

func (e *VorbisEncoder) Flush() error {
	return nil
}

func (e *VorbisEncoder) Close() {
	vorbis.AnalysisWrote(e.vd, 0)
	for vorbis.AnalysisBlockOut(e.vd, e.vb) == 1 {
		vorbis.Analysis(e.vb, nil)
		vorbis.BitrateAddBlock(e.vb)

		for vorbis.BitrateFlushPacket(e.vd, &e.op) != 0 {
			e.oss.PacketIn(&e.op)

			for !e.eos {
				if !e.oss.PageOut(&e.og) {
					break
				}
				e.w.Write(e.og.Header)
				e.w.Write(e.og.Body)
				if e.og.Eos() {
					e.eos = true
				}
			}
		}
	}

	e.vb.Clear()
	e.vd.Clear()
	e.vc.Clear()
	e.vi.Clear()

	C.free(unsafe.Pointer(e.vb))
	C.free(unsafe.Pointer(e.vd))
	C.free(unsafe.Pointer(e.vc))
	C.free(unsafe.Pointer(e.vi))
}
