package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
	llm "example.com/aim/chat-service/llm-internal/client"
	"example.com/aim/shared/errno"
)

func (s *Service) resolveLLMClientAndProvider(botModel model.Bot, providerName string, modelName string) (llm.Client, string, error) {
	client := s.LLM
	selectedProvider := strings.TrimSpace(providerName)
	if s.LLMSelector != nil {
		selectedClient, nextProvider, err := s.LLMSelector(botModel)
		if err != nil {
			return nil, selectedProvider, err
		}
		if strings.TrimSpace(nextProvider) != "" {
			selectedProvider = strings.TrimSpace(nextProvider)
		}
		if selectedClient != nil {
			client = selectedClient
		}
	}
	if client == nil {
		return nil, selectedProvider, errno.Required("llm client")
	}
	_ = modelName
	return client, selectedProvider, nil
}

func (s *Service) generateBotReply(
	ctx context.Context,
	req HandleMentionRequest,
	botModel model.Bot,
	llmClient llm.Client,
	providerName string,
	modelName string,
	prompt string,
	systemPrompt string,
	recentMessages []model.Message,
) error {
	parts, err := buildUserPromptParts(ctx, prompt, recentMessages, supportsVisionModel(modelName))
	if err != nil {
		if logErr := s.createFailedLog(ctx, req, botModel, providerName, modelName, err, 0); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; build prompt parts error: %v", logErr, err)
		}
		if _, replyErr := s.createBotReplyMessage(ctx, req, botModel, err.Error()); replyErr != nil {
			return fmt.Errorf("create bot failure reply error: %w; build prompt parts error: %v", replyErr, err)
		}
		return nil
	}

	llmCtx := ctx
	cancel := func() {}
	if s.LLMTimeout > 0 {
		llmCtx, cancel = context.WithTimeout(ctx, s.LLMTimeout)
	}
	defer cancel()

	llmReq := llm.GenerateRequest{
		Model: modelName,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{
				Role:    "user",
				Content: prompt,
				Parts:   parts,
			},
		},
	}
	logLLMRequestDebug(req, botModel, llmReq)
	start := time.Now()
	var resp *llm.GenerateResponse
	streamer, supportsStreaming := llmClient.(llmStreamingClient)
	if supportsStreaming {
		var (
			contentBuilder strings.Builder
			streamState    *botReplyStreamMeta
			firstChunkAt   time.Time
			chunkCount     int
		)
		streamMeta, metaErr := s.buildStreamMeta(ctx, req, botModel)
		if metaErr != nil {
			log.Printf("bot stream meta unavailable: conversation=%d bot=%d err=%v", req.ConversationID, botModel.ID, metaErr)
		} else {
			streamState = streamMeta
			s.publishBotReplyStream(ctx, *streamState)
		}
		resp, err = streamer.GenerateStream(llmCtx, llmReq, func(chunk llm.StreamChunk) error {
			chunkCount++
			if firstChunkAt.IsZero() && (chunk.Content != "" || chunk.ReasoningContent != "") {
				firstChunkAt = time.Now()
			}
			if chunk.Content != "" {
				contentBuilder.WriteString(chunk.Content)
				if streamState != nil {
					streamState.info.Content = contentBuilder.String()
					streamState.info.Done = false
					s.publishBotReplyStream(ctx, *streamState)
				}
			}
			return nil
		})
		log.Printf(
			"bot stream timing: conversation=%d bot=%d model=%s first_chunk_ms=%d llm_total_ms=%d chunks=%d",
			req.ConversationID,
			botModel.ID,
			modelName,
			durationMillis(start, firstChunkAt),
			time.Since(start).Milliseconds(),
			chunkCount,
		)
		if err == nil && resp != nil {
			if resp.Content == "" {
				resp.Content = contentBuilder.String()
			}
			if streamState != nil && strings.TrimSpace(resp.Content) != "" {
				streamState.info.Content = resp.Content
				streamState.info.Done = true
				s.publishBotReplyStream(ctx, *streamState)
			}
		}
	} else {
		resp, err = llmClient.Generate(llmCtx, llmReq)
		log.Printf(
			"bot llm timing: conversation=%d bot=%d model=%s llm_total_ms=%d mode=non_stream",
			req.ConversationID,
			botModel.ID,
			modelName,
			time.Since(start).Milliseconds(),
		)
	}
	latencyMS := time.Since(start).Milliseconds()
	if err != nil {
		if logErr := s.createFailedLog(ctx, req, botModel, providerName, modelName, err, latencyMS); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; llm error: %v", logErr, err)
		}
		var statusErr *llm.HTTPStatusError
		if errors.As(err, &statusErr) {
			if _, replyErr := s.createBotReplyMessage(ctx, req, botModel, "模型调用失败，请稍后再试。"); replyErr != nil {
				return fmt.Errorf("create bot failure reply error: %w; llm error: %v", replyErr, err)
			}
			return nil
		}
		return err
	}
	if resp == nil {
		err := errno.Internal("llm response is nil")
		if logErr := s.createFailedLog(ctx, req, botModel, providerName, modelName, err, latencyMS); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; bot error: %v", logErr, err)
		}
		return err
	}
	botMessage, err := s.createBotReplyMessage(ctx, req, botModel, resp.Content)
	if err != nil {
		if logErr := s.createFailedLog(ctx, req, botModel, providerName, modelName, err, latencyMS); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; create bot reply error: %v", logErr, err)
		}
		return err
	}
	return s.createSuccessLog(ctx, req, botModel, providerName, modelName, botMessage.ID, resp, latencyMS)
}
