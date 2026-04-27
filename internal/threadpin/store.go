package threadpin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultTTL = 7 * 24 * time.Hour

// Store records which squad employee owns follow-up turns for a Slack thread root
// (channel + parent message ts) after an explicit @mention + mutating skill kickoff.
type Store struct {
	rdb *redis.Client
}

// NewStoreFromURL parses redis:// or rediss:// (same URL style as termsredis).
func NewStoreFromURL(redisURL string) (*Store, error) {
	opts, err := redis.ParseURL(strings.TrimSpace(redisURL))
	if err != nil {
		return nil, fmt.Errorf("threadpin: parse redis url: %w", err)
	}
	return &Store{rdb: redis.NewClient(opts)}, nil
}

// Close closes the Redis client.
func (s *Store) Close() error {
	if s == nil || s.rdb == nil {
		return nil
	}
	return s.rdb.Close()
}

func pinKey(teamID, channelID, threadRootTS string) string {
	return fmt.Sprintf("orchestrator:v1:thread_skill_followup:%s:%s:%s",
		strings.TrimSpace(teamID),
		strings.TrimSpace(channelID),
		strings.TrimSpace(threadRootTS),
	)
}

// SetFollowupEmployee pins threadRootTS → employeeKey with TTL.
func (s *Store) SetFollowupEmployee(ctx context.Context, teamID, channelID, threadRootTS, employeeKey string) error {
	if s == nil || s.rdb == nil {
		return fmt.Errorf("threadpin: nil store")
	}
	tid, ch, root, emp := strings.TrimSpace(teamID), strings.TrimSpace(channelID), strings.TrimSpace(threadRootTS), strings.ToLower(strings.TrimSpace(employeeKey))
	if tid == "" || ch == "" || root == "" || emp == "" {
		return fmt.Errorf("threadpin: empty key field")
	}
	k := pinKey(tid, ch, root)
	return s.rdb.Set(ctx, k, emp, defaultTTL).Err()
}

// GetFollowupEmployee returns the pinned employee key, or ("", nil) when missing.
func (s *Store) GetFollowupEmployee(ctx context.Context, teamID, channelID, threadRootTS string) (string, error) {
	if s == nil || s.rdb == nil {
		return "", fmt.Errorf("threadpin: nil store")
	}
	tid, ch, root := strings.TrimSpace(teamID), strings.TrimSpace(channelID), strings.TrimSpace(threadRootTS)
	if tid == "" || ch == "" || root == "" {
		return "", nil
	}
	v, err := s.rdb.Get(ctx, pinKey(tid, ch, root)).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	out := strings.ToLower(strings.TrimSpace(v))
	if out == "" {
		return "", nil
	}
	return out, nil
}
