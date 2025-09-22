# syntax=docker/dockerfile:1

FROM golang:1.23 AS builder
WORKDIR /app

# Pre-cache modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the remaining source
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/server ./cmd/server

FROM alpine:3.20
WORKDIR /app

RUN adduser -D -g '' appuser \
    && apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/server ./server
COPY static ./static

ENV PORT=8080
EXPOSE 8080

USER appuser
ENTRYPOINT ["./server"]
