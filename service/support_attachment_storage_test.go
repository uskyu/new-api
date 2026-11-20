package service

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"math/rand"
	"mime/multipart"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type supportTestMultipartFile struct {
	*bytes.Reader
}

func (supportTestMultipartFile) Close() error {
	return nil
}

func setupSupportAttachmentTestDir(t *testing.T) string {
	t.Helper()
	previous, hadPrevious := os.LookupEnv("SUPPORT_ATTACHMENT_DIR")
	dir := filepath.Join(t.TempDir(), "support-attachments")
	require.NoError(t, os.Setenv("SUPPORT_ATTACHMENT_DIR", dir))
	t.Cleanup(func() {
		if hadPrevious {
			_ = os.Setenv("SUPPORT_ATTACHMENT_DIR", previous)
		} else {
			_ = os.Unsetenv("SUPPORT_ATTACHMENT_DIR")
		}
	})
	return dir
}

func encodeSupportTestPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 40, G: 120, B: 200, A: 255}}, image.Point{}, draw.Src)
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, img))
	return buffer.Bytes()
}

func encodeSupportTestNoisyPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	_, err := rand.Read(img.Pix)
	require.NoError(t, err)
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, img))
	return buffer.Bytes()
}

func encodeSupportTestGIF(t *testing.T) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.White, color.Black})
	img.SetColorIndex(0, 0, 0)
	var buffer bytes.Buffer
	require.NoError(t, gif.Encode(&buffer, img, nil))
	return buffer.Bytes()
}

func saveSupportTestImage(t *testing.T, conversationId int, fileName string, data []byte) *StoredSupportAttachment {
	t.Helper()
	file := supportTestMultipartFile{bytes.NewReader(data)}
	header := &multipart.FileHeader{Filename: fileName, Size: int64(len(data))}
	stored, err := SaveSupportAttachment(conversationId, file, header)
	require.NoError(t, err)
	require.NotNil(t, stored)
	return stored
}

func supportTestOversizedPNGHeader(width uint32, height uint32) []byte {
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[0:4], width)
	binary.BigEndian.PutUint32(header[4:8], height)
	header[8] = 8 // bit depth
	header[9] = 6 // color type: truecolor with alpha
	crc := crc32.ChecksumIEEE(append([]byte("IHDR"), header...))

	result := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	result = append(result, 0, 0, 0, 13)
	result = append(result, 'I', 'H', 'D', 'R')
	result = append(result, header...)
	result = append(result, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))
	return result
}

func TestSaveSupportAttachmentSmallImageReusesOriginal(t *testing.T) {
	dir := setupSupportAttachmentTestDir(t)
	data := encodeSupportTestPNG(t, 100, 80)

	stored := saveSupportTestImage(t, 7, "C:\\fake\\path\\photo.png", data)
	assert.Equal(t, "photo.png", stored.FileName)
	assert.Equal(t, "image/png", stored.MimeType)
	assert.Equal(t, 100, stored.Width)
	assert.Equal(t, 80, stored.Height)
	assert.Equal(t, int64(len(data)), stored.SizeBytes)
	assert.Equal(t, stored.StorageKey, stored.ThumbnailKey)
	assert.Equal(t, "image/png", stored.ThumbnailMimeType)
	assert.True(t, strings.HasPrefix(stored.StorageKey, "7/"))
	assert.True(t, strings.HasSuffix(stored.StorageKey, ".png"))

	filePath, err := ResolveSupportAttachmentPath(stored.StorageKey)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(filePath, dir))
	onDisk, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, data, onDisk)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	}
}

func TestSaveSupportAttachmentGIF(t *testing.T) {
	setupSupportAttachmentTestDir(t)
	stored := saveSupportTestImage(t, 8, "anim.gif", encodeSupportTestGIF(t))
	assert.Equal(t, "image/gif", stored.MimeType)
	assert.Equal(t, "image/gif", stored.ThumbnailMimeType)
	assert.True(t, strings.HasSuffix(stored.StorageKey, ".gif"))
	assert.Equal(t, stored.StorageKey, stored.ThumbnailKey)
}

func TestSaveSupportAttachmentLargeImageCreatesThumbnail(t *testing.T) {
	dir := setupSupportAttachmentTestDir(t)
	data := encodeSupportTestPNG(t, 1200, 800)

	stored := saveSupportTestImage(t, 9, "large.png", data)
	assert.Equal(t, "image/png", stored.MimeType)
	assert.Equal(t, 1200, stored.Width)
	assert.Equal(t, 800, stored.Height)
	assert.NotEqual(t, stored.StorageKey, stored.ThumbnailKey)
	assert.Equal(t, "image/jpeg", stored.ThumbnailMimeType)
	assert.True(t, strings.HasSuffix(stored.ThumbnailKey, "_thumb.jpg"))

	thumbnailPath, err := ResolveSupportAttachmentPath(stored.ThumbnailKey)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(thumbnailPath, dir))
	thumbnailFile, err := os.Open(thumbnailPath)
	require.NoError(t, err)
	defer thumbnailFile.Close()
	thumbnailConfig, format, err := image.DecodeConfig(thumbnailFile)
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	assert.Equal(t, SupportAttachmentThumbnailMaxSide, thumbnailConfig.Width)
	assert.InDelta(t, 800.0*SupportAttachmentThumbnailMaxSide/1200.0, float64(thumbnailConfig.Height), 1.0)
}

func TestSaveSupportAttachmentLargeFileSmallDimensionsDoesNotUpscale(t *testing.T) {
	setupSupportAttachmentTestDir(t)
	data := encodeSupportTestNoisyPNG(t, 500, 500)
	require.Greater(t, len(data), supportAttachmentThumbnailReuseMax)

	stored := saveSupportTestImage(t, 13, "noisy.png", data)
	assert.NotEqual(t, stored.StorageKey, stored.ThumbnailKey)
	assert.Equal(t, "image/jpeg", stored.ThumbnailMimeType)

	thumbnailPath, err := ResolveSupportAttachmentPath(stored.ThumbnailKey)
	require.NoError(t, err)
	thumbnailFile, err := os.Open(thumbnailPath)
	require.NoError(t, err)
	defer thumbnailFile.Close()
	thumbnailConfig, format, err := image.DecodeConfig(thumbnailFile)
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	assert.Equal(t, 500, thumbnailConfig.Width)
	assert.Equal(t, 500, thumbnailConfig.Height)
}

func TestSaveSupportAttachmentValidation(t *testing.T) {
	setupSupportAttachmentTestDir(t)

	tooLargeData := bytes.Repeat([]byte{0}, int(MaxSupportAttachmentBytes)+1)
	file := supportTestMultipartFile{bytes.NewReader(tooLargeData)}
	header := &multipart.FileHeader{Filename: "too-large.png", Size: int64(len(tooLargeData))}
	_, err := SaveSupportAttachment(10, file, header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "图片不能超过 5MB")

	oversizedData := supportTestOversizedPNGHeader(8000, 7000)
	file = supportTestMultipartFile{bytes.NewReader(oversizedData)}
	header = &multipart.FileHeader{Filename: "oversized.png", Size: int64(len(oversizedData))}
	_, err = SaveSupportAttachment(10, file, header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "图片尺寸过大")

	invalidData := []byte("<svg><rect width=\"10\" height=\"10\"/></svg>")
	file = supportTestMultipartFile{bytes.NewReader(invalidData)}
	header = &multipart.FileHeader{Filename: "image.svg", Size: int64(len(invalidData))}
	_, err = SaveSupportAttachment(10, file, header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "仅支持 JPG、PNG、GIF、WebP 图片")

	emptyFile := supportTestMultipartFile{bytes.NewReader(nil)}
	header = &multipart.FileHeader{Filename: "empty.png", Size: 0}
	_, err = SaveSupportAttachment(10, emptyFile, header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "图片文件为空")

	_, err = SaveSupportAttachment(0, emptyFile, header)
	require.Error(t, err)
}

func TestResolveSupportAttachmentPathRejectsTraversal(t *testing.T) {
	dir := setupSupportAttachmentTestDir(t)

	for _, key := range []string{
		"",
		"../escape.png",
		"a/../../escape.png",
		"/etc/passwd",
		"\\absolute\\path.png",
	} {
		_, err := ResolveSupportAttachmentPath(key)
		require.Error(t, err, "key %q should be rejected", key)
	}

	root := GetSupportAttachmentRootDir()
	assert.Equal(t, dir, root)
	validPath, err := ResolveSupportAttachmentPath("12/202601/abc.png")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(validPath, root))
}

func TestRemoveSupportConversationDirAndStoredFiles(t *testing.T) {
	setupSupportAttachmentTestDir(t)

	storedOne := saveSupportTestImage(t, 11, "one.png", encodeSupportTestPNG(t, 20, 20))
	storedTwo := saveSupportTestImage(t, 11, "two.png", encodeSupportTestPNG(t, 30, 30))
	conversationDir := filepath.Join(GetSupportAttachmentRootDir(), "11")
	_, err := os.Stat(conversationDir)
	require.NoError(t, err)

	require.NoError(t, RemoveSupportConversationDir(11))
	_, err = os.Stat(conversationDir)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(GetSupportAttachmentRootDir())
	require.NoError(t, err)
	require.NoError(t, RemoveSupportConversationDir(11))

	for _, stored := range []StoredSupportAttachment{*storedOne, *storedTwo} {
		path, resolveErr := ResolveSupportAttachmentPath(stored.StorageKey)
		require.NoError(t, resolveErr)
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr))
	}
}

func TestCleanupStoredSupportAttachmentsRemovesThumbnailAndOriginal(t *testing.T) {
	setupSupportAttachmentTestDir(t)
	stored := saveSupportTestImage(t, 12, "large.png", encodeSupportTestPNG(t, 1200, 800))

	CleanupStoredSupportAttachments([]StoredSupportAttachment{*stored})
	originalPath, err := ResolveSupportAttachmentPath(stored.StorageKey)
	require.NoError(t, err)
	_, err = os.Stat(originalPath)
	assert.True(t, os.IsNotExist(err))
	thumbnailPath, err := ResolveSupportAttachmentPath(stored.ThumbnailKey)
	require.NoError(t, err)
	_, err = os.Stat(thumbnailPath)
	assert.True(t, os.IsNotExist(err))
}

func TestCleanupOrphanSupportAttachmentFiles(t *testing.T) {
	root := setupSupportAttachmentTestDir(t)
	require.NoError(t, os.MkdirAll(root, 0o755))

	oldTemp := filepath.Join(root, ".upload-old")
	require.NoError(t, os.WriteFile(oldTemp, []byte("old"), 0o600))
	oldTime := time.Now().Add(-25 * time.Hour)
	require.NoError(t, os.Chtimes(oldTemp, oldTime, oldTime))

	newTemp := filepath.Join(root, ".upload-new")
	require.NoError(t, os.WriteFile(newTemp, []byte("new"), 0o600))
	normalFile := filepath.Join(root, "keep.png")
	require.NoError(t, os.WriteFile(normalFile, []byte("keep"), 0o600))

	require.NoError(t, CleanupOrphanSupportAttachmentFiles())
	_, err := os.Stat(oldTemp)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(newTemp)
	require.NoError(t, err)
	_, err = os.Stat(normalFile)
	require.NoError(t, err)
}

func TestSupportAttachmentStorageKeyUsesConversationDirectory(t *testing.T) {
	setupSupportAttachmentTestDir(t)
	stored := saveSupportTestImage(t, 42, "image.png", encodeSupportTestPNG(t, 10, 10))
	parts := strings.Split(stored.StorageKey, "/")
	require.Len(t, parts, 3)
	assert.Equal(t, "42", parts[0])
	assert.Len(t, parts[1], 6)
}
