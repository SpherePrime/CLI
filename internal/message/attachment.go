package message

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	MimePDF  = "application/pdf"
	MimeDocx = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	MimeXlsx = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	MimePptx = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
)

type Attachment struct {
	FilePath string
	FileName string
	MimeType string
	Content  []byte
}

func normalizeMIME(mimeType string) string {
	base, _, _ := strings.Cut(mimeType, ";")
	return strings.ToLower(strings.TrimSpace(base))
}

func documentMIMEFromPath(nameOrPath string) string {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(nameOrPath))) {
	case ".pdf":
		return MimePDF
	case ".docx":
		return MimeDocx
	case ".xlsx":
		return MimeXlsx
	case ".pptx":
		return MimePptx
	default:
		return ""
	}
}

func documentExtFromMIME(mimeType string) string {
	switch normalizeMIME(mimeType) {
	case MimePDF:
		return ".pdf"
	case MimeDocx:
		return ".docx"
	case MimeXlsx:
		return ".xlsx"
	case MimePptx:
		return ".pptx"
	default:
		return ""
	}
}

func IsPDFMIME(mimeType string) bool {
	return normalizeMIME(mimeType) == MimePDF
}

func IsOfficeDocumentMIME(mimeType string) bool {
	switch normalizeMIME(mimeType) {
	case MimeDocx, MimeXlsx, MimePptx:
		return true
	default:
		return false
	}
}

func IsDocumentMIME(mimeType string) bool {
	return IsPDFMIME(mimeType) || IsOfficeDocumentMIME(mimeType)
}

func NormalizeAttachmentMIME(mimeType, fileName, filePath string, content []byte) string {
	if len(content) > 0 {
		if bytes.HasPrefix(content, []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
			return "image/png"
		}
		if bytes.HasPrefix(content, []byte{255, 216, 255}) {
			return "image/jpeg"
		}
		if bytes.HasPrefix(content, []byte("GIF87a")) || bytes.HasPrefix(content, []byte("GIF89a")) {
			return "image/gif"
		}
		if len(content) >= 12 && bytes.HasPrefix(content, []byte("RIFF")) && bytes.Equal(content[8:12], []byte("WEBP")) {
			return "image/webp"
		}
		if bytes.HasPrefix(content, []byte("%PDF-")) {
			return MimePDF
		}
	}
	if extMIME := documentMIMEFromPath(fileName); extMIME != "" {
		return extMIME
	}
	if extMIME := documentMIMEFromPath(filePath); extMIME != "" {
		return extMIME
	}
	norm := normalizeMIME(mimeType)
	if IsDocumentMIME(norm) || strings.HasPrefix(norm, "image/") || strings.HasPrefix(norm, "audio/") || strings.HasPrefix(norm, "video/") {
		return norm
	}
	if len(content) > 0 {
		if bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content) {
			if norm != "" && !strings.HasPrefix(norm, "text/") {
				return norm
			}
			return "application/octet-stream"
		}
	}
	if norm != "" {
		return norm
	}
	return "text/plain"
}

func (a Attachment) EffectiveMIME() string {
	if extMIME := documentMIMEFromPath(a.FileName); extMIME != "" {
		return extMIME
	}
	if extMIME := documentMIMEFromPath(a.FilePath); extMIME != "" {
		return extMIME
	}
	norm := normalizeMIME(a.MimeType)
	if norm != "" {
		return norm
	}
	return "application/octet-stream"
}

func (a Attachment) DisplayName() string {
	if trimmed := strings.TrimSpace(a.FileName); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(a.FilePath); trimmed != "" {
		return filepath.Base(trimmed)
	}
	return "document" + documentExtFromMIME(a.EffectiveMIME())
}

func (a Attachment) IsPDF() bool {
	return IsPDFMIME(a.EffectiveMIME())
}

func (a Attachment) IsOfficeDocument() bool {
	return IsOfficeDocumentMIME(a.EffectiveMIME())
}

func (a Attachment) IsDocument() bool {
	return IsDocumentMIME(a.EffectiveMIME())
}

func (a Attachment) IsText() bool {
	if a.IsDocument() {
		return false
	}
	return strings.HasPrefix(normalizeMIME(a.MimeType), "text/")
}

func (a Attachment) IsImage() bool {
	return strings.HasPrefix(normalizeMIME(a.MimeType), "image/")
}

func (a Attachment) IsMarkdown() bool {
	return normalizeMIME(a.MimeType) == "text/markdown"
}

func (a Attachment) SupportsFilePart() bool {
	mime := a.EffectiveMIME()
	if IsOfficeDocumentMIME(mime) {
		return false
	}
	return strings.HasPrefix(mime, "image/") ||
		mime == MimePDF ||
		strings.HasPrefix(mime, "audio/") ||
		strings.HasPrefix(mime, "video/")
}

func (a Attachment) DocumentExtension() string {
	if ext := strings.ToLower(filepath.Ext(strings.TrimSpace(a.FileName))); ext != "" {
		return ext
	}
	if ext := strings.ToLower(filepath.Ext(strings.TrimSpace(a.FilePath))); ext != "" {
		return ext
	}
	return documentExtFromMIME(a.EffectiveMIME())
}

func ContainsTextAttachment(attachments []Attachment) bool {
	return slices.ContainsFunc(attachments, func(a Attachment) bool {
		return a.IsText()
	})
}

func ContainsDocumentAttachment(attachments []Attachment) bool {
	return slices.ContainsFunc(attachments, func(a Attachment) bool {
		return a.IsDocument()
	})
}
