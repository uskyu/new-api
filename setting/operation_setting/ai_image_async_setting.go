package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type AIImageAsyncSetting struct {
	Enabled           bool   `json:"enabled"`
	WorkerConcurrency int    `json:"worker_concurrency"`
	RetryCount        int    `json:"retry_count"`
	PollIntervalSec   int    `json:"poll_interval_sec"`
	QueueLimit        int    `json:"queue_limit"`
	S3Enabled         bool   `json:"s3_enabled"`
	S3Endpoint        string `json:"s3_endpoint"`
	S3Region          string `json:"s3_region"`
	S3Bucket          string `json:"s3_bucket"`
	S3AccessKey       string `json:"s3_access_key"`
	S3SecretKey       string `json:"s3_secret_key"`
	S3PublicBaseURL   string `json:"s3_public_base_url"`
	S3PathPrefix      string `json:"s3_path_prefix"`
	S3UseSSL          bool   `json:"s3_use_ssl"`
}

var aiImageAsyncSetting = AIImageAsyncSetting{
	Enabled:           false,
	WorkerConcurrency: 1,
	RetryCount:        1,
	PollIntervalSec:   3,
	QueueLimit:        50,
	S3Enabled:         false,
	S3Region:          "auto",
	S3PathPrefix:      "ai-image",
	S3UseSSL:          true,
}

func init() {
	config.GlobalConfig.Register("ai_image_async_setting", &aiImageAsyncSetting)
}

func GetAIImageAsyncSetting() *AIImageAsyncSetting {
	return &aiImageAsyncSetting
}
