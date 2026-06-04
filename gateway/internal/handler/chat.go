package handler

import (
	"os"
	"strconv"
	"strings"
	"time"

	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model/dto"
	"example.com/aim/gateway/internal/rpc"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
	"github.com/gin-gonic/gin"
)

const (
	maxKnowledgeDocumentUploadBytes int64 = 20 << 20
)

var (
	knowledgeImportParseTimeout      = getenvDuration("KNOWLEDGE_IMPORT_PARSE_TIMEOUT", 30*time.Minute)
	knowledgeImportRAGAddTimeout     = getenvDuration("KNOWLEDGE_IMPORT_RAG_ADD_TIMEOUT", 30*time.Minute)
	knowledgeImportWatchTimeout      = getenvDuration("KNOWLEDGE_IMPORT_WATCH_TIMEOUT", 30*time.Minute)
	knowledgeImportPollInterval      = getenvDuration("KNOWLEDGE_IMPORT_POLL_INTERVAL", 2*time.Second)
	knowledgeImportPollRPCDeadline   = getenvDuration("KNOWLEDGE_IMPORT_POLL_RPC_TIMEOUT", 5*time.Second)
	knowledgeImportNotifyRPCDeadline = getenvDuration("KNOWLEDGE_IMPORT_NOTIFY_RPC_TIMEOUT", 5*time.Second)
)

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if d, err := time.ParseDuration(value); err == nil && d > 0 {
		return d
	}
	return fallback
}

func ListConversations(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListConversations(ctx.Request.Context(), &chatpb.ListConversationsRequest{
		UserId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	conversations := make([]dto.ConversationInfo, 0, len(resp.Conversations))
	for _, conversation := range resp.Conversations {
		conversations = append(conversations, toConversationModel(conversation))
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    conversations,
	})
}
