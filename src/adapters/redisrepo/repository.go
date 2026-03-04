package redisrepo

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"socks5-proxy/src/config"
	"socks5-proxy/src/domain"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type Repository struct {
	client *redis.Client
	cfg    config.Config
}

func New(client *redis.Client, cfg config.Config) *Repository {
	return &Repository{client: client, cfg: cfg}
}

func (r *Repository) Create(ctx context.Context, username, password string, enabled bool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	pipe := r.client.TxPipeline()
	pipe.HSet(ctx, r.cfg.RedisAuthUserKey, username, string(hash))
	pipe.HSet(ctx, r.cfg.RedisEnabledKey, username, strconv.FormatBool(enabled))
	_, err = pipe.Exec(ctx)
	return err
}

func (r *Repository) Update(ctx context.Context, username string, password *string, enabled *bool) error {
	u, err := r.GetUser(ctx, username)
	if err != nil {
		return err
	}

	newHash := u.Password
	if password != nil {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
		if hashErr != nil {
			return hashErr
		}
		newHash = string(hash)
	}

	newEnabled := u.Enabled
	if enabled != nil {
		newEnabled = *enabled
	}

	pipe := r.client.TxPipeline()
	pipe.HSet(ctx, r.cfg.RedisAuthUserKey, username, newHash)
	pipe.HSet(ctx, r.cfg.RedisEnabledKey, username, strconv.FormatBool(newEnabled))
	_, err = pipe.Exec(ctx)
	return err
}

func (r *Repository) Delete(ctx context.Context, username string) error {
	pipe := r.client.TxPipeline()
	pipe.HDel(ctx, r.cfg.RedisAuthUserKey, username)
	pipe.HDel(ctx, r.cfg.RedisEnabledKey, username)
	pipe.HDel(ctx, r.cfg.RedisUsageKey, username)
	pipe.HDel(ctx, r.cfg.RedisAuthDateKey, username)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *Repository) GetUser(ctx context.Context, username string) (domain.User, error) {
	hash, err := r.client.HGet(ctx, r.cfg.RedisAuthUserKey, username).Result()
	if err != nil {
		if err == redis.Nil {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, err
	}

	enabledRaw, err := r.client.HGet(ctx, r.cfg.RedisEnabledKey, username).Result()
	if err != nil && err != redis.Nil {
		return domain.User{}, err
	}

	enabled := true
	if strings.TrimSpace(enabledRaw) != "" {
		parsed, parseErr := strconv.ParseBool(enabledRaw)
		if parseErr == nil {
			enabled = parsed
		}
	}

	return domain.User{Username: username, Password: hash, Enabled: enabled}, nil
}

func (r *Repository) ListUsers(ctx context.Context) ([]domain.User, error) {
	usernames, err := r.client.HKeys(ctx, r.cfg.RedisAuthUserKey).Result()
	if err != nil {
		if err == redis.Nil {
			return []domain.User{}, nil
		}
		return nil, err
	}

	users := make([]domain.User, 0, len(usernames))
	for _, username := range usernames {
		u, getErr := r.GetUser(ctx, username)
		if getErr == nil {
			users = append(users, u)
		}
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].Username < users[j].Username
	})

	return users, nil
}

func (r *Repository) ValidateCredentials(ctx context.Context, username, password string) (bool, error) {
	u, err := r.GetUser(ctx, username)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return false, nil
		}
		return false, err
	}
	if !u.Enabled {
		return false, nil
	}
	if compareErr := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); compareErr != nil {
		return false, nil
	}
	return true, nil
}

func (r *Repository) MarkAuthenticated(ctx context.Context, username string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.client.HSet(ctx, r.cfg.RedisAuthDateKey, username, now).Err()
}

func (r *Repository) AddTraffic(ctx context.Context, username string, upload, download int64) error {
	if username == "" {
		return nil
	}
	total := upload + download
	return r.client.HIncrBy(ctx, r.cfg.RedisUsageKey, username, total).Err()
}

func (r *Repository) GetStats(ctx context.Context, username string) (domain.UserStats, error) {
	totalRaw, err := r.client.HGet(ctx, r.cfg.RedisUsageKey, username).Result()
	if err != nil && err != redis.Nil {
		return domain.UserStats{}, err
	}
	lastAuthDate, err := r.client.HGet(ctx, r.cfg.RedisAuthDateKey, username).Result()
	if err != nil && err != redis.Nil {
		return domain.UserStats{}, err
	}

	totalBytes, _ := strconv.ParseInt(totalRaw, 10, 64)
	return domain.UserStats{
		Username:      username,
		UploadBytes:   0,
		DownloadBytes: 0,
		TotalBytes:    totalBytes,
		UpdatedAt:     lastAuthDate,
	}, nil
}
