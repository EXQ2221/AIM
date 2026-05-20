package bot

import (
	"sync"

	"example.com/aim/shared/errno"
)

var (
	ErrGlobalConcurrencyLimitReached       = errno.Forbidden("global concurrency limit reached")
	ErrConversationConcurrencyLimitReached = errno.Forbidden("conversation concurrency limit reached")
)

type Limiter struct {
	globalSem            chan struct{}
	maxConversation      int
	mu                   sync.Mutex
	conversationInFlight map[uint64]int
}

func NewLimiter(maxGlobal, maxConversation int) *Limiter {
	if maxGlobal <= 0 {
		maxGlobal = 10
	}
	if maxConversation <= 0 {
		maxConversation = 1
	}
	return &Limiter{
		globalSem:            make(chan struct{}, maxGlobal),
		maxConversation:      maxConversation,
		conversationInFlight: make(map[uint64]int),
	}
}

func (l *Limiter) TryAcquire(conversationID uint64) (func(), error) {
	if l == nil {
		return func() {}, nil
	}

	select {
	case l.globalSem <- struct{}{}:
	default:
		return nil, ErrGlobalConcurrencyLimitReached
	}

	l.mu.Lock()
	count := l.conversationInFlight[conversationID]
	if count >= l.maxConversation {
		l.mu.Unlock()
		<-l.globalSem
		return nil, ErrConversationConcurrencyLimitReached
	}
	l.conversationInFlight[conversationID] = count + 1
	l.mu.Unlock()

	released := false
	return func() {
		l.mu.Lock()
		if !released {
			current := l.conversationInFlight[conversationID]
			if current <= 1 {
				delete(l.conversationInFlight, conversationID)
			} else {
				l.conversationInFlight[conversationID] = current - 1
			}
			released = true
			<-l.globalSem
		}
		l.mu.Unlock()
	}, nil
}
