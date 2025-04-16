FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o bot ./cmd

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache sqlite

COPY --from=builder /app/bot .
COPY --from=builder /app/config.yaml .     # если config.yaml нужен
COPY --from=builder /app/bot.db .          # если хочешь копировать начальную БД (опционально)

CMD ["./bot"]
