package agent

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openaicompat"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestPreparePromptRetainsExplicitImagesWithoutCatalogSupport(t *testing.T) {
	agent := &sessionAgent{isSubAgent: true}
	previous := []byte("previous-image-bytes")
	current := []byte("current-image-bytes")
	history, files := agent.preparePrompt([]message.Message{{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Previous image"},
			message.BinaryContent{Path: "previous.png", MIMEType: "image/png", Data: previous},
		},
	}}, false, message.Attachment{
		FileName: "current.jpg",
		MimeType: "image/jpeg",
		Content:  current,
	})
	require.Len(t, history, 1)
	require.Len(t, history[0].Content, 2)
	image, ok := fantasy.AsMessagePart[fantasy.FilePart](history[0].Content[1])
	require.True(t, ok)
	require.Equal(t, previous, image.Data)
	require.Equal(t, "image/png", image.MediaType)
	require.Len(t, files, 1)
	require.Equal(t, current, files[0].Data)
	require.Equal(t, "image/jpeg", files[0].MediaType)
	prompt := append(history, fantasy.NewUserMessage("Current image", files...))
	payload, warnings := openaicompat.ToPromptFunc(prompt, "vision-model", "custom-provider")
	require.Empty(t, warnings)
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	var requests []struct {
		Content []struct {
			Type     string `json:"type"`
			ImageURL *struct {
				URL string `json:"url"`
			} `json:"image_url"`
		} `json:"content"`
	}
	require.NoError(t, json.Unmarshal(encoded, &requests))
	var imageURLs []string
	for _, request := range requests {
		for _, part := range request.Content {
			if part.Type == "image_url" && part.ImageURL != nil {
				imageURLs = append(imageURLs, part.ImageURL.URL)
			}
		}
	}
	require.Equal(t, []string{
		"data:image/png;base64," + base64.StdEncoding.EncodeToString(previous),
		"data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(current),
	}, imageURLs)
}
