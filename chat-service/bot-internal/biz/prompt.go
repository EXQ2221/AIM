package bot

import (
	"fmt"
	"strings"

	"example.com/aim/chat-service/internal/dal/model"
)

const DefaultSystemPrompt = "\u4f60\u662f AIM \u7fa4\u804a\u4e2d\u7684 AI \u52a9\u624b\u3002\u8bf7\u57fa\u4e8e\u7fa4\u804a\u4e0a\u4e0b\u6587\u56de\u7b54\u7528\u6237\u95ee\u9898\u3002\u8981\u6c42\uff1a1. \u56de\u7b54\u7b80\u6d01\u3001\u51c6\u786e\u30022. \u5982\u679c\u4e0a\u4e0b\u6587\u4e0d\u8db3\uff0c\u8bf7\u76f4\u63a5\u8bf4\u660e\u4e0d\u786e\u5b9a\u30023. \u4e0d\u8981\u7f16\u9020\u7fa4\u804a\u4e2d\u6ca1\u6709\u7684\u4fe1\u606f\u3002"

func BuildPrompt(recentMessages []model.Message, currentContent string, limit int, userDisplayNames map[uint64]string, currentUserID uint64) string {
	return BuildPromptWithRAG(recentMessages, currentContent, limit, userDisplayNames, currentUserID, model.BotScopeConversationOnly, nil)
}

func BuildPromptWithRAG(
	recentMessages []model.Message,
	currentContent string,
	limit int,
	userDisplayNames map[uint64]string,
	currentUserID uint64,
	scope model.BotPermissionScope,
	ragChunks []RAGChunk,
) string {
	if limit <= 0 {
		limit = 20
	}
	useConversation := scope != model.BotScopeKnowledgeBaseOnly
	if useConversation && len(recentMessages) > limit {
		recentMessages = recentMessages[len(recentMessages)-limit:]
	}

	var builder strings.Builder
	if useConversation {
		builder.WriteString("\u3010\u7fa4\u804a\u4e0a\u4e0b\u6587\u3011\n")
		if len(recentMessages) == 0 {
			builder.WriteString("\uff08\u6682\u65e0\u6700\u8fd1\u6d88\u606f\uff09\n")
		} else {
			for _, msg := range recentMessages {
				builder.WriteString(formatMessageLine(msg, userDisplayNames))
				builder.WriteByte('\n')
			}
		}
		builder.WriteByte('\n')
	}

	useKnowledgeBase := scope == model.BotScopeKnowledgeBaseOnly || scope == model.BotScopeConversationAndKB
	if useKnowledgeBase {
		builder.WriteString("\u3010\u77e5\u8bc6\u5e93\u8d44\u6599\u3011\n")
		if len(ragChunks) == 0 {
			builder.WriteString("\uff08\u672a\u68c0\u7d22\u5230\u76f8\u5173\u8d44\u6599\uff0c\u6216\u77e5\u8bc6\u5e93\u68c0\u7d22\u6682\u4e0d\u53ef\u7528\uff09\n")
		} else {
			for index, chunk := range ragChunks {
				builder.WriteString(fmt.Sprintf("[%d] %s\n", index+1, strings.TrimSpace(chunk.Content)))
			}
		}
		builder.WriteByte('\n')
	}

	builder.WriteString("\u3010\u5f53\u524d\u63d0\u95ee\u7528\u6237\u3011")
	builder.WriteString(userDisplayName(currentUserID, userDisplayNames))
	builder.WriteByte('\n')
	builder.WriteString("\u3010\u7528\u6237\u95ee\u9898\u3011")
	builder.WriteString(ExtractQuestion(currentContent))
	builder.WriteByte('\n')
	builder.WriteString("\u3010\u56de\u7b54\u8981\u6c42\u3011\n")
	builder.WriteString("1. \u4f18\u5148\u4f9d\u636e\u77e5\u8bc6\u5e93\u8d44\u6599\u56de\u7b54\uff1b\u8d44\u6599\u4e0d\u8db3\u8bf7\u660e\u786e\u8bf4\u660e\u4e0d\u786e\u5b9a\u3002\n")
	builder.WriteString("2. \u4e0d\u8981\u7f16\u9020\u7fa4\u804a\u6216\u77e5\u8bc6\u5e93\u4e2d\u4e0d\u5b58\u5728\u7684\u4fe1\u606f\u3002\n")
	builder.WriteString("3. \u56de\u7b54\u4fdd\u6301\u7b80\u6d01\u51c6\u786e\u3002\n")
	return builder.String()
}

func formatMessageLine(msg model.Message, userDisplayNames map[uint64]string) string {
	content := strings.TrimSpace(model.MessagePreview(msg.MessageType, msg.Content))
	switch msg.SenderType {
	case model.SenderTypeBot:
		return fmt.Sprintf("[AIM]: %s", content)
	case model.SenderTypeSystem:
		return fmt.Sprintf("[SYSTEM]: %s", content)
	default:
		return fmt.Sprintf("[%s]: %s", userDisplayName(msg.SenderID, userDisplayNames), content)
	}
}

func userDisplayName(userID uint64, userDisplayNames map[uint64]string) string {
	if userDisplayNames != nil {
		if name := strings.TrimSpace(userDisplayNames[userID]); name != "" {
			return name
		}
	}
	return fmt.Sprintf("\u7528\u6237%d", userID)
}
