FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o bot ./cmd

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache sqlite

COPY --from=builder /app/bot .
COPY --from=builder /app/config.yaml .

CMD ["./bot"]
