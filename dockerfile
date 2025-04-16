FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o bot ./cmd

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/bot .
COPY config.yaml .

CMD ["./bot"]
