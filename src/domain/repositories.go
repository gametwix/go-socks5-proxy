package domain

import "context"

type UserRepository interface {
	Create(ctx context.Context, username, password string, enabled bool) error
	Update(ctx context.Context, username string, password *string, enabled *bool) error
	Delete(ctx context.Context, username string) error
	GetUser(ctx context.Context, username string) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	ValidateCredentials(ctx context.Context, username, password string) (bool, error)
	MarkAuthenticated(ctx context.Context, username string) error
}

type StatsRepository interface {
	AddTraffic(ctx context.Context, username string, upload, download int64) error
	GetStats(ctx context.Context, username string) (UserStats, error)
}
