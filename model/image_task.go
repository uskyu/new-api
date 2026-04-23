package model

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type ImageTaskStatus string

const (
	ImageTaskStatusPending    ImageTaskStatus = "PENDING"
	ImageTaskStatusProcessing ImageTaskStatus = "PROCESSING"
	ImageTaskStatusSucceeded  ImageTaskStatus = "SUCCEEDED"
	ImageTaskStatusFailed     ImageTaskStatus = "FAILED"
)

type ImageTask struct {
	ID                  int64           `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID              string          `json:"task_id" gorm:"type:varchar(191);uniqueIndex"`
	UserID              int             `json:"user_id" gorm:"index"`
	Username            string          `json:"username,omitempty" gorm:"-"`
	Group               string          `json:"group" gorm:"type:varchar(64);index"`
	Model               string          `json:"model" gorm:"type:varchar(191);index"`
	Size                string          `json:"size" gorm:"type:varchar(32)"`
	Prompt              string          `json:"prompt" gorm:"type:text"`
	Status              ImageTaskStatus `json:"status" gorm:"type:varchar(20);index"`
	ChannelID           int             `json:"channel_id" gorm:"index"`
	ResultURL           string          `json:"result_url" gorm:"type:text"`
	ResultKey           string          `json:"result_key" gorm:"type:text"`
	ReferenceImageKey   string          `json:"reference_image_key" gorm:"type:text"`
	ErrorMessage        string          `json:"error_message" gorm:"type:text"`
	StartedAt           int64           `json:"started_at" gorm:"index"`
	FinishedAt          int64           `json:"finished_at" gorm:"index"`
	CreatedAt           int64           `json:"created_at" gorm:"index"`
	UpdatedAt           int64           `json:"updated_at"`
}

func (t *ImageTask) BeforeCreate(tx any) error {
	now := time.Now().Unix()
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	if t.UpdatedAt == 0 {
		t.UpdatedAt = now
	}
	if t.TaskID == "" {
		key, _ := common.GenerateRandomCharsKey(32)
		t.TaskID = "imgtask_" + key
	}
	if t.Status == "" {
		t.Status = ImageTaskStatusPending
	}
	return nil
}

func (t *ImageTask) BeforeUpdate(tx any) error {
	t.UpdatedAt = time.Now().Unix()
	return nil
}

func CreateImageTask(task *ImageTask) error {
	for retry := 0; retry < 3; retry++ {
		if task.TaskID == "" {
			key, _ := common.GenerateRandomCharsKey(32)
			task.TaskID = "imgtask_" + key
		}
		err := DB.Create(task).Error
		if err == nil {
			return nil
		}
		if common.UsingSQLite {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				task.TaskID = ""
				continue
			}
		} else if common.UsingMySQL {
			if strings.Contains(err.Error(), "Duplicate entry") {
				task.TaskID = ""
				continue
			}
		} else if common.UsingPostgreSQL {
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				task.TaskID = ""
				continue
			}
		}
		return err
	}
	key, _ := common.GenerateRandomCharsKey(32)
	task.TaskID = "imgtask_" + key
	return DB.Create(task).Error
}

func GetImageTaskByTaskID(taskID string) (*ImageTask, error) {
	var task ImageTask
	err := DB.Where("task_id = ?", taskID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func GetImageTaskByTaskIDAndUserID(taskID string, userID int) (*ImageTask, error) {
	var task ImageTask
	err := DB.Where("task_id = ? AND user_id = ?", taskID, userID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func ListUserImageTasks(userID, startIdx, limit int, status string) ([]*ImageTask, int64, error) {
	query := DB.Model(&ImageTask{}).Where("user_id = ?", userID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tasks []*ImageTask
	err := query.Order("id desc").Offset(startIdx).Limit(limit).Find(&tasks).Error
	return tasks, total, err
}

func ListAllImageTasks(startIdx, limit int, userID int, modelName, status string) ([]*ImageTask, int64, error) {
	query := DB.Model(&ImageTask{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tasks []*ImageTask
	err := query.Order("id desc").Offset(startIdx).Limit(limit).Find(&tasks).Error
	return tasks, total, err
}

func CountImageTasksByStatus(status ImageTaskStatus) (int64, error) {
	var total int64
	err := DB.Model(&ImageTask{}).Where("status = ?", status).Count(&total).Error
	return total, err
}

func CountPendingImageTasks() (int64, error) {
	var total int64
	err := DB.Model(&ImageTask{}).Where("status IN ?", []ImageTaskStatus{ImageTaskStatusPending, ImageTaskStatusProcessing}).Count(&total).Error
	return total, err
}

func GetPendingImageTasks(limit int) ([]*ImageTask, error) {
	var tasks []*ImageTask
	err := DB.Where("status = ?", ImageTaskStatusPending).Order("id asc").Limit(limit).Find(&tasks).Error
	return tasks, err
}

func UpdateImageTaskStatus(taskID string, fromStatus, toStatus ImageTaskStatus, updates map[string]any) (bool, error) {
	if updates == nil {
		updates = map[string]any{}
	}
	updates["status"] = toStatus
	updates["updated_at"] = time.Now().Unix()
	result := DB.Model(&ImageTask{}).Where("task_id = ? AND status = ?", taskID, fromStatus).Updates(updates)
	return result.RowsAffected > 0, result.Error
}

func UpdateImageTaskFields(taskID string, updates map[string]any) error {
	if updates == nil {
		updates = map[string]any{}
	}
	updates["updated_at"] = time.Now().Unix()
	return DB.Model(&ImageTask{}).Where("task_id = ?", taskID).Updates(updates).Error
}

func CleanStaleFailedImageTasks(beforeTimestamp int64) (int64, error) {
	result := DB.Where("status = ? AND created_at < ?", ImageTaskStatusFailed, beforeTimestamp).Delete(&ImageTask{})
	return result.RowsAffected, result.Error
}

func DeleteImageTaskByTaskIDAndUserID(taskID string, userID int) (*ImageTask, error) {
	var task ImageTask
	if err := DB.Where("task_id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		return nil, err
	}
	if err := DB.Delete(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func GetStaleImageTasks(beforeTimestamp int64, limit int) ([]*ImageTask, error) {
	var tasks []*ImageTask
	err := DB.Where("status IN ? AND created_at < ?", []ImageTaskStatus{ImageTaskStatusSucceeded, ImageTaskStatusFailed}, beforeTimestamp).
		Order("id asc").Limit(limit).Find(&tasks).Error
	return tasks, err
}

func BatchDeleteImageTasksByIDs(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB.Where("id IN ?", ids).Delete(&ImageTask{}).Error
}
