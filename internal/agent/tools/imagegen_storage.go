package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// detectImageExtension identifies the image format from magic bytes and
// falls back to png, the format most image APIs return.
func detectImageExtension(bytes []byte) string {
	switch {
	case len(bytes) >= 8 && string(bytes[:8]) == "\x89PNG\r\n\x1a\n":
		return "png"
	case len(bytes) >= 3 && bytes[0] == 0xff && bytes[1] == 0xd8 && bytes[2] == 0xff:
		return "jpg"
	case len(bytes) >= 12 && string(bytes[:4]) == "RIFF" && string(bytes[8:12]) == "WEBP":
		return "webp"
	default:
		return "png"
	}
}

func sanitizeImagePrefix(prefix string) string {
	cleaned := strings.Trim(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '_' || r == '-':
			return r
		default:
			return '-'
		}
	}, prefix), "-")
	if cleaned == "" {
		return "image"
	}
	return cleaned
}

// saveImages writes the generated images into outputDir and returns the
// absolute file paths in the same order as the input.
func saveImages(images []generatedImage, outputDir, filenamePrefix string) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	prefix := sanitizeImagePrefix(filenamePrefix)
	timestamp := time.Now().Format("20060102-150405")

	paths := make([]string, 0, len(images))
	for index, image := range images {
		fileName := fmt.Sprintf("%s-%s-%d.%s", prefix, timestamp, index+1, detectImageExtension(image.Bytes))
		filePath := filepath.Join(outputDir, fileName)
		if err := os.WriteFile(filePath, image.Bytes, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", filePath, err)
		}
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			absPath = filePath
		}
		paths = append(paths, absPath)
	}
	return paths, nil
}
