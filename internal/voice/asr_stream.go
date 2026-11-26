package voice

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/SpherePrime/CLI/internal/voice/asrpb"
)

type tinyAsrTranscriber struct {
	dataDir       string
	language      string
	startLanguage string
}

func newTinyAsrTranscriber(settings Settings) Transcriber {
	return &tinyAsrTranscriber{
		dataDir:       settings.DataDir,
		language:      NormalizeLanguage(settings.Language),
		startLanguage: NormalizeLanguage(settings.InterfaceLanguage),
	}
}

func (t *tinyAsrTranscriber) Name() string {
	return EngineTinyASR
}

func (t *tinyAsrTranscriber) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	if audio == nil || len(audio.WAV) == 0 {
		return "", ErrNoSpeech
	}
	pcm, err := extractPcmS16LE(audio.WAV)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrTranscribeFailed, err)
	}
	if len(pcm) == 0 {
		return "", ErrNoSpeech
	}
	client, token, err := sharedAsrPool.acquire(ctx, t.dataDir)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrTranscribeFailed, err)
	}
	reqCtx := withAsrToken(ctx, token)
	resp, err := client.Transcribe(reqCtx, &asrpb.TranscribeRequest{
		PcmS16LE:      pcm,
		Language:      t.language,
		StartLanguage: t.startLanguage,
	})
	if err != nil {
		sharedAsrPool.invalidate()
		return "", fmt.Errorf("%w: %w", ErrTranscribeFailed, err)
	}
	return strings.TrimSpace(resp.Text), nil
}

type LiveUpdate struct {
	Committed string
	Open      string
	Tail      string
	Language  string
	Final     bool
}

type LiveStream struct {
	stream asrpb.StreamRecognizeClient
	done   chan struct{}
	final  string
	err    error
}

func StartLiveStream(ctx context.Context, dataDir, language, startLanguage string, onUpdate func(LiveUpdate)) (*LiveStream, error) {
	client, token, err := sharedAsrPool.acquire(ctx, dataDir)
	if err != nil {
		return nil, err
	}
	stream, err := client.StreamRecognize(withAsrToken(ctx, token))
	if err != nil {
		sharedAsrPool.invalidate()
		return nil, err
	}
	if err := stream.Send(&asrpb.RecognizeRequest{
		Start: &asrpb.StartSession{
			Language:      NormalizeLanguage(language),
			StartLanguage: NormalizeLanguage(startLanguage),
		},
	}); err != nil {
		return nil, err
	}

	live := &LiveStream{
		stream: stream,
		done:   make(chan struct{}),
	}
	go func() {
		defer close(live.done)
		for {
			ev, recvErr := stream.Recv()
			if recvErr != nil {
				if !errors.Is(recvErr, io.EOF) {
					live.err = recvErr
				}
				return
			}
			switch {
			case ev.Partial != nil && onUpdate != nil:
				onUpdate(LiveUpdate{
					Committed: ev.Partial.Committed,
					Open:      ev.Partial.Open,
					Tail:      ev.Partial.Tail,
				})
			case ev.Language != nil && onUpdate != nil:
				onUpdate(LiveUpdate{Language: ev.Language.Language})
			case ev.Final != nil:
				live.final = strings.TrimSpace(ev.Final.Text)
				if onUpdate != nil {
					onUpdate(LiveUpdate{Committed: live.final, Final: true})
				}
				return
			}
		}
	}()
	return live, nil
}

func (l *LiveStream) PushPCM(pcm []byte) error {
	if l == nil || len(pcm) == 0 {
		return nil
	}
	return l.stream.Send(&asrpb.RecognizeRequest{
		Audio: &asrpb.AudioChunk{PcmS16LE: pcm},
	})
}

func (l *LiveStream) Finish() (string, error) {
	if l == nil {
		return "", nil
	}
	_ = l.stream.Send(&asrpb.RecognizeRequest{Finish: true})
	_ = l.stream.CloseSend()
	<-l.done
	return l.final, l.err
}

func (l *LiveStream) Cancel() {
	if l == nil {
		return
	}
	_ = l.stream.Send(&asrpb.RecognizeRequest{Cancel: true})
	_ = l.stream.CloseSend()
	<-l.done
}

func extractPcmS16LE(wav []byte) ([]byte, error) {
	if len(wav) < 12 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return wav, nil
	}
	offset := 12
	for offset+8 <= len(wav) {
		chunkID := string(wav[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(wav[offset+4 : offset+8]))
		dataStart := offset + 8
		dataEnd := dataStart + chunkSize
		if dataEnd > len(wav) {
			dataEnd = len(wav)
		}
		if chunkID == "data" {
			return wav[dataStart:dataEnd], nil
		}
		offset = dataStart + chunkSize
		if chunkSize%2 == 1 {
			offset++
		}
	}
	return nil, errors.New("WAV missing data chunk")
}

func ValidateUpload(wav []byte) (int, error) {
	if len(wav) < 44 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return 0, errors.New("invalid WAV upload")
	}
	pcm, err := extractPcmS16LE(wav)
	if err != nil {
		return 0, err
	}
	if len(pcm) == 0 {
		return 0, ErrNoSpeech
	}
	return len(pcm), nil
}

func TranscribeUpload(ctx context.Context, settings Settings, wav []byte) (string, error) {
	if _, err := ValidateUpload(wav); err != nil {
		return "", err
	}
	return newTinyAsrTranscriber(settings).Transcribe(ctx, &Audio{WAV: wav})
}
