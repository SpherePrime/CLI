package tools

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/disintegration/imaging"
)

const (
	maxModelImageRawBytes  = 3_600_000
	maxModelImageDimension = 8000
)

var (
	modelImageTargetEdges   = []int{2048, 1568, 1200, 800}
	modelImageJPEGQualities = []int{88, 78, 68}
)

func fitImageForModel(data []byte, mimeType string) ([]byte, string) {
	if len(data) <= maxModelImageRawBytes {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || (cfg.Width <= maxModelImageDimension && cfg.Height <= maxModelImageDimension) {
			return data, mimeType
		}
	}

	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, mimeType
	}
	bounds := decoded.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return data, mimeType
	}

	isPNG := strings.EqualFold(mimeType, "image/png")
	var bestFallback []byte
	for idx, edge := range modelImageTargetEdges {
		maxEdge := min(edge, maxModelImageDimension)
		resized := imaging.Fit(decoded, maxEdge, maxEdge, imaging.Lanczos)
		if resized.Bounds().Dx() <= 0 || resized.Bounds().Dy() <= 0 {
			continue
		}

		if idx == 0 && isPNG {
			if encoded, ok := encodeImageAsPNG(resized); ok && len(encoded) <= maxModelImageRawBytes {
				return encoded, "image/png"
			}
		}

		flattened := compositeOntoWhiteIfNeeded(resized)
		for _, quality := range modelImageJPEGQualities {
			encoded, ok := encodeImageAsJPEG(flattened, quality)
			if !ok {
				continue
			}
			bestFallback = encoded
			if len(encoded) <= maxModelImageRawBytes {
				return encoded, "image/jpeg"
			}
		}
	}

	if len(bestFallback) > 0 {
		return bestFallback, "image/jpeg"
	}
	return data, mimeType
}

func encodeImageAsPNG(img image.Image) ([]byte, bool) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil || buf.Len() == 0 {
		return nil, false
	}
	return buf.Bytes(), true
}

func encodeImageAsJPEG(img image.Image, quality int) ([]byte, bool) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil || buf.Len() == 0 {
		return nil, false
	}
	return buf.Bytes(), true
}

func compositeOntoWhiteIfNeeded(img image.Image) image.Image {
	if checker, ok := img.(interface{ Opaque() bool }); ok && checker.Opaque() {
		return img
	}
	bounds := img.Bounds()
	opaqueCanvas := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(opaqueCanvas, opaqueCanvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(opaqueCanvas, opaqueCanvas.Bounds(), img, bounds.Min, draw.Over)
	return opaqueCanvas
}
