package agent

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"
)

const (
	imageMaxEdgePixels      = 1568
	imageMaxTotalPixels     = 1_150_000
	imagePixelsPerToken     = 750
	imageMinTokens          = 85
	imageMaxTokens          = 1600
	imageUnknownSizeTokens  = 1200
	documentTokensPerPage   = 2000
	base64ConfigPrefixBytes = 262_144
)

func estimateMediaTokens(mediaType, text string, data []byte) int64 {
	tokens := approxTokenCount(text)
	if len(data) == 0 {
		return tokens + approxTokenCount(mediaType)
	}
	mediaType = strings.ToLower(mediaType)
	switch {
	case strings.HasPrefix(mediaType, "image/"):
		return tokens + estimateImageTokens(data)
	case mediaType == "application/pdf":
		return tokens + estimatePDFTokens(data)
	case strings.HasPrefix(mediaType, "text/"), mediaType == "application/json":
		return tokens + approxTokenCount(string(data))
	default:
		return tokens + approxTokenCount(mediaType) + int64(len(data)/4)
	}
}

func estimateBase64MediaTokens(mediaType, text, encoded string) int64 {
	if encoded == "" {
		return estimateMediaTokens(mediaType, text, nil)
	}
	if strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		prefix := encoded
		if len(prefix) > base64ConfigPrefixBytes {
			prefix = prefix[:base64ConfigPrefixBytes]
		}
		decoded, _ := base64.StdEncoding.DecodeString(prefix[:len(prefix)/4*4])
		if len(decoded) == 0 {
			return approxTokenCount(text) + imageUnknownSizeTokens
		}
		return approxTokenCount(text) + estimateImageTokens(decoded)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return approxTokenCount(text) + int64(len(encoded)*3/16)
	}
	return estimateMediaTokens(mediaType, text, decoded)
}

func estimateImageTokens(data []byte) int64 {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return imageUnknownSizeTokens
	}
	return imageTokensForSize(config.Width, config.Height)
}

func imageTokensForSize(width, height int) int64 {
	w, h := float64(width), float64(height)
	if edge := math.Max(w, h); edge > imageMaxEdgePixels {
		scale := imageMaxEdgePixels / edge
		w, h = w*scale, h*scale
	}
	if total := w * h; total > imageMaxTotalPixels {
		scale := math.Sqrt(imageMaxTotalPixels / total)
		w, h = w*scale, h*scale
	}
	tokens := int64(math.Ceil(w * h / imagePixelsPerToken))
	return min(max(tokens, imageMinTokens), imageMaxTokens)
}

func estimatePDFTokens(data []byte) int64 {
	pages := bytes.Count(data, []byte("/Type /Page")) - bytes.Count(data, []byte("/Type /Pages"))
	pages += bytes.Count(data, []byte("/Type/Page")) - bytes.Count(data, []byte("/Type/Pages"))
	if pages <= 0 {
		pages = 1
	}
	return int64(pages) * documentTokensPerPage
}
