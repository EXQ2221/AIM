package router

import (
	"example.com/aim/gateway/internal/handler"
	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/observability"
	"example.com/aim/gateway/internal/upload"
	"github.com/gin-gonic/gin"
)

func New() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()
	engine.Use(middleware.Recovery())
	engine.Use(observability.MetricsMiddleware())

	engine.GET("/healthz", func(ctx *gin.Context) {
		ctx.String(200, "ok")
	})
	engine.GET("/metrics", observability.MetricsHandler())
	engine.Static(upload.PublicPrefix(), upload.Dir())
	engine.GET("/ws/chat", handler.ChatWebSocket)

	authGroup := engine.Group("/api/v1/auth")
	authGroup.POST("/register", handler.Register)
	authGroup.POST("/login", handler.Login)
	authGroup.POST("/refresh", handler.Refresh)
	authGroup.Use(middleware.Auth())
	authGroup.POST("/logout", handler.Logout)
	authGroup.POST("/logout-all", handler.LogoutAll)
	authGroup.GET("/sessions", handler.ListSessions)
	authGroup.POST("/sessions/revoke", handler.RevokeSession)

	userGroup := engine.Group("/api/v1/users")
	userGroup.Use(middleware.Auth())
	userGroup.GET("/me", handler.Me)
	userGroup.POST("/me/avatar", handler.UploadAvatar)

	uploadGroup := engine.Group("/api/v1/uploads")
	uploadGroup.Use(middleware.Auth())
	uploadGroup.POST("/images", handler.UploadImage)
	uploadGroup.POST("/files", handler.UploadFile)
	uploadGroup.POST("/voices", handler.UploadVoice)

	friendGroup := engine.Group("/api/v1/friends")
	friendGroup.Use(middleware.Auth())
	friendGroup.GET("", handler.ListFriends)
	friendGroup.POST("", handler.AddFriend)
	friendGroup.GET("/requests", handler.ListFriendRequests)
	friendGroup.POST("/requests/:requestId/respond", handler.RespondFriendRequest)
	friendGroup.PATCH("/:friendUserId", handler.UpdateFriend)
	friendGroup.DELETE("/:friendUserId", handler.DeleteFriend)
	friendGroup.GET("/groups", handler.ListFriendGroups)
	friendGroup.POST("/groups", handler.CreateFriendGroup)

	conversationGroup := engine.Group("/api/v1/conversations")
	conversationGroup.Use(middleware.Auth())
	conversationGroup.POST("/group", handler.CreateGroup)
	conversationGroup.GET("", handler.ListConversations)
	conversationGroup.GET("/single", handler.FindSingleConversation)
	conversationGroup.GET("/:conversationId/group", handler.GetGroupInfo)
	conversationGroup.POST("/:conversationId/members", handler.JoinGroup)
	conversationGroup.POST("/:conversationId/members/invite", handler.InviteMember)
	conversationGroup.DELETE("/:conversationId/members/me", handler.LeaveGroup)
	conversationGroup.DELETE("/:conversationId/members/:targetUserId", handler.RemoveMember)
	conversationGroup.POST("/:conversationId/members/:targetUserId/mute", handler.MuteMember)
	conversationGroup.DELETE("/:conversationId/members/:targetUserId/mute", handler.UnmuteMember)
	conversationGroup.GET("/:conversationId/members", handler.ListMembers)
	conversationGroup.POST("/:conversationId/owner/transfer", handler.TransferOwner)
	conversationGroup.POST("/:conversationId/admins", handler.SetAdmin)
	conversationGroup.DELETE("/:conversationId/admins/:targetUserId", handler.RemoveAdmin)
	conversationGroup.POST("/:conversationId/mute-all", handler.EnableGroupMuteAll)
	conversationGroup.DELETE("/:conversationId/mute-all", handler.DisableGroupMuteAll)
	conversationGroup.PUT("/:conversationId/announcement", handler.UpdateGroupAnnouncement)
	conversationGroup.POST("/:conversationId/read", handler.MarkConversationRead)
	conversationGroup.POST("/:conversationId/messages/:messageId/recall", handler.RecallMessage)
	conversationGroup.GET("/:conversationId/bots", handler.ListConversationBots)
	conversationGroup.POST("/:conversationId/bots", handler.AddConversationBot)
	conversationGroup.DELETE("/:conversationId/bots/:botId", handler.RemoveConversationBot)
	conversationGroup.POST("/:conversationId/knowledge-bases", handler.BindConversationKnowledgeBase)
	conversationGroup.GET("/:conversationId/knowledge-bases", handler.ListConversationKnowledgeBases)
	conversationGroup.DELETE("/:conversationId/knowledge-bases/:knowledgeBaseId", handler.UnbindConversationKnowledgeBase)
	conversationGroup.GET("/:conversationId/ai-call-logs", handler.ListAICallLogs)
	conversationGroup.GET("/:conversationId/messages", handler.ListMessages)
	conversationGroup.GET("/history/search", handler.SearchHistoryMessages)

	knowledgeBaseGroup := engine.Group("/api/v1/knowledge-bases")
	knowledgeBaseGroup.Use(middleware.Auth())
	knowledgeBaseGroup.GET("", handler.ListKnowledgeBases)
	knowledgeBaseGroup.POST("", handler.CreateKnowledgeBase)
	knowledgeBaseGroup.POST("/:knowledgeBaseId/documents/text", handler.AddKnowledgeDocumentText)
	knowledgeBaseGroup.POST("/:knowledgeBaseId/documents/file", handler.AddKnowledgeDocumentFile)
	knowledgeBaseGroup.GET("/:knowledgeBaseId/documents", handler.ListKnowledgeDocuments)
	knowledgeBaseGroup.DELETE("/:knowledgeBaseId/documents/:documentId", handler.DeleteKnowledgeDocument)
	knowledgeBaseGroup.POST("/:knowledgeBaseId/search", handler.SearchKnowledgeBase)

	botGroup := engine.Group("/api/v1/bots")
	botGroup.Use(middleware.Auth())
	botGroup.GET("", handler.ListBots)
	botGroup.POST("", handler.CreateCustomBot)

	notificationGroup := engine.Group("/api/v1/notifications")
	notificationGroup.Use(middleware.Auth())
	notificationGroup.GET("", handler.ListNotifications)
	notificationGroup.POST("/read-all", handler.MarkAllNotificationsRead)
	notificationGroup.POST("/:notificationId/read", handler.MarkNotificationRead)

	return engine
}
