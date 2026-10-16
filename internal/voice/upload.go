package voice

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"
)

func ValidateUpload(wav []byte) (*Audio, error) {
	if len(wav) < 44 || len(wav) > 20*1024*1024 || string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, fmt.Errorf("expected PCM WAV audio up to 20 MiB")
	}
	formatOK := false
	var pcm []byte
	for offset := 12; offset+8 <= len(wav); {
		size := int(binary.LittleEndian.Uint32(wav[offset+4 : offset+8]))
		body := offset + 8
		if size > len(wav)-body {
			return nil, fmt.Errorf("truncated WAV chunk")
		}
		switch string(wav[offset : offset+4]) {
		case "fmt ":
			if size < 16 {
				return nil, fmt.Errorf("invalid WAV format")
			}
			formatOK = binary.LittleEndian.Uint16(wav[body:]) == 1 && binary.LittleEndian.Uint16(wav[body+2:]) == 1 && binary.LittleEndian.Uint32(wav[body+4:]) == SampleRate && binary.LittleEndian.Uint16(wav[body+14:]) == 16
		case "data":
			pcm = wav[body : body+size]
		}
		offset = body + size + size%2
	}
	if !formatOK || len(pcm) == 0 || len(pcm)%2 != 0 {
		return nil, fmt.Errorf("expected 16 kHz mono 16-bit PCM WAV")
	}
	duration := pcmDuration(pcm)
	if duration > 120*time.Second || duration < minSpeechDuration {
		return nil, fmt.Errorf("audio duration must be between 300 ms and 120 seconds")
	}
	return &Audio{WAV: encodeWAV(pcm), Duration: duration, Recorder: "desktop"}, nil
}

func TranscribeUpload(ctx context.Context, settings Settings, wav []byte) (string, error) {
	audio, err := ValidateUpload(wav)
	if err != nil {
		return "", err
	}
	transcribers := newTranscribers(ctx, settings, newProber())
	if len(transcribers) == 0 {
		return "", ErrNoTranscriber
	}
	var lastErr error
	for _, transcriber := range transcribers {
		text, err := transcriber.Transcribe(ctx, audio)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	return "", lastErr
}
