package bot

import (
	"sync"
	"time"
)

type sharedKnowledgeSnapshot struct {
	MessageID      uint64
	ConversationID uint64
	BotID          uint64
	Family         workflowFamily
	OutputMode     workflowOutputMode
	EvidenceMode   workflowEvidenceMode
	KnowledgeScope string
	Question       string
	Answer         string
	PrimaryDocID   uint64
	PrimaryDocTitle string
	DocumentRefs   []knowledgeContextDocument
	CreatedAt      time.Time
}

type sharedKnowledgeSnapshotStore struct {
	mu       sync.RWMutex
	items    map[uint64]sharedKnowledgeSnapshot
	ttl      time.Duration
	maxItems int
}

func newSharedKnowledgeSnapshotStore(ttl time.Duration) *sharedKnowledgeSnapshotStore {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &sharedKnowledgeSnapshotStore{
		items:    make(map[uint64]sharedKnowledgeSnapshot),
		ttl:      ttl,
		maxItems: 1024,
	}
}

func (s *sharedKnowledgeSnapshotStore) Get(messageID uint64) (sharedKnowledgeSnapshot, bool) {
	if s == nil || messageID == 0 {
		return sharedKnowledgeSnapshot{}, false
	}
	s.mu.RLock()
	item, ok := s.items[messageID]
	s.mu.RUnlock()
	if !ok {
		return sharedKnowledgeSnapshot{}, false
	}
	if s.ttl > 0 && time.Since(item.CreatedAt) > s.ttl {
		s.mu.Lock()
		delete(s.items, messageID)
		s.mu.Unlock()
		return sharedKnowledgeSnapshot{}, false
	}
	return item, true
}

func (s *sharedKnowledgeSnapshotStore) Set(item sharedKnowledgeSnapshot) {
	if s == nil || item.MessageID == 0 || item.ConversationID == 0 || item.BotID == 0 {
		return
	}
	item.DocumentRefs = normalizeKnowledgeContextDocuments(item.DocumentRefs)
	item.CreatedAt = time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) >= s.maxItems {
		s.gcLocked()
	}
	s.items[item.MessageID] = item
}

func (s *sharedKnowledgeSnapshotStore) gcLocked() {
	if s == nil {
		return
	}
	if s.ttl > 0 {
		now := time.Now()
		for key, item := range s.items {
			if now.Sub(item.CreatedAt) > s.ttl {
				delete(s.items, key)
			}
		}
	}
	if len(s.items) < s.maxItems {
		return
	}
	var oldestID uint64
	var oldestAt time.Time
	first := true
	for id, item := range s.items {
		if first || item.CreatedAt.Before(oldestAt) {
			first = false
			oldestID = id
			oldestAt = item.CreatedAt
		}
	}
	if oldestID > 0 {
		delete(s.items, oldestID)
	}
}
