package voice

import "context"

func newTranscribers(_ context.Context, settings Settings, _ prober) []Transcriber {
	return []Transcriber{newTinyAsrTranscriber(settings)}
}
