package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"socks5-proxy/src/adapters/redisrepo"
	"socks5-proxy/src/config"
	"socks5-proxy/src/transport/httpapi"
	"socks5-proxy/src/transport/socks5"
	"socks5-proxy/src/usecase"

	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer rdb.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err = rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}

	repo := redisrepo.New(rdb, cfg)
	userService := usecase.NewUserService(repo)
	statsService := usecase.NewStatsService(repo, repo)

	socksServer := socks5.New(userService, statsService, time.Duration(cfg.DialTimeoutSec)*time.Second)
	httpTransport := httpapi.New(userService, statsService)
	httpServer := &http.Server{
		Addr:    cfg.APIAddr,
		Handler: httpTransport.Handler(),
	}

	errCh := make(chan error, 2)
	go func() {
		errCh <- socksServer.Serve(ctx, cfg.SocksAddr)
	}()
	go func() {
		log.Printf("REST API listening on %s", cfg.APIAddr)
		if serveErr := httpServer.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
		log.Println("shutdown complete")
	case runErr := <-errCh:
		if runErr != nil {
			log.Printf("server error: %v", runErr)
			os.Exit(1)
		}
	}
}
