package usecase

import (
	"context"
	"errors"
	"strings"

	"socks5-proxy/src/domain"
)

type UserService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Create(ctx context.Context, username, password string, enabled bool) error {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return errors.New("username and password are required")
	}
	return s.repo.Create(ctx, username, password, enabled)
}

func (s *UserService) Update(ctx context.Context, username string, password *string, enabled *bool) error {
	if password != nil && strings.TrimSpace(*password) == "" {
		return errors.New("password cannot be empty")
	}
	return s.repo.Update(ctx, username, password, enabled)
}

func (s *UserService) Delete(ctx context.Context, username string) error {
	return s.repo.Delete(ctx, username)
}

func (s *UserService) Get(ctx context.Context, username string) (domain.User, error) {
	return s.repo.GetUser(ctx, username)
}

func (s *UserService) List(ctx context.Context) ([]domain.User, error) {
	return s.repo.ListUsers(ctx)
}

func (s *UserService) ValidateCredentials(ctx context.Context, username, password string) (bool, error) {
	return s.repo.ValidateCredentials(ctx, username, password)
}

func (s *UserService) MarkAuthenticated(ctx context.Context, username string) error {
	return s.repo.MarkAuthenticated(ctx, username)
}
