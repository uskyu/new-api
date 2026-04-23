package dto

type CreateImageTaskRequest struct {
	Model  string `json:"model" binding:"required"`
	Prompt string `json:"prompt" binding:"required"`
	Group  string `json:"group,omitempty"`
	Size   string `json:"size,omitempty"`
	N      *uint  `json:"n,omitempty"`
}

type ImageTaskDTO struct {
	ID           int64  `json:"id"`
	TaskID       string `json:"task_id"`
	UserID       int    `json:"user_id"`
	Username     string `json:"username,omitempty"`
	Group        string `json:"group"`
	Model        string `json:"model"`
	Size         string `json:"size,omitempty"`
	Prompt       string `json:"prompt"`
	Status       string `json:"status"`
	ChannelID    int    `json:"channel_id"`
	ResultURL    string `json:"result_url,omitempty"`
	ResultKey    string `json:"result_key,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	StartedAt    int64  `json:"started_at"`
	FinishedAt   int64  `json:"finished_at"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

type ImageTaskStatsResponse struct {
	Pending           int64 `json:"pending"`
	Processing        int64 `json:"processing"`
	Succeeded         int64 `json:"succeeded"`
	Failed            int64 `json:"failed"`
	WorkerConcurrency int   `json:"worker_concurrency"`
	QueueLimit        int   `json:"queue_limit"`
}
