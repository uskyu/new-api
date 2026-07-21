package claude

import (
	"encoding/base64"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func BuildBase64MediaMessage(contentType string, base64Data string, mimeType string, filename string) (*dto.ClaudeMediaMessage, error) {
	mimeType = strings.TrimSpace(mimeType)
	if parsed, _, err := mime.ParseMediaType(mimeType); err == nil {
		mimeType = parsed
	}
	if mimeType == "" {
		switch strings.ToLower(filepath.Ext(filename)) {
		case ".pdf":
			mimeType = "application/pdf"
		case ".txt":
			mimeType = "text/plain"
		default:
			mimeType = mime.TypeByExtension(filepath.Ext(filename))
			if parsed, _, err := mime.ParseMediaType(mimeType); err == nil {
				mimeType = parsed
			}
		}
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	switch contentType {
	case dto.ContentTypeImageURL, "input_image":
		if !strings.HasPrefix(mimeType, "image/") {
			return nil, nil
		}
		return &dto.ClaudeMediaMessage{
			Type: "image",
			Source: &dto.ClaudeMessageSource{
				Type:      "base64",
				MediaType: mimeType,
				Data:      base64Data,
			},
		}, nil
	case dto.ContentTypeFile, "input_file":
		switch {
		case strings.HasPrefix(mimeType, "application/pdf"):
			return &dto.ClaudeMediaMessage{
				Type: "document",
				Source: &dto.ClaudeMessageSource{
					Type:      "base64",
					MediaType: mimeType,
					Data:      base64Data,
				},
			}, nil
		case strings.HasPrefix(mimeType, "text/"):
			text, err := base64.StdEncoding.DecodeString(base64Data)
			if err != nil {
				return nil, fmt.Errorf("decode Claude text file data: %w", err)
			}
			return &dto.ClaudeMediaMessage{
				Type: "text",
				Text: common.GetPointer(string(text)),
			}, nil
		}
	}
	return nil, nil
}
