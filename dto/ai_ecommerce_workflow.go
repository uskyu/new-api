package dto

type EcommerceWorkflowTemplateRequest struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	StylePrompt string   `json:"style_prompt"`
	Modules     []string `json:"modules"`
}

type CreateEcommerceWorkflowRequest struct {
	Model             string                           `json:"model" binding:"required"`
	Group             string                           `json:"group,omitempty"`
	Size              string                           `json:"size,omitempty"`
	Template          EcommerceWorkflowTemplateRequest `json:"template"`
	ProductName       string                           `json:"product_name,omitempty"`
	ProductType       string                           `json:"product_type,omitempty"`
	Platform          string                           `json:"platform,omitempty"`
	SellingPoints     string                           `json:"selling_points,omitempty"`
	ExtraRequirements string                           `json:"extra_requirements,omitempty"`
	ReferenceImages   []string                         `json:"reference_images,omitempty"`
}

type RedrawEcommerceSegmentRequest struct {
	Prompt         string `json:"prompt,omitempty"`
	AnnotatedImage string `json:"annotated_image,omitempty"`
}

type EcommerceWorkflowSegmentDTO struct {
	ID                 int64  `json:"id"`
	WorkflowID         string `json:"workflow_id"`
	SegmentKey         string `json:"segment_key"`
	SegmentIndex       int    `json:"segment_index"`
	Label              string `json:"label"`
	Description        string `json:"description"`
	Prompt             string `json:"prompt"`
	CustomRedrawPrompt string `json:"custom_redraw_prompt,omitempty"`
	TaskID             string `json:"task_id,omitempty"`
	Status             string `json:"status"`
	ResultURL          string `json:"result_url,omitempty"`
	ResultKey          string `json:"result_key,omitempty"`
	SliceURL           string `json:"slice_url,omitempty"`
	SliceKey           string `json:"slice_key,omitempty"`
	RedrawCount        int    `json:"redraw_count"`
	CreatedAt          int64  `json:"created_at"`
	UpdatedAt          int64  `json:"updated_at"`
}

type EcommerceWorkflowDTO struct {
	ID                int64                          `json:"id"`
	WorkflowID        string                         `json:"workflow_id"`
	UserID            int                            `json:"user_id"`
	Username          string                         `json:"username,omitempty"`
	TemplateKey       string                         `json:"template_key"`
	TemplateName      string                         `json:"template_name"`
	ProductName       string                         `json:"product_name"`
	ProductType       string                         `json:"product_type"`
	Platform          string                         `json:"platform"`
	SellingPoints     string                         `json:"selling_points"`
	ExtraRequirements string                         `json:"extra_requirements"`
	Model             string                         `json:"model"`
	Group             string                         `json:"group"`
	Size              string                         `json:"size,omitempty"`
	Status            string                         `json:"status"`
	MotherTaskID      string                         `json:"mother_task_id,omitempty"`
	MotherResultURL   string                         `json:"mother_result_url,omitempty"`
	MotherResultKey   string                         `json:"mother_result_key,omitempty"`
	AssembledURL      string                         `json:"assembled_url,omitempty"`
	AssembledKey      string                         `json:"assembled_key,omitempty"`
	ErrorMessage      string                         `json:"error_message,omitempty"`
	Segments          []*EcommerceWorkflowSegmentDTO `json:"segments"`
	ConfirmedAt       int64                          `json:"confirmed_at"`
	FinishedAt        int64                          `json:"finished_at"`
	CreatedAt         int64                          `json:"created_at"`
	UpdatedAt         int64                          `json:"updated_at"`
}
