# socks5-proxy

SOCKS5-прокси на Go с:
- аутентификацией пользователей через Redis;
- учетом потребления трафика по пользователям;
- REST API для CRUD операций над пользователями и просмотра статистики;
- Redis-схемой, совместимой с `nskondratev/socks5-proxy-server`.

## Архитектура

Проект реализован в стиле clean architecture:
- `internal/domain` — доменные модели и контракты;
- `internal/usecase` — бизнес-логика;
- `internal/adapters` — адаптеры (Redis);
- `internal/transport` — входные транспорты (SOCKS5, HTTP);
- `src/main.go` — composition root.

## Redis-схема (совместима с nskondratev)

Используются hash-ключи:
- `user_auth` — `username -> bcrypt_hash_password`;
- `user_usage_data` — `username -> total_bytes`;
- `user_auth_date` — `username -> ISO datetime`.

Дополнительно для REST API:
- `user_enabled` — `username -> true|false`.

Если `user_enabled` не задан, пользователь считается включенным (`true`).

## Переменные окружения

- `SOCKS5_ADDR` (по умолчанию `:1080`)
- `API_ADDR` (по умолчанию `:8080`)
- `REDIS_ADDR` (по умолчанию `127.0.0.1:6379`)
- `REDIS_PASSWORD` (по умолчанию пусто)
- `REDIS_DB` (по умолчанию `0`)
- `REDIS_AUTH_USER_KEY` (по умолчанию `user_auth`)
- `REDIS_USAGE_KEY` (по умолчанию `user_usage_data`)
- `REDIS_AUTH_DATE_KEY` (по умолчанию `user_auth_date`)
- `REDIS_ENABLED_KEY` (по умолчанию `user_enabled`)
- `DIAL_TIMEOUT_SECONDS` (по умолчанию `10`)

## Локальный запуск

Требования:
- Go >= 1.26
- Redis

1. Запустите Redis.
2. Запустите приложение:
```bash
go run ./src
```

## Docker запуск

```bash
docker compose up -d --build
```

Сервисы:
- SOCKS5: `localhost:1080`
- REST API: `http://localhost:8080`
- Redis: `localhost:6379`

Остановить:
```bash
docker compose down
```

С очисткой данных Redis:
```bash
docker compose down -v
```

## REST API

### Health
- `GET /healthz`

### Users
- `POST /users`
- `GET /users`
- `GET /users/{username}`
- `PUT /users/{username}`
- `DELETE /users/{username}`

Пример создания пользователя:
```bash
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"username":"user1","password":"pass1","enabled":true}'
```

### Stats
- `GET /stats`
- `GET /stats/{username}`

Пример:
```bash
curl http://localhost:8080/stats/user1
```

## Проверка работы SOCKS5

Пример cURL через SOCKS5:
```bash
curl --proxy socks5h://user1:pass1@127.0.0.1:1080 https://ifconfig.me
```
