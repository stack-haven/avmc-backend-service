package loginattempt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultPrefix      = "auth:login-attempt"
	defaultMaxAttempts = int64(5)
	defaultWindow      = 15 * time.Minute
	defaultLockout     = 15 * time.Minute
)

var ErrLocked = errors.New("login temporarily locked")

type Guard interface {
	Check(context.Context, string, string, uint32) error
	Failure(context.Context, string, string, uint32) error
	Success(context.Context, string, string, uint32) error
}

type Options struct {
	Prefix      string
	MaxAttempts int64
	Window      time.Duration
	Lockout     time.Duration
}

func OptionsFromEnv(prefix string) (Options, error) {
	opts := Options{}
	if raw, ok := os.LookupEnv(prefix + "_MAX_ATTEMPTS"); ok {
		value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || value < 3 || value > 20 {
			return Options{}, fmt.Errorf("%s_MAX_ATTEMPTS must be an integer between 3 and 20", prefix)
		}
		opts.MaxAttempts = value
	}
	for suffix, target := range map[string]*time.Duration{
		"_WINDOW":  &opts.Window,
		"_LOCKOUT": &opts.Lockout,
	} {
		raw, ok := os.LookupEnv(prefix + suffix)
		if !ok {
			continue
		}
		value, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil || value < time.Minute || value > 24*time.Hour {
			return Options{}, fmt.Errorf("%s%s must be a duration between 1m and 24h", prefix, suffix)
		}
		*target = value
	}
	return opts, nil
}

type RedisGuard struct {
	client      *redis.Client
	prefix      string
	maxAttempts int64
	window      time.Duration
	lockout     time.Duration
}

func NewRedisGuard(client *redis.Client, opts Options) *RedisGuard {
	if opts.Prefix == "" {
		opts.Prefix = defaultPrefix
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = defaultMaxAttempts
	}
	if opts.Window <= 0 {
		opts.Window = defaultWindow
	}
	if opts.Lockout <= 0 {
		opts.Lockout = defaultLockout
	}
	return &RedisGuard{
		client:      client,
		prefix:      opts.Prefix,
		maxAttempts: opts.MaxAttempts,
		window:      opts.Window,
		lockout:     opts.Lockout,
	}
}

func (g *RedisGuard) Check(ctx context.Context, scope, identity string, tenantID uint32) error {
	if g == nil || g.client == nil {
		return errors.New("login attempt guard is unavailable")
	}
	locked, err := g.client.Exists(ctx, g.lockKey(scope, identity, tenantID)).Result()
	if err != nil {
		return fmt.Errorf("checking login lock: %w", err)
	}
	if locked > 0 {
		return ErrLocked
	}
	return nil
}

func (g *RedisGuard) Failure(ctx context.Context, scope, identity string, tenantID uint32) error {
	if g == nil || g.client == nil {
		return errors.New("login attempt guard is unavailable")
	}
	result, err := recordFailureScript.Run(
		ctx,
		g.client,
		[]string{g.attemptKey(scope, identity, tenantID), g.lockKey(scope, identity, tenantID)},
		g.maxAttempts,
		g.window.Milliseconds(),
		g.lockout.Milliseconds(),
	).Int64()
	if err != nil {
		return fmt.Errorf("recording login failure: %w", err)
	}
	if result >= g.maxAttempts {
		return ErrLocked
	}
	return nil
}

func (g *RedisGuard) Success(ctx context.Context, scope, identity string, tenantID uint32) error {
	if g == nil || g.client == nil {
		return errors.New("login attempt guard is unavailable")
	}
	if err := g.client.Del(ctx, g.attemptKey(scope, identity, tenantID), g.lockKey(scope, identity, tenantID)).Err(); err != nil {
		return fmt.Errorf("resetting login failures: %w", err)
	}
	return nil
}

func (g *RedisGuard) attemptKey(scope, identity string, tenantID uint32) string {
	return g.key("attempt", scope, identity, tenantID)
}

func (g *RedisGuard) lockKey(scope, identity string, tenantID uint32) string {
	return g.key("lock", scope, identity, tenantID)
}

func (g *RedisGuard) key(kind, scope, identity string, tenantID uint32) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(identity))))
	return fmt.Sprintf("%s:%s:%s:%d:%s", g.prefix, kind, scope, tenantID, hex.EncodeToString(sum[:]))
}

var recordFailureScript = redis.NewScript(`
local attempts = redis.call("INCR", KEYS[1])
if attempts == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
if attempts >= tonumber(ARGV[1]) then
  redis.call("SET", KEYS[2], "1", "PX", ARGV[3])
  redis.call("DEL", KEYS[1])
end
return attempts
`)
