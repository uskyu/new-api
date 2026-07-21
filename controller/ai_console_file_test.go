package controller

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type aiConsoleFileTestEnvelope struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message"`
	Data    aiConsoleFileParseResponse `json:"data"`
}

func newAIConsoleFileRequest(t *testing.T, filename string, content []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/api/ai-console/files/parse", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestParseAIConsoleFileReturnsPlainTextAndSanitizedFilename(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = newAIConsoleFileRequest(t, `folder\notes.txt`, append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello")...))

	ParseAIConsoleFile(context)

	var response aiConsoleFileTestEnvelope
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.Equal(t, "notes.txt", response.Data.Filename)
	assert.Equal(t, "hello", response.Data.Markdown)
}

func TestParseAIConsoleFileRejectsUnsupportedExtensions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = newAIConsoleFileRequest(t, "payload.exe", []byte("not executable"))

	ParseAIConsoleFile(context)

	var response aiConsoleFileTestEnvelope
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "not supported")
}

func TestLimitAIConsoleRunesPreservesUnicodeBoundary(t *testing.T) {
	result, truncated := limitAIConsoleRunes("a你好", 2)

	assert.True(t, truncated)
	assert.Equal(t, "a你", result)
}
