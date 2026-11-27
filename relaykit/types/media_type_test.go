package types

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	mp4Sample  = []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}
	jpegSample = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	pngSample  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	webpSample = []byte{'R', 'I', 'F', 'F', 0x24, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}
	gifSample  = []byte("GIF89a\x01\x00\x01\x00")
)

func TestDetectMediaContentType(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		sample []byte
		want   string
	}{
		{name: "mp4", sample: mp4Sample, want: "video/mp4"},
		{name: "jpeg", sample: jpegSample, want: "image/jpeg"},
		{name: "png", sample: pngSample, want: "image/png"},
		{name: "webp", sample: webpSample, want: "image/webp"},
		{name: "gif", sample: gifSample, want: "image/gif"},
		{name: "nil", sample: nil, want: ""},
		{name: "empty", sample: []byte{}, want: ""},
		{name: "text", sample: []byte("this is not media"), want: ""},
		{name: "html", sample: []byte("<html><body>error</body></html>"), want: ""},
		{name: "truncated jpeg", sample: []byte{0xFF, 0xD8}, want: ""},
		{name: "short ftyp", sample: []byte("ftyp"), want: ""},
		{name: "riff but not webp", sample: []byte("RIFF\x00\x00\x00\x00WAVE"), want: ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, DetectMediaContentType(testCase.sample))
		})
	}
}

func TestDetectBase64MediaContentType(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		encoded string
		want    string
	}{
		{name: "std jpeg", encoded: base64.StdEncoding.EncodeToString(jpegSample), want: "image/jpeg"},
		{name: "raw std jpeg", encoded: base64.RawStdEncoding.EncodeToString(jpegSample), want: "image/jpeg"},
		{name: "url", encoded: base64.URLEncoding.EncodeToString(pngSample), want: "image/png"},
		{name: "raw url", encoded: base64.RawURLEncoding.EncodeToString(pngSample), want: "image/png"},
		{name: "data url prefix", encoded: "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(mp4Sample), want: "video/mp4"},
		{name: "whitespace", encoded: " \n" + base64.StdEncoding.EncodeToString(gifSample) + "\r\n ", want: "image/gif"},
		{name: "not base64", encoded: "<html>error</html>", want: ""},
		{name: "not media", encoded: base64.StdEncoding.EncodeToString([]byte("plain text payload")), want: ""},
		{name: "empty", encoded: "", want: ""},
		{name: "data url without comma", encoded: "data:image/png;base64", want: ""},
		{name: "data url not base64", encoded: "data:image/png,PNG", want: ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, DetectBase64MediaContentType(testCase.encoded))
		})
	}
}

// The detector must stop at its sniff window: garbage far past the first 684
// base64 characters must not turn a recognizable prefix into a failure.
func TestDetectBase64MediaContentTypeDecodesOnlyPrefix(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(pngSample) + strings.Repeat("A", 4096) + "!!!!"
	assert.Equal(t, "image/png", DetectBase64MediaContentType(encoded))
}

func TestIsGenericMediaContentType(t *testing.T) {
	for _, value := range []string{
		"", "   ", "application/octet-stream", "Application/Octet-Stream",
		"application/octet-stream; charset=binary", "binary/octet-stream",
		"application/binary", "application/x-binary", "application/download",
		"application/force-download", "application/unknown", "unknown/unknown", "*/*",
	} {
		assert.True(t, IsGenericMediaContentType(value), "value %q must be generic", value)
	}
	for _, value := range []string{"video/mp4", "image/png; charset=utf-8", "audio/mpeg", "text/plain", "application/json"} {
		assert.False(t, IsGenericMediaContentType(value), "value %q must not be generic", value)
	}
}

func TestIsMediaContentType(t *testing.T) {
	for _, value := range []string{"video/mp4", "image/jpeg", "audio/mpeg", "VIDEO/WEBM; codecs=vp9"} {
		assert.True(t, IsMediaContentType(value), "value %q must be media", value)
	}
	for _, value := range []string{"", "application/octet-stream", "text/html", "application/json", "application/mp4"} {
		assert.False(t, IsMediaContentType(value), "value %q must not be media", value)
	}
}
