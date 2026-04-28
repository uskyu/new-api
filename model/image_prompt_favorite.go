package model

import (
	"strings"
	"time"
)

type ImagePromptFavorite struct {
	ID          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      int    `json:"user_id" gorm:"index"`
	Title       string `json:"title" gorm:"type:varchar(191)"`
	Prompt      string `json:"prompt" gorm:"type:text"`
	Model       string `json:"model" gorm:"type:varchar(191)"`
	Group       string `json:"group" gorm:"type:varchar(64)"`
	Size        string `json:"size" gorm:"type:varchar(32)"`
	Resolution  string `json:"resolution" gorm:"type:varchar(32)"`
	AspectRatio string `json:"aspect_ratio" gorm:"type:varchar(32)"`
	Tags        string `json:"tags" gorm:"type:text"`
	LastUsedAt  int64  `json:"last_used_at" gorm:"index"`
	CreatedAt   int64  `json:"created_at" gorm:"index"`
	UpdatedAt   int64  `json:"updated_at"`
}

func (ImagePromptFavorite) TableName() string {
	return "ai_image_prompt_favorites"
}

func (f *ImagePromptFavorite) BeforeCreate(tx any) error {
	now := time.Now().Unix()
	if f.CreatedAt == 0 {
		f.CreatedAt = now
	}
	if f.UpdatedAt == 0 {
		f.UpdatedAt = now
	}
	return nil
}

func (f *ImagePromptFavorite) BeforeUpdate(tx any) error {
	f.UpdatedAt = time.Now().Unix()
	return nil
}

func ListImagePromptFavorites(userID, offset, limit int, keyword string) ([]*ImagePromptFavorite, int64, error) {
	query := DB.Model(&ImagePromptFavorite{}).Where("user_id = ?", userID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR prompt LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var favorites []*ImagePromptFavorite
	err := query.Order("updated_at desc, id desc").Offset(offset).Limit(limit).Find(&favorites).Error
	return favorites, total, err
}

func CreateImagePromptFavorite(favorite *ImagePromptFavorite) error {
	return DB.Create(favorite).Error
}

func GetImagePromptFavoriteByIDAndUserID(id int64, userID int) (*ImagePromptFavorite, error) {
	var favorite ImagePromptFavorite
	err := DB.Where("id = ? AND user_id = ?", id, userID).First(&favorite).Error
	if err != nil {
		return nil, err
	}
	return &favorite, nil
}

func UpdateImagePromptFavorite(id int64, userID int, updates map[string]any) (*ImagePromptFavorite, error) {
	if updates == nil {
		updates = map[string]any{}
	}
	updates["updated_at"] = time.Now().Unix()
	if err := DB.Model(&ImagePromptFavorite{}).Where("id = ? AND user_id = ?", id, userID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetImagePromptFavoriteByIDAndUserID(id, userID)
}

func MarkImagePromptFavoriteUsed(id int64, userID int) (*ImagePromptFavorite, error) {
	return UpdateImagePromptFavorite(id, userID, map[string]any{"last_used_at": time.Now().Unix()})
}

func DeleteImagePromptFavorite(id int64, userID int) error {
	return DB.Where("id = ? AND user_id = ?", id, userID).Delete(&ImagePromptFavorite{}).Error
}
