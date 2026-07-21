package claude

import (
	"encoding/base64"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildBase64MediaMessage(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		data        string
		mimeType    string
		filename    string
		wantType    string
		wantText    string
	}{
		{name: "image", contentType: dto.ContentTypeImageURL, data: "image-data", mimeType: "image/png", wantType: "image"},
		{name: "PDF file", contentType: dto.ContentTypeFile, data: "pdf-data", mimeType: "application/pdf; version=1.7", wantType: "document"},
		{name: "PDF filename fallback", contentType: dto.ContentTypeFile, data: "pdf-data", filename: "spec.pdf", wantType: "document"},
		{name: "text file", contentType: dto.ContentTypeFile, data: base64.StdEncoding.EncodeToString([]byte("alpha\nbeta")), mimeType: "text/plain; charset=utf-8", wantType: "text", wantText: "alpha\nbeta"},
		{name: "text filename fallback", contentType: dto.ContentTypeFile, data: base64.StdEncoding.EncodeToString([]byte("alpha\nbeta")), filename: "notes.txt", wantType: "text", wantText: "alpha\nbeta"},
		{name: "audio is unsupported", contentType: dto.ContentTypeInputAudio, data: "audio-data", mimeType: "audio/wav"},
		{name: "binary file is unsupported", contentType: dto.ContentTypeFile, data: "binary-data", mimeType: "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildBase64MediaMessage(tt.contentType, tt.data, tt.mimeType, tt.filename)
			require.NoError(t, err)
			if tt.wantType == "" {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.wantType, got.Type)
			if tt.wantType == "text" {
				require.NotNil(t, got.Text)
				assert.Equal(t, tt.wantText, *got.Text)
				assert.Nil(t, got.Source)
				return
			}
			require.NotNil(t, got.Source)
			if tt.wantType == "document" {
				assert.Equal(t, "application/pdf", got.Source.MediaType)
			} else {
				assert.Equal(t, tt.mimeType, got.Source.MediaType)
			}
			assert.Equal(t, tt.data, got.Source.Data)
		})
	}
}
