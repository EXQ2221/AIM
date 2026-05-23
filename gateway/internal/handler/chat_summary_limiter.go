package handler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
)

const conversationSummaryKeyPrefix = "aim:conversation-summary:daily:"

type conversationSummaryLimiter interface {
	Allow(ctx context.Context, userID int64, limit int) (allowed bool, remaining int, err error)
}

type memoryConversationSummaryLimiter struct {
	mu    sync.Mutex
	count map[string]int
}

func newMemoryConversationSummaryLimiter() *memoryConversationSummaryLimiter {
	return &memoryConversationSummaryLimiter{
		count: make(map[string]int),
	}
}

func (l *memoryConversationSummaryLimiter) Allow(_ context.Context, userID int64, limit int) (bool, int, error) {
	if limit <= 0 {
		limit = 2
	}
	key := limiterDailyKey(userID, time.Now())
	l.mu.Lock()
	defer l.mu.Unlock()
	l.count[key]++
	current := l.count[key]
	remaining := limit - current
	if remaining < 0 {
		remaining = 0
	}
	return current <= limit, remaining, nil
}

type redisConversationSummaryLimiter struct {
	client *redisv9.Client
}

func newRedisConversationSummaryLimiter(client *redisv9.Client) *redisConversationSummaryLimiter {
	return &redisConversationSummaryLimiter{client: client}
}

func (l *redisConversationSummaryLimiter) Allow(ctx context.Context, userID int64, limit int) (bool, int, error) {
	if l == nil || l.client == nil {
		return false, 0, errors.New("redis limiter is not initialized")
	}
	if limit <= 0 {
		limit = 2
	}
	now := time.Now()
	key := limiterDailyKey(userID, now)
	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return false, 0, err
	}
	if count == 1 {
		nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		_ = l.client.ExpireAt(ctx, key, nextDay).Err()
	}
	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}
	return count <= int64(limit), remaining, nil
}

func limiterDailyKey(userID int64, now time.Time) string {
	return fmt.Sprintf("%s%d:%s", conversationSummaryKeyPrefix, userID, now.Format("20060102"))
}

var (
	conversationSummaryLimiterOnce sync.Once
	defaultConversationSummaryRate conversationSummaryLimiter
)

func defaultConversationSummaryLimiter() conversationSummaryLimiter {
	conversationSummaryLimiterOnce.Do(func() {
		redisAddr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
		if redisAddr != "" {
			client := redisv9.NewClient(&redisv9.Options{Addr: redisAddr})
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := client.Ping(ctx).Err(); err == nil {
				defaultConversationSummaryRate = newRedisConversationSummaryLimiter(client)
				return
			}
		}
		defaultConversationSummaryRate = newMemoryConversationSummaryLimiter()
	})
	return defaultConversationSummaryRate
}

func parseConversationSummaryDailyLimit() int {
	raw := strings.TrimSpace(os.Getenv("CONVERSATION_SUMMARY_DAILY_LIMIT"))
	if raw == "" {
		return 2
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 2
	}
	return value
}

