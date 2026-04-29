package model

import (
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type EcommerceWorkflowStatus string

const (
	EcommerceWorkflowStatusMotherPending      EcommerceWorkflowStatus = "MOTHER_PENDING"
	EcommerceWorkflowStatusMotherProcessing   EcommerceWorkflowStatus = "MOTHER_PROCESSING"
	EcommerceWorkflowStatusWaitingConfirm     EcommerceWorkflowStatus = "WAITING_CONFIRM"
	EcommerceWorkflowStatusMotherFailed       EcommerceWorkflowStatus = "MOTHER_FAILED"
	EcommerceWorkflowStatusSegmentsPending    EcommerceWorkflowStatus = "SEGMENTS_PENDING"
	EcommerceWorkflowStatusSegmentsProcessing EcommerceWorkflowStatus = "SEGMENTS_PROCESSING"
	EcommerceWorkflowStatusSucceeded          EcommerceWorkflowStatus = "SUCCEEDED"
	EcommerceWorkflowStatusFailed             EcommerceWorkflowStatus = "FAILED"
)

type EcommerceWorkflow struct {
	ID                int64                   `json:"id" gorm:"primaryKey;autoIncrement"`
	WorkflowID        string                  `json:"workflow_id" gorm:"type:varchar(191);uniqueIndex"`
	UserID            int                     `json:"user_id" gorm:"index"`
	Username          string                  `json:"username,omitempty" gorm:"-"`
	TemplateKey       string                  `json:"template_key" gorm:"type:varchar(100);index"`
	TemplateName      string                  `json:"template_name" gorm:"type:varchar(191)"`
	ProductName       string                  `json:"product_name" gorm:"type:varchar(191);index"`
	ProductType       string                  `json:"product_type" gorm:"type:varchar(191)"`
	Platform          string                  `json:"platform" gorm:"type:varchar(191)"`
	SellingPoints     string                  `json:"selling_points" gorm:"type:text"`
	ExtraRequirements string                  `json:"extra_requirements" gorm:"type:text"`
	StylePrompt       string                  `json:"style_prompt" gorm:"type:text"`
	Modules           string                  `json:"modules" gorm:"type:text"`
	ReferenceImageKey string                  `json:"reference_image_key" gorm:"type:text"`
	Model             string                  `json:"model" gorm:"type:varchar(191);index"`
	Group             string                  `json:"group" gorm:"type:varchar(64);index"`
	Size              string                  `json:"size" gorm:"type:varchar(32)"`
	Status            EcommerceWorkflowStatus `json:"status" gorm:"type:varchar(32);index"`
	MotherTaskID      string                  `json:"mother_task_id" gorm:"type:varchar(191);index"`
	MotherResultURL   string                  `json:"mother_result_url" gorm:"type:text"`
	MotherResultKey   string                  `json:"mother_result_key" gorm:"type:text"`
	AssembledURL      string                  `json:"assembled_url" gorm:"type:text"`
	AssembledKey      string                  `json:"assembled_key" gorm:"type:text"`
	ErrorMessage      string                  `json:"error_message" gorm:"type:text"`
	ConfirmedAt       int64                   `json:"confirmed_at" gorm:"index"`
	FinishedAt        int64                   `json:"finished_at" gorm:"index"`
	CreatedAt         int64                   `json:"created_at" gorm:"index"`
	UpdatedAt         int64                   `json:"updated_at"`
}

type EcommerceWorkflowSegment struct {
	ID                 int64           `json:"id" gorm:"primaryKey;autoIncrement"`
	WorkflowID         string          `json:"workflow_id" gorm:"type:varchar(191);index"`
	SegmentKey         string          `json:"segment_key" gorm:"type:varchar(64);index"`
	SegmentIndex       int             `json:"segment_index" gorm:"index"`
	Label              string          `json:"label" gorm:"type:varchar(191)"`
	Description        string          `json:"description" gorm:"type:text"`
	Prompt             string          `json:"prompt" gorm:"type:text"`
	CustomRedrawPrompt string          `json:"custom_redraw_prompt" gorm:"type:text"`
	TaskID             string          `json:"task_id" gorm:"type:varchar(191);index"`
	Status             ImageTaskStatus `json:"status" gorm:"type:varchar(20);index"`
	ResultURL          string          `json:"result_url" gorm:"type:text"`
	ResultKey          string          `json:"result_key" gorm:"type:text"`
	SliceKey           string          `json:"slice_key" gorm:"type:text"`
	RedrawCount        int             `json:"redraw_count"`
	CreatedAt          int64           `json:"created_at" gorm:"index"`
	UpdatedAt          int64           `json:"updated_at"`
}

func (w *EcommerceWorkflow) GetReferenceImageKeys() []string {
	return parseStoredKeys(w.ReferenceImageKey)
}

func (w *EcommerceWorkflow) SetReferenceImageKeys(keys []string) error {
	stored, err := stringifyStoredKeys(keys)
	if err != nil {
		return err
	}
	w.ReferenceImageKey = stored
	return nil
}

func parseStoredKeys(stored string) []string {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return nil
	}
	if strings.HasPrefix(stored, "[") {
		var keys []string
		if err := common.Unmarshal([]byte(stored), &keys); err == nil {
			result := make([]string, 0, len(keys))
			for _, key := range keys {
				if trimmed := strings.TrimSpace(key); trimmed != "" {
					result = append(result, trimmed)
				}
			}
			return result
		}
	}
	return []string{stored}
}

func stringifyStoredKeys(keys []string) (string, error) {
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return "", nil
	}
	if len(result) == 1 {
		return result[0], nil
	}
	data, err := common.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (w *EcommerceWorkflow) BeforeCreate(tx any) error {
	now := time.Now().Unix()
	if w.CreatedAt == 0 {
		w.CreatedAt = now
	}
	if w.UpdatedAt == 0 {
		w.UpdatedAt = now
	}
	if w.WorkflowID == "" {
		key, _ := common.GenerateRandomCharsKey(32)
		w.WorkflowID = "ecwf_" + key
	}
	if w.Status == "" {
		w.Status = EcommerceWorkflowStatusMotherPending
	}
	return nil
}

func (w *EcommerceWorkflow) BeforeUpdate(tx any) error {
	w.UpdatedAt = time.Now().Unix()
	return nil
}

func (s *EcommerceWorkflowSegment) BeforeCreate(tx any) error {
	now := time.Now().Unix()
	if s.CreatedAt == 0 {
		s.CreatedAt = now
	}
	if s.UpdatedAt == 0 {
		s.UpdatedAt = now
	}
	if s.Status == "" {
		s.Status = ImageTaskStatusPending
	}
	return nil
}

func (s *EcommerceWorkflowSegment) BeforeUpdate(tx any) error {
	s.UpdatedAt = time.Now().Unix()
	return nil
}

func CreateEcommerceWorkflow(workflow *EcommerceWorkflow) error {
	for retry := 0; retry < 3; retry++ {
		if workflow.WorkflowID == "" {
			key, _ := common.GenerateRandomCharsKey(32)
			workflow.WorkflowID = "ecwf_" + key
		}
		err := DB.Create(workflow).Error
		if err == nil {
			return nil
		}
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "duplicate") {
			workflow.WorkflowID = ""
			continue
		}
		return err
	}
	key, _ := common.GenerateRandomCharsKey(32)
	workflow.WorkflowID = "ecwf_" + key
	return DB.Create(workflow).Error
}

func GetEcommerceWorkflowByWorkflowID(workflowID string) (*EcommerceWorkflow, error) {
	var workflow EcommerceWorkflow
	err := DB.Where("workflow_id = ?", workflowID).First(&workflow).Error
	if err != nil {
		return nil, err
	}
	return &workflow, nil
}

func GetEcommerceWorkflowByWorkflowIDAndUserID(workflowID string, userID int) (*EcommerceWorkflow, error) {
	var workflow EcommerceWorkflow
	err := DB.Where("workflow_id = ? AND user_id = ?", workflowID, userID).First(&workflow).Error
	if err != nil {
		return nil, err
	}
	return &workflow, nil
}

func UpdateEcommerceWorkflowFields(workflowID string, updates map[string]any) error {
	if updates == nil {
		updates = map[string]any{}
	}
	updates["updated_at"] = time.Now().Unix()
	return DB.Model(&EcommerceWorkflow{}).Where("workflow_id = ?", workflowID).Updates(updates).Error
}

func ListUserEcommerceWorkflows(userID, startIdx, limit int) ([]*EcommerceWorkflow, int64, error) {
	query := DB.Model(&EcommerceWorkflow{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var workflows []*EcommerceWorkflow
	err := query.Order("id desc").Offset(startIdx).Limit(limit).Find(&workflows).Error
	return workflows, total, err
}

func ListAllEcommerceWorkflows(startIdx, limit int, userID int, status, templateKey string, startTime, endTime int64) ([]*EcommerceWorkflow, int64, error) {
	query := DB.Model(&EcommerceWorkflow{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if templateKey != "" {
		query = query.Where("template_key = ?", templateKey)
	}
	if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var workflows []*EcommerceWorkflow
	err := query.Order("id desc").Offset(startIdx).Limit(limit).Find(&workflows).Error
	return workflows, total, err
}

func GetRunnableEcommerceWorkflows(limit int) ([]*EcommerceWorkflow, error) {
	statuses := []EcommerceWorkflowStatus{
		EcommerceWorkflowStatusMotherPending,
		EcommerceWorkflowStatusMotherProcessing,
		EcommerceWorkflowStatusSegmentsPending,
		EcommerceWorkflowStatusSegmentsProcessing,
	}
	var workflows []*EcommerceWorkflow
	err := DB.Where("status IN ?", statuses).Order("id asc").Limit(limit).Find(&workflows).Error
	return workflows, err
}

func CreateEcommerceWorkflowSegment(segment *EcommerceWorkflowSegment) error {
	return DB.Create(segment).Error
}

func GetEcommerceWorkflowSegments(workflowID string) ([]*EcommerceWorkflowSegment, error) {
	var segments []*EcommerceWorkflowSegment
	err := DB.Where("workflow_id = ?", workflowID).Order("segment_index asc, id asc").Find(&segments).Error
	if err != nil {
		return nil, err
	}
	return dedupeEcommerceWorkflowSegments(segments), nil
}

func GetEcommerceWorkflowSegment(workflowID, segmentKey string) (*EcommerceWorkflowSegment, error) {
	var segment EcommerceWorkflowSegment
	err := DB.Where("workflow_id = ? AND segment_key = ?", workflowID, segmentKey).Order("id desc").First(&segment).Error
	if err != nil {
		return nil, err
	}
	return &segment, nil
}

func ecommerceWorkflowSegmentStatusRank(status ImageTaskStatus) int {
	switch status {
	case ImageTaskStatusSucceeded:
		return 4
	case ImageTaskStatusProcessing:
		return 3
	case ImageTaskStatusPending:
		return 2
	case ImageTaskStatusFailed:
		return 1
	default:
		return 0
	}
}

func dedupeEcommerceWorkflowSegments(segments []*EcommerceWorkflowSegment) []*EcommerceWorkflowSegment {
	if len(segments) <= 1 {
		return segments
	}
	chosen := make(map[string]*EcommerceWorkflowSegment)
	for _, segment := range segments {
		if segment == nil {
			continue
		}
		key := segment.SegmentKey
		current := chosen[key]
		if current == nil {
			chosen[key] = segment
			continue
		}
		currentRank := ecommerceWorkflowSegmentStatusRank(current.Status)
		nextRank := ecommerceWorkflowSegmentStatusRank(segment.Status)
		if nextRank > currentRank || (nextRank == currentRank && segment.ID > current.ID) {
			chosen[key] = segment
		}
	}
	result := make([]*EcommerceWorkflowSegment, 0, len(chosen))
	for _, segment := range chosen {
		result = append(result, segment)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SegmentIndex == result[j].SegmentIndex {
			return result[i].ID < result[j].ID
		}
		return result[i].SegmentIndex < result[j].SegmentIndex
	})
	return result
}

func UpdateEcommerceWorkflowSegmentFields(workflowID, segmentKey string, updates map[string]any) error {
	if updates == nil {
		updates = map[string]any{}
	}
	updates["updated_at"] = time.Now().Unix()
	return DB.Model(&EcommerceWorkflowSegment{}).Where("workflow_id = ? AND segment_key = ?", workflowID, segmentKey).Updates(updates).Error
}

func CountEcommerceWorkflowsByTime(startTime, endTime int64) (int64, error) {
	query := DB.Model(&EcommerceWorkflow{})
	if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}
	var total int64
	err := query.Count(&total).Error
	return total, err
}
