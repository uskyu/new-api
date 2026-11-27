package types

import (
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

// MediaSniffSampleBytes is how much of a payload the media detectors inspect.
// 512 bytes cover every signature below and match the HTTP sniffing window.
const MediaSniffSampleBytes = 512

// DetectMediaContentType identifies the leading signature of common media
// containers. It returns "" when the sample matches no known media format, so
// callers never see text or generic application types from it.
func DetectMediaContentType(sample []byte) string {
	switch {
	case len(sample) >= 8 && string(sample[4:8]) == "ftyp":
		// ISO base media file format: the box size precedes the ftyp box.
		return "video/mp4"
	case len(sample) >= 3 && sample[0] == 0xFF && sample[1] == 0xD8 && sample[2] == 0xFF:
		return "image/jpeg"
	case len(sample) >= 8 && sample[0] == 0x89 && sample[1] == 0x50 && sample[2] == 0x4E && sample[3] == 0x47 &&
		sample[4] == 0x0D && sample[5] == 0x0A && sample[6] == 0x1A && sample[7] == 0x0A:
		return "image/png"
	case len(sample) >= 12 && string(sample[0:4]) == "RIFF" && string(sample[8:12]) == "WEBP":
		return "image/webp"
	case len(sample) >= 6 && (string(sample[0:6]) == "GIF87a" || string(sample[0:6]) == "GIF89a"):
		return "image/gif"
	}
	return ""
}

// DetectBase64MediaContentType identifies the media type of a base64 payload
// without decoding the whole thing. A data URL prefix is accepted; Std, raw
// Std, URL and raw URL base64 alphabets are tried, and only enough input for
// one 512-byte sample is decoded.
func DetectBase64MediaContentType(encoded string) string {
	payload := strings.TrimSpace(encoded)
	if strings.HasPrefix(payload, "data:") {
		comma := strings.IndexByte(payload, ',')
		if comma < 0 {
			return ""
		}
		if !strings.Contains(strings.ToLower(payload[:comma]), ";base64") {
			return ""
		}
		payload = strings.TrimSpace(payload[comma+1:])
	}
	sample, ok := decodeBase64MediaSample(payload)
	if !ok {
		return ""
	}
	return DetectMediaContentType(sample)
}

// IsGenericMediaContentType reports whether a Content-Type carries no real
// format information, such as an absent value or application/octet-stream.
func IsGenericMediaContentType(value string) bool {
	switch normalizeMediaType(value) {
	case "", "*/*", "application/octet-stream", "binary/octet-stream",
		"application/binary", "application/x-binary",
		"application/download", "application/force-download", "application/x-download",
		"application/unknown", "application/x-unknown", "application/unknown-content-type",
		"content/unknown", "unknown/unknown":
		return true
	}
	return false
}

// IsMediaContentType reports whether a Content-Type belongs to the media
// families the task artifact proxy serves.
func IsMediaContentType(value string) bool {
	mediaType := normalizeMediaType(value)
	return strings.HasPrefix(mediaType, "image/") ||
		strings.HasPrefix(mediaType, "video/") ||
		strings.HasPrefix(mediaType, "audio/")
}

func normalizeMediaType(value string) string {
	mediaType := strings.TrimSpace(value)
	if index := strings.IndexByte(mediaType, ';'); index >= 0 {
		mediaType = mediaType[:index]
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}

// decodeBase64MediaSample decodes at most one sniff window. A payload cut at
// the window boundary ends mid-quantum, which the decoders report as an
// expected error after yielding the leading bytes.
func decodeBase64MediaSample(payload string) ([]byte, bool) {
	if payload == "" {
		return nil, false
	}
	maxEncoded := (MediaSniffSampleBytes + 2) / 3 * 4
	if len(payload) > maxEncoded {
		payload = payload[:maxEncoded]
	}
	encodings := []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding}
	if strings.ContainsAny(payload, "-_") {
		encodings = []*base64.Encoding{base64.URLEncoding, base64.RawURLEncoding}
	}
	sample := make([]byte, MediaSniffSampleBytes)
	var partial []byte
	for _, encoding := range encodings {
		decoder := base64.NewDecoder(encoding, strings.NewReader(payload))
		n, err := io.ReadFull(decoder, sample)
		if err == nil || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			if n > 0 {
				return sample[:n], true
			}
			continue
		}
		// Padding or an alphabet mismatch after the prefix still yields the
		// bytes before it, which is all detection inspects.
		if n > 0 && partial == nil {
			partial = append([]byte(nil), sample[:n]...)
		}
	}
	return partial, partial != nil
}
