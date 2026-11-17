FROM golang:1.24.1-alpine AS builder

WORKDIR /app

# Install build dependencies for go-sqlite3
RUN apk add --no-cache gcc musl-dev sqlite-dev

COPY go.mod go.sum ./
RUN go mod download

COPY src/ src/
COPY templates/ templates/
COPY localization/ localization/

RUN go build -o boardgame_night_bot ./src/main.go
RUN chmod +x boardgame_night_bot

# Pinned instead of alpine:latest so rebuilds are reproducible.
FROM alpine:3.22

# Install sqlite runtime libraries; su-exec drops root in the entrypoint
RUN apk add --no-cache sqlite-libs curl tzdata su-exec \
    && addgroup -S -g 10001 bot \
    && adduser -S -D -H -u 10001 -G bot bot

COPY --from=builder /app/boardgame_night_bot .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/localization ./localization
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

EXPOSE 8080

ENV GIN_MODE=release
# Starts as root only to fix ownership of the data volume, then runs as "bot".
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./boardgame_night_bot"]