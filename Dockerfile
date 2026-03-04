FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY src ./src

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/socks5-proxy ./src

FROM alpine:3.22

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /bin/socks5-proxy /usr/local/bin/socks5-proxy

USER app

EXPOSE 1080 8080

ENTRYPOINT ["/usr/local/bin/socks5-proxy"]
