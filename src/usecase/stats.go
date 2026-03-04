package usecase

import (
	"context"
	"sort"

	"socks5-proxy/src/domain"
)

type StatsService struct {
	statsRepo domain.StatsRepository
	userRepo  domain.UserRepository
}

func NewStatsService(statsRepo domain.StatsRepository, userRepo domain.UserRepository) *StatsService {
	return &StatsService{statsRepo: statsRepo, userRepo: userRepo}
}

func (s *StatsService) AddTraffic(ctx context.Context, username string, upload, download int64) error {
	return s.statsRepo.AddTraffic(ctx, username, upload, download)
}

func (s *StatsService) Get(ctx context.Context, username string) (domain.UserStats, error) {
	return s.statsRepo.GetStats(ctx, username)
}

func (s *StatsService) List(ctx context.Context) ([]domain.UserStats, error) {
	users, err := s.userRepo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]domain.UserStats, 0, len(users))
	for _, u := range users {
		stats, statsErr := s.statsRepo.GetStats(ctx, u.Username)
		if statsErr != nil {
			return nil, statsErr
		}
		out = append(out, stats)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Username < out[j].Username
	})

	return out, nil
}
