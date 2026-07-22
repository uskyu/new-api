package router

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// registerExtensionApiRoutes keeps locally maintained routes isolated from the
// upstream API router so future upstream updates only touch this call site.
func registerExtensionApiRoutes(apiRouter *gin.RouterGroup) {
	agentRoute := apiRouter.Group("/agent")
	{
		agentRoute.GET("/status", middleware.PermissionAuth(common.PermissionAgentDownlineTransfer), controller.GetAgentBootstrapStatus)
		agentRoute.GET("/overview", middleware.AdminAuth(), controller.GetAgentAdminOverview)
		agentRoute.GET("/daily-metrics", middleware.AdminAuth(), controller.GetAgentDailyMetrics)
		agentRoute.POST("/init", middleware.RootAuth(), controller.InitializeAgentModule)

		agentRoute.GET("/self", middleware.UserAuth(), controller.GetAgentSelfSummary)
		agentRoute.GET("/self/daily-metrics", middleware.UserAuth(), controller.GetAgentSelfDailyMetrics)
		agentRoute.GET("/self/downlines", middleware.UserAuth(), controller.GetAgentSelfDownlines)
		agentRoute.GET("/self/rebates", middleware.UserAuth(), controller.GetAgentSelfRebateRecords)
		agentRoute.GET("/self/adjustments", middleware.UserAuth(), controller.GetAgentSelfAdjustments)
		agentRoute.GET("/self/promo-links", middleware.UserAuth(), controller.GetAgentSelfPromoLinks)
		agentRoute.GET("/self/promo-link-stats", middleware.UserAuth(), controller.GetAgentSelfPromoLinkStats)
		agentRoute.POST("/self/promo-link", middleware.UserAuth(), controller.CreateAgentSelfPromoLink)
		agentRoute.DELETE("/self/promo-link/:id", middleware.UserAuth(), controller.DeleteAgentSelfPromoLink)
		agentRoute.POST("/self/upgrade-request", middleware.UserAuth(), controller.CreateAgentUpgradeRequest)
		agentRoute.POST("/self/withdraw-request", middleware.UserAuth(), controller.CreateAgentWithdrawRequest)
		agentRoute.GET("/self/withdraw-requests", middleware.UserAuth(), controller.GetAgentSelfWithdrawRequests)

		agentRoute.GET("/groups", middleware.AdminAuth(), controller.GetAgentRebateGroups)
		agentRoute.POST("/group", middleware.AdminAuth(), controller.UpsertAgentRebateGroup)
		agentRoute.DELETE("/group/:id", middleware.AdminAuth(), controller.DeleteAgentRebateGroup)
		agentRoute.POST("/profile", middleware.AdminAuth(), controller.UpsertAgentProfile)
		agentRoute.GET("/promo-links", middleware.AdminAuth(), controller.GetAgentPromoLinks)
		agentRoute.GET("/promo-link-stats", middleware.AdminAuth(), controller.GetAgentPromoLinkStats)
		agentRoute.POST("/promo-link", middleware.AdminAuth(), controller.UpsertAgentPromoLink)
		agentRoute.DELETE("/promo-link/:id", middleware.AdminAuth(), controller.DeleteAgentPromoLink)
		agentRoute.GET("/withdraw-requests", middleware.AdminAuth(), controller.GetAgentWithdrawRequests)
		agentRoute.GET("/withdraw-requests/export", middleware.AdminAuth(), controller.ExportAgentWithdrawRequests)
		agentRoute.POST("/withdraw-requests/import", middleware.AdminAuth(), controller.ImportAgentWithdrawResults)
		agentRoute.GET("/upgrade-requests", middleware.AdminAuth(), controller.GetAgentUpgradeRequests)
		agentRoute.POST("/upgrade-request/:id/review", middleware.AdminAuth(), controller.ReviewAgentUpgradeRequest)

		agentRoute.GET("/profiles", middleware.PermissionAuth(common.PermissionAgentDownlineTransfer), controller.GetAgentProfiles)
		agentRoute.GET("/downlines", middleware.AdminAuth(), controller.GetAgentDownlineUsers)
		agentRoute.POST("/downline/assign", middleware.PermissionAuth(common.PermissionAgentDownlineAssign), controller.AssignAgentDownlineUser)
		agentRoute.POST("/downline/transfer", middleware.PermissionAuth(common.PermissionAgentDownlineTransfer), controller.TransferAgentDownlineUser)
		agentRoute.POST("/downline/change", middleware.PermissionAuth(common.PermissionAgentDownlineTransfer), controller.ChangeAgentDownlineUser)
		agentRoute.POST("/adjust", middleware.PermissionAuth(common.PermissionAgentBalanceAdjust), controller.AdjustAgentBalance)
		agentRoute.GET("/adjustments", middleware.PermissionAuth(common.PermissionAgentBalanceAdjust), controller.GetAgentAdjustments)
	}

	supportRoute := apiRouter.Group("/support")
	supportRoute.Use(middleware.PermissionAuth(common.PermissionUserQuotaDecrease))
	{
		supportRoute.GET("/users/search", controller.SearchSupportUsers)
		supportRoute.GET("/users/:id", controller.GetSupportUser)
		supportRoute.POST("/users/:id/quota/decrease", controller.SupportDecreaseUserQuota)
	}

	apiRouter.GET("/user/redemption/self", middleware.UserAuth(), controller.GetUserRedemptionBills)
	apiRouter.GET("/user/redemption", middleware.AdminAuth(), controller.GetAllRedemptionBills)
	apiRouter.GET("/user/:id/redemptions", middleware.AdminAuth(), controller.GetUserRedemptionBillsByAdmin)
	apiRouter.GET("/user/:id/topups", middleware.AdminAuth(), controller.GetUserTopUpsByAdmin)

	aiConsoleFileRoute := apiRouter.Group("/ai-console/files")
	aiConsoleFileRoute.Use(middleware.UserAuth(), middleware.AIConsoleUploadCleanup())
	{
		aiConsoleFileRoute.POST("/parse", controller.ParseAIConsoleFile)
	}
}
