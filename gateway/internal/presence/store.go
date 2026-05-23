package presence

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"

	redisv9 "github.com/redis/go-redis/v9"
)

const invisibleKeyPrefix = "aim:presence:invisible:"

type Store interface {
	GetInvisible(ctx context.Context, userID int64) (bool, error)
	SetInvisible(ctx context.Context, userID int64, invisible bool) error
}

type memoryStore struct {
	mu        sync.RWMutex
	invisible map[int64]bool
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		invisible: make(map[int64]bool),
	}
}

func (s *memoryStore) GetInvisible(_ context.Context, userID int64) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.invisible[userID], nil
}

func (s *memoryStore) SetInvisible(_ context.Context, userID int64, invisible bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if invisible {
		s.invisible[userID] = true
		return nil
	}
	delete(s.invisible, userID)
	return nil
}

type redisStore struct {
	client *redisv9.Client
}

func newRedisStore(client *redisv9.Client) *redisStore {
	return &redisStore{client: client}
}

func (s *redisStore) GetInvisible(ctx context.Context, userID int64) (bool, error) {
	value, err := s.client.Get(ctx, invisibleKey(userID)).Result()
	if err != nil {
		if errors.Is(err, redisv9.Nil) {
			return false, nil
		}
		return false, err
	}
	return value == "1" || strings.EqualFold(value, "true"), nil
}

func (s *redisStore) SetInvisible(ctx context.Context, userID int64, invisible bool) error {
	if invisible {
		return s.client.Set(ctx, invisibleKey(userID), "1", 0).Err()
	}
	return s.client.Del(ctx, invisibleKey(userID)).Err()
}

func invisibleKey(userID int64) string {
	return invisibleKeyPrefix + strconv.FormatInt(userID, 10)
}

var (
	defaultStoreOnce sync.Once
	defaultStore     Store
)

func DefaultStore() Store {
	defaultStoreOnce.Do(func() {
		redisAddr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
		if redisAddr != "" {
			client := redisv9.NewClient(&redisv9.Options{Addr: redisAddr})
			if err := client.Ping(context.Background()).Err(); err == nil {
				defaultStore = newRedisStore(client)
				return
			}
		}
		defaultStore = newMemoryStore()
	})
	return defaultStore
}
