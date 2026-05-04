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
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const (
	aiConsoleFileMaxBytes    = 20 << 20
	aiConsoleMarkdownMaxRune = 30000
	aiConsoleParseTimeout    = 90 * time.Second
)

type aiConsoleFileParseResponse struct {
	Filename string   `json:"filename"`
	Markdown string   `json:"markdown"`
	Warnings []string `json:"warnings,omitempty"`
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
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, aiConsoleFileMaxBytes)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "请上传文件")
		return
	}
	defer file.Close()

	if header == nil || header.Filename == "" {
		common.ApiErrorMsg(c, "文件名不能为空")
		return
	}
	if header.Size > aiConsoleFileMaxBytes {
		common.ApiErrorMsg(c, "文件不能超过 20MB")
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !aiConsoleAllowedFileExts[ext] {
		common.ApiErrorMsg(c, "暂不支持该文件类型")
		return
	}

	content, err := parseAIConsoleUploadedFile(c.Request.Context(), file, header, ext)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	markdown, truncated := limitRunes(strings.TrimSpace(content), aiConsoleMarkdownMaxRune)
	if markdown == "" {
		common.ApiErrorMsg(c, "未能从文件中提取到有效内容")
		return
	}

	warnings := make([]string, 0, 1)
	if truncated {
		warnings = append(warnings, "文件内容较长，已截取前 30000 字")
	}

	common.ApiSuccess(c, aiConsoleFileParseResponse{
		Filename: filepath.Base(header.Filename),
		Markdown: markdown,
		Warnings: warnings,
	})
}

func parseAIConsoleUploadedFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, ext string) (string, error) {
	if isAIConsolePlainTextExt(ext) {
		data, err := io.ReadAll(io.LimitReader(file, aiConsoleFileMaxBytes+1))
		if err != nil {
			return "", fmt.Errorf("读取文件失败")
		}
		return string(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})), nil
	}

	if _, err := exec.LookPath("markitdown"); err != nil {
		return "", fmt.Errorf("服务器暂未启用办公文件解析服务，请联系管理员或先上传 txt、md、csv、json、log 文件")
	}

	tmpFile, err := os.CreateTemp("", "ai-console-*"+ext)
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败")
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, io.LimitReader(file, aiConsoleFileMaxBytes+1)); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("保存临时文件失败")
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("保存临时文件失败")
	}

	parseCtx, cancel := context.WithTimeout(ctx, aiConsoleParseTimeout)
	defer cancel()

	cmd := exec.CommandContext(parseCtx, "markitdown", tmpPath)
	output, err := cmd.CombinedOutput()
	if parseCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("文件解析超时")
	}
	if err != nil {
		return "", fmt.Errorf("文件解析失败：%s", strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func isAIConsolePlainTextExt(ext string) bool {
	switch ext {
	case ".txt", ".md", ".csv", ".json", ".log":
		return true
	default:
		return false
	}
}

func limitRunes(value string, maxRunes int) (string, bool) {
	if maxRunes <= 0 {
		return "", value != ""
	}
	if utf8.RuneCountInString(value) <= maxRunes {
		return value, false
	}
	runes := []rune(value)
	return string(runes[:maxRunes]), true
}
