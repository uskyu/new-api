package service

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
	"golang.org/x/image/draw"

	// 注册 WebP 解码器，用于校验真实文件格式。
	_ "golang.org/x/image/webp"
)

const (
	// DefaultSupportAttachmentDir 相对当前工作目录；容器内 WORKDIR 为 /data。
	DefaultSupportAttachmentDir = "./support-attachments"

	// MaxSupportAttachmentBytes 单张图片最大 5MB。
	MaxSupportAttachmentBytes int64 = 5 << 20
	// MaxSupportAttachmentsPerMessage 每条消息最多 4 张图片。
	MaxSupportAttachmentsPerMessage = 4
	// MaxSupportMessageAttachmentBytes 单条消息附件总大小最大 20MB。
	MaxSupportMessageAttachmentBytes int64 = 20 << 20
	// MaxSupportAttachmentPixels 图片最大像素总数。
	MaxSupportAttachmentPixels int64 = 50_000_000

	SupportAttachmentThumbnailMaxSide  = 640
	supportAttachmentThumbnailQuality  = 80
	supportAttachmentThumbnailReuseMax = 200 << 10
)

var supportAttachmentMimeByFormat = map[string]string{
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"gif":  "image/gif",
	"webp": "image/webp",
}

var supportAttachmentExtByFormat = map[string]string{
	"jpeg": "jpg",
	"png":  "png",
	"gif":  "gif",
	"webp": "webp",
}

// StoredSupportAttachment 已写入磁盘的附件信息。
type StoredSupportAttachment struct {
	StorageKey        string
	ThumbnailKey      string
	ThumbnailMimeType string
	MimeType          string
	FileName          string
	SizeBytes         int64
	Width             int
	Height            int
}

// GetSupportAttachmentRootDir 返回附件根目录的绝对路径，并确保目录存在。
func GetSupportAttachmentRootDir() string {
	dir, err := supportAttachmentRootDir()
	if err != nil {
		common.SysError("failed to prepare support attachment directory: " + err.Error())
	}
	return dir
}

func supportAttachmentRootDir() (string, error) {
	dir := strings.TrimSpace(os.Getenv("SUPPORT_ATTACHMENT_DIR"))
	if dir == "" {
		dir = DefaultSupportAttachmentDir
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return abs, err
	}
	return abs, nil
}

// SaveSupportAttachment 校验并保存图片及其缩略图，返回存储信息。
func SaveSupportAttachment(conversationId int, file multipart.File, header *multipart.FileHeader) (*StoredSupportAttachment, error) {
	if conversationId <= 0 {
		return nil, errors.New("无效的会话 ID")
	}
	if file == nil {
		return nil, errors.New("图片文件为空")
	}

	data, err := io.ReadAll(io.LimitReader(file, MaxSupportAttachmentBytes+1))
	if err != nil {
		return nil, errors.New("读取图片失败")
	}
	if int64(len(data)) > MaxSupportAttachmentBytes {
		return nil, errors.New("图片不能超过 5MB")
	}
	if len(data) == 0 {
		return nil, errors.New("图片文件为空")
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("仅支持 JPG、PNG、GIF、WebP 图片")
	}
	mimeType, ok := supportAttachmentMimeByFormat[format]
	if !ok {
		return nil, errors.New("仅支持 JPG、PNG、GIF、WebP 图片")
	}
	ext := supportAttachmentExtByFormat[format]
	if config.Width <= 0 || config.Height <= 0 {
		return nil, errors.New("图片尺寸无效")
	}
	if int64(config.Width)*int64(config.Height) > MaxSupportAttachmentPixels {
		return nil, errors.New("图片尺寸过大")
	}

	// 完整解码一次，损坏的图片在写入磁盘前直接拒绝。
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("图片文件已损坏，无法读取")
	}

	root, err := supportAttachmentRootDir()
	if err != nil {
		return nil, fmt.Errorf("创建附件目录失败: %w", err)
	}

	attachmentUUID := uuid.New().String()
	monthPath := time.Now().UTC().Format("200601")
	storageKey := fmt.Sprintf("%d/%s/%s.%s", conversationId, monthPath, attachmentUUID, ext)
	dir := filepath.Join(root, filepath.FromSlash(path.Dir(storageKey)))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建附件目录失败: %w", err)
	}

	fileName := sanitizeSupportAttachmentFileName(header, ext)
	objectName := attachmentUUID + "." + ext
	originalPath, err := writeSupportAttachmentFile(dir, objectName, data)
	if err != nil {
		return nil, fmt.Errorf("保存图片失败: %w", err)
	}

	stored := &StoredSupportAttachment{
		StorageKey:   storageKey,
		ThumbnailKey: storageKey,
		MimeType:     mimeType,
		FileName:     fileName,
		SizeBytes:    int64(len(data)),
		Width:        config.Width,
		Height:       config.Height,
	}

	needThumbnail := int64(len(data)) >= supportAttachmentThumbnailReuseMax ||
		config.Width > SupportAttachmentThumbnailMaxSide ||
		config.Height > SupportAttachmentThumbnailMaxSide
	if !needThumbnail {
		stored.ThumbnailMimeType = mimeType
		return stored, nil
	}

	thumbnailData, err := buildSupportAttachmentThumbnail(decoded)
	if err != nil {
		_ = os.Remove(originalPath)
		return nil, fmt.Errorf("生成缩略图失败: %w", err)
	}
	thumbnailName := attachmentUUID + "_thumb.jpg"
	if _, err := writeSupportAttachmentFile(dir, thumbnailName, thumbnailData); err != nil {
		_ = os.Remove(originalPath)
		return nil, fmt.Errorf("保存缩略图失败: %w", err)
	}

	stored.ThumbnailKey = fmt.Sprintf("%d/%s/%s", conversationId, monthPath, thumbnailName)
	stored.ThumbnailMimeType = "image/jpeg"
	return stored, nil
}

func sanitizeSupportAttachmentFileName(header *multipart.FileHeader, ext string) string {
	fallback := "image." + ext
	if header == nil {
		return fallback
	}
	name := filepath.Base(strings.TrimSpace(header.Filename))
	switch name {
	case "", ".", "..", "/", "\\":
		return fallback
	}
	if strings.ContainsAny(name, "/\\") {
		name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
		if name == "" || name == "." || name == ".." {
			return fallback
		}
	}
	if strings.ContainsRune(name, '\x00') {
		return fallback
	}
	runes := []rune(name)
	if len(runes) > 255 {
		name = string(runes[:255])
	}
	if strings.TrimSpace(name) == "" {
		return fallback
	}
	return name
}

func buildSupportAttachmentThumbnail(source image.Image) ([]byte, error) {
	bounds := source.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, errors.New("图片尺寸无效")
	}

	targetWidth := width
	targetHeight := height
	if width > SupportAttachmentThumbnailMaxSide || height > SupportAttachmentThumbnailMaxSide {
		if width >= height {
			targetWidth = SupportAttachmentThumbnailMaxSide
			targetHeight = int(int64(height) * int64(SupportAttachmentThumbnailMaxSide) / int64(width))
		} else {
			targetHeight = SupportAttachmentThumbnailMaxSide
			targetWidth = int(int64(width) * int64(SupportAttachmentThumbnailMaxSide) / int64(height))
		}
	}
	if targetWidth < 1 {
		targetWidth = 1
	}
	if targetHeight < 1 {
		targetHeight = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), source, bounds, draw.Over, nil)

	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, dst, &jpeg.Options{Quality: supportAttachmentThumbnailQuality}); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeSupportAttachmentFile(dir string, name string, data []byte) (string, error) {
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	finalPath := filepath.Join(dir, name)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", err
	}
	keepTemp = false
	return finalPath, nil
}

// ResolveSupportAttachmentPath 将存储 key 解析为根目录下的绝对路径，拒绝路径穿越。
func ResolveSupportAttachmentPath(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("附件路径为空")
	}
	if filepath.IsAbs(key) || strings.HasPrefix(key, "/") || strings.HasPrefix(key, "\\") {
		return "", errors.New("附件路径非法")
	}
	if filepath.VolumeName(key) != "" {
		return "", errors.New("附件路径非法")
	}
	key = strings.ReplaceAll(key, "\\", "/")
	if strings.Contains(key, "..") {
		return "", errors.New("附件路径非法")
	}

	root, err := supportAttachmentRootDir()
	if err != nil {
		return "", err
	}
	finalPath := filepath.Join(root, filepath.FromSlash(key))
	rel, err := filepath.Rel(root, finalPath)
	if err != nil {
		return "", errors.New("附件路径非法")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("附件路径非法")
	}
	return finalPath, nil
}

// RemoveSupportConversationDir 删除会话对应的附件目录，目录不存在视为成功。
func RemoveSupportConversationDir(conversationId int) error {
	if conversationId <= 0 {
		return errors.New("无效的会话 ID")
	}
	root, err := supportAttachmentRootDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, strconv.Itoa(conversationId))
	info, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("附件路径不是目录: %s", dir)
	}
	return os.RemoveAll(dir)
}

// RemoveSupportAttachmentFile 删除单个存储 key 对应的文件，文件不存在视为成功。
func RemoveSupportAttachmentFile(storageKey string) error {
	finalPath, err := ResolveSupportAttachmentPath(storageKey)
	if err != nil {
		return err
	}
	if err := os.Remove(finalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// RemoveSupportAttachmentThumbnail 删除缩略图文件；缩略图复用原图时不做操作。
func RemoveSupportAttachmentThumbnail(stored StoredSupportAttachment) error {
	if stored.ThumbnailKey == "" || stored.ThumbnailKey == stored.StorageKey {
		return nil
	}
	return RemoveSupportAttachmentFile(stored.ThumbnailKey)
}

// CleanupStoredSupportAttachments 删除一次上传中已经写入的全部文件，用于失败回滚。
func CleanupStoredSupportAttachments(stored []StoredSupportAttachment) {
	for _, item := range stored {
		if err := RemoveSupportAttachmentThumbnail(item); err != nil {
			common.SysError("failed to remove support attachment thumbnail: " + err.Error())
		}
		if err := RemoveSupportAttachmentFile(item.StorageKey); err != nil {
			common.SysError("failed to remove support attachment file: " + err.Error())
		}
	}
}

// CleanupOrphanSupportAttachmentFiles 删除 24 小时前残留的 .upload- 临时文件。
func CleanupOrphanSupportAttachmentFiles() error {
	root, err := supportAttachmentRootDir()
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	var errs []error
	walkErr := filepath.WalkDir(root, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			errs = append(errs, walkErr)
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasPrefix(filepath.Base(currentPath), ".upload-") {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			errs = append(errs, infoErr)
			return nil
		}
		if !info.ModTime().Before(cutoff) {
			return nil
		}
		if removeErr := os.Remove(currentPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			errs = append(errs, removeErr)
		}
		return nil
	})
	if walkErr != nil {
		errs = append(errs, walkErr)
	}
	return errors.Join(errs...)
}
