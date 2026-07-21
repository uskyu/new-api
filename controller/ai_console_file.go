package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const (
	aiConsoleFileMaxBytes       = 20 << 20
	aiConsoleRequestMaxBytes    = aiConsoleFileMaxBytes + (1 << 20)
	aiConsoleMarkdownMaxRunes   = 30000
	aiConsoleParserOutputMaxLen = 2 << 20
	aiConsoleParseTimeout       = 90 * time.Second
)

type aiConsoleFileParseResponse struct {
	Filename string   `json:"filename"`
	Markdown string   `json:"markdown"`
	Warnings []string `json:"warnings,omitempty"`
}

type aiConsoleBoundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *aiConsoleBoundedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		writeLength := min(remaining, len(p))
		_, _ = b.buffer.Write(p[:writeLength])
	}
	return len(p), nil
}

func (b *aiConsoleBoundedBuffer) String() string {
	return b.buffer.String()
}

var aiConsoleAllowedFileExts = map[string]bool{
	".txt":  true,
	".md":   true,
	".csv":  true,
	".json": true,
	".log":  true,
	".pdf":  true,
	".docx": true,
	".xlsx": true,
	".pptx": true,
}

func ParseAIConsoleFile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, aiConsoleRequestMaxBytes)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "Please upload a supported file")
		return
	}
	defer file.Close()

	if header == nil || strings.TrimSpace(header.Filename) == "" {
		common.ApiErrorMsg(c, "The file name cannot be empty")
		return
	}
	if header.Size < 0 || header.Size > aiConsoleFileMaxBytes {
		common.ApiErrorMsg(c, "The file must not exceed 20 MB")
		return
	}

	filename := path.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	ext := strings.ToLower(filepath.Ext(filename))
	if !aiConsoleAllowedFileExts[ext] {
		common.ApiErrorMsg(c, "This file type is not supported")
		return
	}

	content, err := parseAIConsoleUploadedFile(c.Request.Context(), file, ext)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	markdown, truncated := limitAIConsoleRunes(strings.TrimSpace(content), aiConsoleMarkdownMaxRunes)
	if markdown == "" {
		common.ApiErrorMsg(c, "No readable content was found in the file")
		return
	}

	warnings := make([]string, 0, 1)
	if truncated {
		warnings = append(warnings, "The file was truncated to the first 30,000 characters")
	}

	common.ApiSuccess(c, aiConsoleFileParseResponse{
		Filename: filename,
		Markdown: markdown,
		Warnings: warnings,
	})
}

func parseAIConsoleUploadedFile(ctx context.Context, file multipart.File, ext string) (string, error) {
	if isAIConsolePlainTextExt(ext) {
		data, err := io.ReadAll(io.LimitReader(file, aiConsoleFileMaxBytes+1))
		if err != nil {
			return "", fmt.Errorf("File parsing failed")
		}
		if len(data) > aiConsoleFileMaxBytes {
			return "", fmt.Errorf("The file must not exceed 20 MB")
		}
		return string(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})), nil
	}

	if _, err := exec.LookPath("markitdown"); err != nil {
		return "", fmt.Errorf("Office file parsing is not enabled on this server")
	}

	tmpFile, err := os.CreateTemp("", "ai-console-*"+ext)
	if err != nil {
		return "", fmt.Errorf("File parsing failed")
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	written, copyErr := io.Copy(tmpFile, io.LimitReader(file, aiConsoleFileMaxBytes+1))
	closeErr := tmpFile.Close()
	if copyErr != nil || closeErr != nil {
		return "", fmt.Errorf("File parsing failed")
	}
	if written > aiConsoleFileMaxBytes {
		return "", fmt.Errorf("The file must not exceed 20 MB")
	}

	parseCtx, cancel := context.WithTimeout(ctx, aiConsoleParseTimeout)
	defer cancel()

	stdout := &aiConsoleBoundedBuffer{limit: aiConsoleParserOutputMaxLen}
	stderr := &aiConsoleBoundedBuffer{limit: 8 << 10}
	cmd := exec.CommandContext(parseCtx, "markitdown", tmpPath)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	if parseCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("File parsing timed out")
	}
	if err != nil {
		return "", fmt.Errorf("File parsing failed")
	}
	return stdout.String(), nil
}

func isAIConsolePlainTextExt(ext string) bool {
	switch ext {
	case ".txt", ".md", ".csv", ".json", ".log":
		return true
	default:
		return false
	}
}

func limitAIConsoleRunes(value string, maxRunes int) (string, bool) {
	if maxRunes <= 0 {
		return "", value != ""
	}
	if utf8.RuneCountInString(value) <= maxRunes {
		return value, false
	}
	runes := []rune(value)
	return string(runes[:maxRunes]), true
}
