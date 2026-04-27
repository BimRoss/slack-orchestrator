package termsredis

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

// KeyPrefix matches makeacompany-ai backend/internal/app/store.go (user profiles + slack index).
const KeyPrefix = "makeacompany"

// Checker reads Joanne #humans terms acceptance from the same Redis keys as makeacompany /admin Slack Users enrichment.
type Checker struct {
	rdb *redis.Client
}

// NewCheckerFromURL parses a redis:// or rediss:// URL (same as employee-factory REDIS_URL).
func NewCheckerFromURL(redisURL string) (*Checker, error) {
	opts, err := redis.ParseURL(strings.TrimSpace(redisURL))
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return &Checker{rdb: redis.NewClient(opts)}, nil
}

// Close closes the Redis client.
func (c *Checker) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func userBySlackRedisKey(slackUserID string) string {
	return KeyPrefix + ":user_by_slack:" + strings.TrimSpace(slackUserID)
}

func userProfileRedisKey(email string) string {
	return fmt.Sprintf("%s:user_profile:%s", KeyPrefix, normalizeEmail(email))
}

// HumansTermsAccepted is true when profile hash has non-empty humans_terms_accepted_at for this Slack user
// (resolved via makeacompany:user_by_slack:<U…> → email → makeacompany:user_profile:<email>).
// Missing slack index, missing profile, or empty acceptance timestamp → false, nil error.
func (c *Checker) HumansTermsAccepted(ctx context.Context, slackUserID string) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, fmt.Errorf("nil termsredis checker")
	}
	sid := strings.TrimSpace(slackUserID)
	if sid == "" {
		return false, nil
	}
	emailRaw, err := c.rdb.Get(ctx, userBySlackRedisKey(sid)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	em := normalizeEmail(emailRaw)
	if em == "" || !strings.Contains(em, "@") {
		return false, nil
	}
	at, err := c.rdb.HGet(ctx, userProfileRedisKey(em), "humans_terms_accepted_at").Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(at) != "", nil
}
