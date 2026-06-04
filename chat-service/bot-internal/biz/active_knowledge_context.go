package bot

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

type activeKnowledgeContext struct {
	UserID         uint64
	ConversationID uint64
	BotID          uint64
	Family         workflowFamily
	OutputMode     workflowOutputMode
	EvidenceMode   workflowEvidenceMode
	KnowledgeScope string
	LastQuestion   string
	LastAnswer     string
	PrimaryDocID   uint64
	PrimaryDocTitle string
	DocumentRefs   []knowledgeContextDocument
	UpdatedAt      time.Time
}

type knowledgeContextDocument struct {
	DocumentID uint64
	Title      string
}

type activeKnowledgeContextStore struct {
	mu       sync.RWMutex
	items    map[string]activeKnowledgeContext
	ttl      time.Duration
	maxItems int
}

func newActiveKnowledgeContextStore(ttl time.Duration) *activeKnowledgeContextStore {
	if ttl <= 0 {
		ttl = 20 * time.Minute
	}
	return &activeKnowledgeContextStore{
		items:    make(map[string]activeKnowledgeContext),
		ttl:      ttl,
		maxItems: 512,
	}
}

func (s *activeKnowledgeContextStore) Get(userID uint64, conversationID uint64, botID uint64) (activeKnowledgeContext, bool) {
	if s == nil || userID == 0 || conversationID == 0 || botID == 0 {
		return activeKnowledgeContext{}, false
	}
	key := activeKnowledgeContextKey(userID, conversationID, botID)
	s.mu.RLock()
	item, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return activeKnowledgeContext{}, false
	}
	if s.ttl > 0 && time.Since(item.UpdatedAt) > s.ttl {
		s.mu.Lock()
		delete(s.items, key)
		s.mu.Unlock()
		return activeKnowledgeContext{}, false
	}
	return item, true
}

func (s *activeKnowledgeContextStore) Set(item activeKnowledgeContext) {
	if s == nil || item.UserID == 0 || item.ConversationID == 0 || item.BotID == 0 {
		return
	}
	item.LastQuestion = strings.TrimSpace(item.LastQuestion)
	item.LastAnswer = strings.TrimSpace(item.LastAnswer)
	item.PrimaryDocTitle = strings.TrimSpace(item.PrimaryDocTitle)
	item.DocumentRefs = normalizeKnowledgeContextDocuments(item.DocumentRefs)
	item.UpdatedAt = time.Now()
	key := activeKnowledgeContextKey(item.UserID, item.ConversationID, item.BotID)

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) >= s.maxItems {
		s.gcLocked()
	}
	s.items[key] = item
}

func (s *activeKnowledgeContextStore) Delete(userID uint64, conversationID uint64, botID uint64) {
	if s == nil || userID == 0 || conversationID == 0 || botID == 0 {
		return
	}
	s.mu.Lock()
	delete(s.items, activeKnowledgeContextKey(userID, conversationID, botID))
	s.mu.Unlock()
}

func (s *activeKnowledgeContextStore) gcLocked() {
	if s == nil {
		return
	}
	if s.ttl > 0 {
		now := time.Now()
		for key, item := range s.items {
			if now.Sub(item.UpdatedAt) > s.ttl {
				delete(s.items, key)
			}
		}
	}
	if len(s.items) < s.maxItems {
		return
	}
	var oldestKey string
	var oldestAt time.Time
	first := true
	for key, item := range s.items {
		if first || item.UpdatedAt.Before(oldestAt) {
			first = false
			oldestKey = key
			oldestAt = item.UpdatedAt
		}
	}
	if oldestKey != "" {
		delete(s.items, oldestKey)
	}
}

func activeKnowledgeContextKey(userID uint64, conversationID uint64, botID uint64) string {
	return strings.Join([]string{uint64ToString(userID), uint64ToString(conversationID), uint64ToString(botID)}, ":")
}

func uint64ToString(value uint64) string {
	return strconv.FormatUint(value, 10)
}

func normalizeKnowledgeContextDocuments(items []knowledgeContextDocument) []knowledgeContextDocument {
	if len(items) == 0 {
		return nil
	}
	result := make([]knowledgeContextDocument, 0, len(items))
	seen := make(map[uint64]struct{}, len(items))
	for _, item := range items {
		if item.DocumentID == 0 {
			continue
		}
		if _, exists := seen[item.DocumentID]; exists {
			continue
		}
		seen[item.DocumentID] = struct{}{}
		item.Title = strings.TrimSpace(item.Title)
		result = append(result, item)
	}
	return result
}
