package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	maxImagePromptFavoriteTitleLength  = 120
	maxImagePromptFavoritePromptLength = 8000
	maxImagePromptFavoriteListLimit    = 100
)

type imagePromptFavoriteRequest struct {
	Title       string `json:"title"`
	Prompt      string `json:"prompt"`
	Model       string `json:"model"`
	Group       string `json:"group"`
	Size        string `json:"size"`
	Resolution  string `json:"resolution"`
	AspectRatio string `json:"aspect_ratio"`
	Tags        string `json:"tags"`
}

func getImagePromptFavoriteID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid id")
		return 0, false
	}
	return id, true
}

func normalizeImagePromptFavoriteTitle(title, prompt string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = strings.TrimSpace(prompt)
	}
	if len([]rune(title)) > maxImagePromptFavoriteTitleLength {
		runes := []rune(title)
		title = string(runes[:maxImagePromptFavoriteTitleLength])
	}
	return title
}

func normalizeImagePromptFavoriteRequest(req imagePromptFavoriteRequest) imagePromptFavoriteRequest {
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.Title = normalizeImagePromptFavoriteTitle(req.Title, req.Prompt)
	req.Model = strings.TrimSpace(req.Model)
	req.Group = strings.TrimSpace(req.Group)
	req.Size = strings.TrimSpace(req.Size)
	req.Resolution = strings.TrimSpace(req.Resolution)
	req.AspectRatio = strings.TrimSpace(req.AspectRatio)
	req.Tags = strings.TrimSpace(req.Tags)
	return req
}

func ListImagePromptFavorites(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	limit := pageInfo.GetPageSize()
	if limit <= 0 || limit > maxImagePromptFavoriteListLimit {
		limit = maxImagePromptFavoriteListLimit
	}
	items, total, err := model.ListImagePromptFavorites(c.GetInt("id"), pageInfo.GetStartIdx(), limit, c.Query("q"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetItems(items)
	pageInfo.SetTotal(int(total))
	common.ApiSuccess(c, pageInfo)
}

func CreateImagePromptFavorite(c *gin.Context) {
	var req imagePromptFavoriteRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	req = normalizeImagePromptFavoriteRequest(req)
	if req.Prompt == "" {
		common.ApiErrorMsg(c, "prompt is required")
		return
	}
	if len([]rune(req.Prompt)) > maxImagePromptFavoritePromptLength {
		common.ApiErrorMsg(c, "prompt is too long")
		return
	}
	favorite := &model.ImagePromptFavorite{
		UserID:      c.GetInt("id"),
		Title:       req.Title,
		Prompt:      req.Prompt,
		Model:       req.Model,
		Group:       req.Group,
		Size:        req.Size,
		Resolution:  req.Resolution,
		AspectRatio: req.AspectRatio,
		Tags:        req.Tags,
	}
	if err := model.CreateImagePromptFavorite(favorite); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, favorite)
}

func UpdateImagePromptFavorite(c *gin.Context) {
	id, ok := getImagePromptFavoriteID(c)
	if !ok {
		return
	}
	var req imagePromptFavoriteRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	req = normalizeImagePromptFavoriteRequest(req)
	if req.Prompt == "" {
		common.ApiErrorMsg(c, "prompt is required")
		return
	}
	if len([]rune(req.Prompt)) > maxImagePromptFavoritePromptLength {
		common.ApiErrorMsg(c, "prompt is too long")
		return
	}
	favorite, err := model.UpdateImagePromptFavorite(id, c.GetInt("id"), map[string]any{
		"title":        req.Title,
		"prompt":       req.Prompt,
		"model":        req.Model,
		"group":        req.Group,
		"size":         req.Size,
		"resolution":   req.Resolution,
		"aspect_ratio": req.AspectRatio,
		"tags":         req.Tags,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, favorite)
}

func UseImagePromptFavorite(c *gin.Context) {
	id, ok := getImagePromptFavoriteID(c)
	if !ok {
		return
	}
	favorite, err := model.MarkImagePromptFavoriteUsed(id, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, favorite)
}

func DeleteImagePromptFavorite(c *gin.Context) {
	id, ok := getImagePromptFavoriteID(c)
	if !ok {
		return
	}
	if err := model.DeleteImagePromptFavorite(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
