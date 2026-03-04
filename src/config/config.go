package config

import "github.com/caarlos0/env/v11"

type Config struct {
	SocksAddr string `env:"SOCKS5_ADDR" envDefault:":1080"`
	APIAddr   string `env:"API_ADDR" envDefault:":8080"`

	RedisAddr     string `env:"REDIS_ADDR" envDefault:"127.0.0.1:6379"`
	RedisPassword string `env:"REDIS_PASSWORD"`
	RedisDB       int    `env:"REDIS_DB" envDefault:"0"`

	RedisAuthUserKey string `env:"REDIS_AUTH_USER_KEY" envDefault:"user_auth"`
	RedisUsageKey    string `env:"REDIS_USAGE_KEY" envDefault:"user_usage_data"`
	RedisAuthDateKey string `env:"REDIS_AUTH_DATE_KEY" envDefault:"user_auth_date"`
	RedisEnabledKey  string `env:"REDIS_ENABLED_KEY" envDefault:"user_enabled"`

	DialTimeoutSec int `env:"DIAL_TIMEOUT_SECONDS" envDefault:"10"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
