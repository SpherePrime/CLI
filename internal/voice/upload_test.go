package voice

import (
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadedAudioValidation(t *testing.T) {
	valid := encodeWAV(make([]byte, SampleRate*2))
	if _, err := ValidateUpload(valid); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateUpload([]byte("not audio")); err == nil {
		t.Fatal("invalid audio accepted")
	}
	stereo := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint16(stereo[22:24], 2)
	if _, err := ValidateUpload(stereo); err == nil {
		t.Fatal("unsupported audio accepted")
	}
	if _, err := ValidateUpload(encodeWAV(make([]byte, SampleRate*2*121))); err == nil {
		t.Fatal("long audio accepted")
	}
}

func TestUploadReusesConfiguredTranscriber(t *testing.T) {
	ctx := context.Background()
	_, err := TranscribeUpload(ctx, Settings{}, []byte("invalid"))
	if err == nil {
		t.Fatal("invalid upload reached transcriber")
	}
}

func TestUploadTranscribesThroughConfiguredProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			t.Errorf("wrong endpoint %s", r.URL.Path)
		}
		fields, audio, _ := formOf(t, r)
		if fields["model"] != "test-model" || len(audio) == 0 {
			t.Error("missing model or audio")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"готово"}`))
	}))
	defer server.Close()
	text, err := TranscribeUpload(context.Background(), Settings{Engine: EngineOpenAI, BaseURL: server.URL, APIKey: "fixture", Model: "test-model"}, encodeWAV(make([]byte, SampleRate*2)))
	if err != nil || text != "готово" {
		t.Fatalf("text %q error %v", text, err)
	}
}
