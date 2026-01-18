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

FROM alpine:latest

# Install sqlite runtime libraries
RUN apk add --no-cache sqlite-libs curl 
RUN apk add --no-cache tzdata

COPY --from=builder /app/boardgame_night_bot .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/localization ./localization

EXPOSE 8080

ENV GIN_MODE=release
CMD ["./boardgame_night_bot"]