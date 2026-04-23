package service

import (
	"bytes"
	"context"
	"fmt"
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
	return setting.S3Enabled && setting.S3Endpoint != "" && setting.S3Bucket != "" && setting.S3AccessKey != "" && setting.S3SecretKey != ""
}

func NewObjectStorageClient() (*minio.Client, error) {
	setting := operation_setting.GetAIImageAsyncSetting()
	if !IsObjectStorageEnabled() {
		return nil, fmt.Errorf("object storage is not enabled")
	}
	return minio.New(setting.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(setting.S3AccessKey, setting.S3SecretKey, ""),
		Secure: setting.S3UseSSL,
		Region: setting.S3Region,
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
	return path.Join(prefix, fmt.Sprintf("%d", userID), now.Format("2006"), now.Format("01"), fmt.Sprintf("%s.%s", strings.ReplaceAll(strings.ReplaceAll(now.Format(time.RFC3339Nano), ":", ""), ".", ""), ext))
}

func UploadBytesToObjectStorage(ctx context.Context, objectKey string, contentType string, data []byte) (string, string, error) {
	client, err := NewObjectStorageClient()
	if err != nil {
		return "", "", err
	}
	setting := operation_setting.GetAIImageAsyncSetting()
	_, err = client.PutObject(ctx, setting.S3Bucket, objectKey, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", "", err
	}
	accessURL, err := GenerateObjectStorageAccessURL(ctx, objectKey)
	if err != nil {
		return "", "", err
	}
	return objectKey, accessURL, nil
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
	presignedURL, err := client.PresignedGetObject(ctx, setting.S3Bucket, objectKey, AIImageObjectURLTTL, url.Values{})
	if err != nil {
		return "", err
	}
	return presignedURL.String(), nil
}
