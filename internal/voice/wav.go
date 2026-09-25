package voice

import (
	"encoding/binary"
	"fmt"
	"time"
)

// pcmDuration returns how much audio the PCM buffer holds.
func pcmDuration(pcm []byte) time.Duration {
	samples := int64(len(pcm)) / int64(bytesPerSample*Channels)
	return time.Duration(samples) * time.Second / time.Duration(SampleRate)
}

// encodeWAV wraps raw 16 kHz mono little-endian PCM in a RIFF header. The
// header is written by Prime rather than the recorder because capture ends by
// killing the recorder process, which leaves tools such as ffmpeg unable to
// seek back and patch their own size fields.
func encodeWAV(pcm []byte) []byte {
	dataSize := uint32(min(len(pcm), 0x7ffffffe))

	out := make([]byte, 0, headerSize+len(pcm))
	out = append(out, []byte("RIFF")...)
	out = binary.LittleEndian.AppendUint32(out, headerSize-8+dataSize)
	out = append(out, []byte("WAVE")...)
	out = append(out, []byte("fmt ")...)
	out = binary.LittleEndian.AppendUint32(out, 16)
	out = binary.LittleEndian.AppendUint16(out, pcmFormat)
	out = binary.LittleEndian.AppendUint16(out, Channels)
	out = binary.LittleEndian.AppendUint32(out, SampleRate)
	out = binary.LittleEndian.AppendUint32(out, uint32(SampleRate*Channels*bytesPerSample))
	out = binary.LittleEndian.AppendUint16(out, uint16(Channels*bytesPerSample))
	out = binary.LittleEndian.AppendUint16(out, uint16(bytesPerSample*8))
	out = append(out, []byte("data")...)
	out = binary.LittleEndian.AppendUint32(out, dataSize)
	out = append(out, pcm...)
	return out
}

const (
	headerSize = 44
	pcmFormat  = 1
)

// decodePCM accepts either raw PCM or a WAV container and returns the PCM it
// carries. Recording tools that write files themselves produce a WAV header,
// and a header interrupted by a kill can carry a bogus size, so the actual
// byte count wins over the declared one.
func decodePCM(raw []byte) ([]byte, error) {
	if len(raw) >= 12 && string(raw[:4]) == "RIFF" && string(raw[8:12]) == "WAVE" {
		return pcmFromWAV(raw)
	}
	// Odd-length buffers cannot end mid-sample; drop the stray byte so the
	// RIFF header Prime writes stays consistent.
	if len(raw)%2 != 0 {
		raw = raw[:len(raw)-1]
	}
	return raw, nil
}

// pcmFromWAV walks the chunks of a WAV buffer looking for "data".
func pcmFromWAV(raw []byte) ([]byte, error) {
	for offset := 12; offset+8 <= len(raw); {
		id := string(raw[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(raw[offset+4 : offset+8]))
		body := offset + 8
		if size < 0 || body+size > len(raw) {
			// Truncated or unpatched size: take what is actually there.
			size = len(raw) - body
		}
		if id == "data" {
			return decodePCM(raw[body : body+size])
		}
		offset = body + size + size%2
	}
	return nil, fmt.Errorf("wav buffer has no data chunk")
}
