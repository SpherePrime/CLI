package agent

import (
	"encoding/base64"
	"fmt"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const recentImageResults = 2

func extraToolImages(msgs []message.Message) map[string][]fantasy.FilePart {
	var order []string
	images := make(map[string][]fantasy.FilePart)
	for _, m := range msgs {
		if m.Role != message.Tool {
			continue
		}
		for _, result := range m.ToolResults() {
			if result.IsError || len(result.Images) < 2 {
				continue
			}
			var files []fantasy.FilePart
			for index, image := range result.Images[1:] {
				decoded, err := base64.StdEncoding.DecodeString(image.Data)
				if err != nil || len(decoded) == 0 {
					continue
				}
				files = append(files, fantasy.FilePart{
					Data:      decoded,
					MediaType: image.MIMEType,
					Filename:  fmt.Sprintf("tool-result-%s-%d", result.ToolCallID, index+2),
				})
			}
			if len(files) == 0 {
				continue
			}
			if _, seen := images[result.ToolCallID]; !seen {
				order = append(order, result.ToolCallID)
			}
			images[result.ToolCallID] = files
		}
	}
	for len(order) > recentImageResults {
		delete(images, order[0])
		order = order[1:]
	}
	return images
}

func extraImagesMessage(calls []message.ToolCall, images map[string][]fantasy.FilePart) (fantasy.Message, bool) {
	var files []fantasy.FilePart
	for _, call := range calls {
		files = append(files, images[call.ID]...)
	}
	if len(files) == 0 {
		return fantasy.Message{}, false
	}
	return fantasy.NewUserMessage(fmt.Sprintf("The remaining %d images from the tool result above, in order:", len(files)), files...), true
}
