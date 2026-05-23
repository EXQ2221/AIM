package bot

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
	llm "example.com/aim/chat-service/llm-internal/client"
)

type MemoryCandidate struct {
	ID      uint64
	Content string
}

type UserMemoryExtractRequest struct {
	Question   string
	Candidates []MemoryCandidate
}

type UserMemoryExtractResult struct {
	RecallIDs   []uint64
	NewMemories []string
}

type UserMemoryExtractor interface {
	Extract(ctx context.Context, req UserMemoryExtractRequest) (UserMemoryExtractResult, error)
}

type LLMUserMemoryExtractor struct {
	Client llm.Client
	Model  string
}

func (e *LLMUserMemoryExtractor) Extract(ctx context.Context, req UserMemoryExtractRequest) (UserMemoryExtractResult, error) {
	if e == nil || e.Client == nil || strings.TrimSpace(e.Model) == "" {
		return UserMemoryExtractResult{}, fmt.Errorf("memory extractor is not configured")
	}

	var candidateBuilder strings.Builder
	for _, item := range req.Candidates {
		content := strings.TrimSpace(item.Content)
		if item.ID == 0 || content == "" {
			continue
		}
		candidateBuilder.WriteString(fmt.Sprintf("- id=%d content=%s\n", item.ID, content))
	}
	candidateText := candidateBuilder.String()
	if strings.TrimSpace(candidateText) == "" {
		candidateText = "(none)"
	}

	systemPrompt := strings.Join([]string{
		"你是用户长期记忆提取器。",
		"你的任务是：根据用户当前问题，从候选记忆中选出最相关的记忆ID，并提取可长期保存的新记忆。",
		"必须返回 JSON 对象，格式：{\"recall_ids\":[1,2],\"new_memories\":[\"...\",\"...\"]}",
		"规则：",
		"1) recall_ids 只能包含候选里的 id；",
		"2) new_memories 只保留稳定且长期有价值的信息（偏好、身份、长期项目、约束），不要包含一次性上下文；",
		"3) 每条 new_memories 最长 120 字；",
		"4) 如果无相关内容，返回空数组。",
	}, "\n")

	userPrompt := fmt.Sprintf("当前问题：%s\n\n候选记忆：\n%s", strings.TrimSpace(req.Question), candidateText)
	resp, err := e.Client.Generate(ctx, llm.GenerateRequest{
		Model: e.Model,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return UserMemoryExtractResult{}, err
	}

	return parseMemoryExtractResult(resp.Content)
}

func parseMemoryExtractResult(raw string) (UserMemoryExtractResult, error) {
	type payload struct {
		RecallIDs   []uint64 `json:"recall_ids"`
		NewMemories []string `json:"new_memories"`
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return UserMemoryExtractResult{}, nil
	}

	var p payload
	if err := json.Unmarshal([]byte(trimmed), &p); err == nil {
		return UserMemoryExtractResult{
			RecallIDs:   p.RecallIDs,
			NewMemories: p.NewMemories,
		}, nil
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		candidate := trimmed[start : end+1]
		if err := json.Unmarshal([]byte(candidate), &p); err == nil {
			return UserMemoryExtractResult{
				RecallIDs:   p.RecallIDs,
				NewMemories: p.NewMemories,
			}, nil
		}
	}
	return UserMemoryExtractResult{}, fmt.Errorf("memory extractor returned invalid json")
}

func (s *Service) collectLongTermMemories(ctx context.Context, req HandleMentionRequest, question string) []string {
	if s == nil || s.UserMemoryRepo == nil || s.UserMemoryExtractor == nil {
		return nil
	}
	question = strings.TrimSpace(question)
	if question == "" {
		return nil
	}

	candidateLimit := s.MemoryCandidateLimit
	if candidateLimit <= 0 {
		candidateLimit = 20
	}
	candidateItems, err := s.UserMemoryRepo.ListRecentByUserID(ctx, req.UserID, candidateLimit)
	if err != nil {
		log.Printf("memory list failed: user=%d err=%v", req.UserID, err)
		return nil
	}
	candidates := make([]MemoryCandidate, 0, len(candidateItems))
	for _, item := range candidateItems {
		content := strings.TrimSpace(item.Content)
		if item.ID == 0 || content == "" {
			continue
		}
		candidates = append(candidates, MemoryCandidate{
			ID:      item.ID,
			Content: content,
		})
	}

	extractCtx := ctx
	cancel := func() {}
	if s.MemoryExtractTimeout > 0 {
		extractCtx, cancel = context.WithTimeout(ctx, s.MemoryExtractTimeout)
	}
	defer cancel()

	result, err := s.UserMemoryExtractor.Extract(extractCtx, UserMemoryExtractRequest{
		Question:   question,
		Candidates: candidates,
	})
	if err != nil {
		log.Printf("memory extract degraded: user=%d err=%v", req.UserID, err)
		return nil
	}

	now := time.Now()
	candidateMap := make(map[uint64]string, len(candidates))
	for _, item := range candidates {
		candidateMap[item.ID] = item.Content
	}

	recallLimit := s.MemoryMaxRecall
	if recallLimit <= 0 {
		recallLimit = 5
	}
	selected := make([]string, 0, recallLimit)
	touchedIDs := make([]uint64, 0, recallLimit)
	seenID := make(map[uint64]struct{}, recallLimit)
	for _, id := range result.RecallIDs {
		if len(selected) >= recallLimit {
			break
		}
		if _, exists := seenID[id]; exists {
			continue
		}
		content, ok := candidateMap[id]
		if !ok || strings.TrimSpace(content) == "" {
			continue
		}
		seenID[id] = struct{}{}
		selected = append(selected, content)
		touchedIDs = append(touchedIDs, id)
	}
	if len(touchedIDs) > 0 {
		if err := s.UserMemoryRepo.TouchByIDs(context.Background(), req.UserID, touchedIDs, now); err != nil {
			log.Printf("memory touch failed: user=%d err=%v", req.UserID, err)
		}
	}

	if len(result.NewMemories) > 0 {
		seenHash := make(map[string]struct{}, len(result.NewMemories))
		for _, raw := range result.NewMemories {
			content := strings.TrimSpace(raw)
			if content == "" {
				continue
			}
			if len([]rune(content)) > 120 {
				runes := []rune(content)
				content = string(runes[:120])
			}
			hash := hashMemoryContent(content)
			if hash == "" {
				continue
			}
			if _, exists := seenHash[hash]; exists {
				continue
			}
			seenHash[hash] = struct{}{}
			record := &model.UserMemory{
				UserID:               req.UserID,
				MemoryHash:           hash,
				Content:              content,
				SourceConversationID: req.ConversationID,
				SourceMessageID:      nullableID(req.RequestMessageID),
				LastUsedAt:           now,
			}
			if err := s.UserMemoryRepo.UpsertByHash(context.Background(), record); err != nil {
				log.Printf("memory upsert failed: user=%d err=%v", req.UserID, err)
			}
		}
	}

	return selected
}

func hashMemoryContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	sum := sha1.Sum([]byte(strings.ToLower(content)))
	return hex.EncodeToString(sum[:])
}
