package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const AIImageObjectURLTTL = 7 * 24 * time.Hour

func IsObjectStorageEnabled() bool {
	setting := operation_setting.GetAIImageAsyncSetting()
	return setting.S3Enabled &&
		strings.TrimSpace(setting.S3Endpoint) != "" &&
		strings.TrimSpace(setting.S3Bucket) != "" &&
		strings.TrimSpace(setting.S3AccessKey) != "" &&
		strings.TrimSpace(setting.S3SecretKey) != ""
}

func normalizeObjectStorageEndpoint(raw string) (string, error) {
	endpoint := strings.TrimSpace(raw)
	if endpoint == "" {
		return "", fmt.Errorf("object storage endpoint is empty")
	}
	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", fmt.Errorf("invalid object storage endpoint: %w", err)
		}
		if parsed.Host == "" {
			return "", fmt.Errorf("invalid object storage endpoint host")
		}
		if parsed.Path != "" && parsed.Path != "/" {
			return "", fmt.Errorf("object storage endpoint should not include a path")
		}
		endpoint = parsed.Host
	}
	endpoint = strings.Trim(endpoint, "/")
	if endpoint == "" {
		return "", fmt.Errorf("invalid object storage endpoint")
	}
	if strings.Contains(endpoint, "/") {
		return "", fmt.Errorf("object storage endpoint should only contain host:port")
	}
	return endpoint, nil
}

func validateObjectStorageConfig(setting *operation_setting.AIImageAsyncSetting, endpoint string) error {
	region := strings.TrimSpace(setting.S3Region)
	if region == "" {
		region = "auto"
	}
	if strings.HasPrefix(strings.TrimSpace(setting.S3AccessKey), "AKID") && strings.EqualFold(region, "auto") {
		return fmt.Errorf("Tencent COS requires an explicit region, for example ap-guangzhou, instead of auto")
	}
	if strings.Contains(endpoint, ".myqcloud.com") && strings.EqualFold(region, "auto") {
		return fmt.Errorf("Tencent COS requires an explicit region, for example ap-guangzhou, instead of auto")
	}
	return nil
}

func NewObjectStorageClient() (*minio.Client, error) {
	setting := operation_setting.GetAIImageAsyncSetting()
	if !IsObjectStorageEnabled() {
		return nil, fmt.Errorf("object storage is not enabled")
	}
	endpoint, err := normalizeObjectStorageEndpoint(setting.S3Endpoint)
	if err != nil {
		return nil, err
	}
	if err := validateObjectStorageConfig(setting, endpoint); err != nil {
		return nil, err
	}
	region := strings.TrimSpace(setting.S3Region)
	if region == "" {
		region = "auto"
	}
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(strings.TrimSpace(setting.S3AccessKey), strings.TrimSpace(setting.S3SecretKey), ""),
		Secure: setting.S3UseSSL,
		Region: region,
	})
}

func BuildAIImageObjectKey(userID int, ext string) string {
	setting := operation_setting.GetAIImageAsyncSetting()
	prefix := strings.Trim(setting.S3PathPrefix, "/")
	if prefix == "" {
		prefix = "ai-image"
	}
	if ext == "" {
		ext = "png"
	}
	now := time.Now().UTC()
	return path.Join(prefix, fmt.Sprintf("%d", userID), now.Format("2006"), now.Format("01"), fmt.Sprintf("%d.%s", now.UnixNano(), ext))
}

func BuildAIImageRefObjectKey(userID int, ext string) string {
	setting := operation_setting.GetAIImageAsyncSetting()
	prefix := strings.Trim(setting.S3PathPrefix, "/")
	if prefix == "" {
		prefix = "ai-image"
	}
	refPrefix := prefix + "-ref"
	if ext == "" {
		ext = "png"
	}
	now := time.Now().UTC()
	return path.Join(refPrefix, fmt.Sprintf("%d", userID), now.Format("2006"), now.Format("01"), fmt.Sprintf("%d.%s", now.UnixNano(), ext))
}

func UploadBytesToObjectStorage(ctx context.Context, objectKey string, contentType string, data []byte) (string, string, error) {
	client, err := NewObjectStorageClient()
	if err != nil {
		return "", "", err
	}
	setting := operation_setting.GetAIImageAsyncSetting()
	bucket := strings.TrimSpace(setting.S3Bucket)
	_, err = client.PutObject(ctx, bucket, objectKey, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", "", fmt.Errorf("put object to storage failed (bucket=%s, key=%s, content_type=%s): %w", bucket, objectKey, contentType, err)
	}
	accessURL, err := GenerateObjectStorageAccessURL(ctx, objectKey)
	if err != nil {
		return "", "", fmt.Errorf("generate object access url failed (bucket=%s, key=%s): %w", bucket, objectKey, err)
	}
	return objectKey, accessURL, nil
}

func DeleteObjectFromStorage(ctx context.Context, objectKey string) error {
	client, err := NewObjectStorageClient()
	if err != nil {
		return err
	}
	setting := operation_setting.GetAIImageAsyncSetting()
	return client.RemoveObject(ctx, strings.TrimSpace(setting.S3Bucket), objectKey, minio.RemoveObjectOptions{})
}

func DownloadObjectFromStorage(ctx context.Context, objectKey string) ([]byte, string, error) {
	client, err := NewObjectStorageClient()
	if err != nil {
		return nil, "", err
	}
	setting := operation_setting.GetAIImageAsyncSetting()
	bucket := strings.TrimSpace(setting.S3Bucket)
	obj, err := client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", err
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", err
	}
	contentType := "image/png"
	objInfo, err := client.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
	if err == nil && objInfo.ContentType != "" {
		contentType = objInfo.ContentType
	}
	return data, contentType, nil
}

func GenerateObjectStorageAccessURL(ctx context.Context, objectKey string) (string, error) {
	client, err := NewObjectStorageClient()
	if err != nil {
		return "", err
	}
	setting := operation_setting.GetAIImageAsyncSetting()
	publicBaseURL := strings.TrimSpace(setting.S3PublicBaseURL)
	if publicBaseURL != "" {
		return strings.TrimRight(publicBaseURL, "/") + "/" + strings.TrimLeft(objectKey, "/"), nil
	}
	bucket := strings.TrimSpace(setting.S3Bucket)
	presignedURL, err := client.PresignedGetObject(ctx, bucket, objectKey, AIImageObjectURLTTL, url.Values{})
	if err != nil {
		return "", fmt.Errorf("presign object failed (bucket=%s, key=%s): %w", bucket, objectKey, err)
	}
	return presignedURL.String(), nil
}
