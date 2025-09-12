# Makefile (optional)
SHELL := /usr/bin/env bash

DB_URL ?= postgres://rpatt:admin@localhost:5432/go_cmms?sslmode=disable

.PHONY: tools migrate-up migrate-down migrate-version sqlc run

tools:
	sudo apt update
	sudo snap install sqlc
	# For migrate, you'll likely need to download it from GitHub releases
	curl -L https://github.com/golang-migrate/migrate/releases/download/v4.15.2/migrate.linux-amd64.tar.gz | tar xvz
	sudo mv migrate /usr/local/bin/


migrate-up:
	echo $(DB_URL)
	DATABASE_URL="$(DB_URL)" migrate -path database/schema -database "$(DB_URL)" up

migrate-down:
	DATABASE_URL="$(DB_URL)" migrate -path database/schema -database "$(DB_URL)" down

migrate-redo:
	DATABASE_URL="$(DB_URL)" migrate -path database/schema -database "$(DB_URL)" down 1
	DATABASE_URL="$(DB_URL)" migrate -path database/schema -database "$(DB_URL)" up 1

migrate-version:
	DATABASE_URL="$(DB_URL)" migrate -path database/schema -database "$(DB_URL)" version

sqlc:
	cd database && sqlc generate

run:
	go run cmd/server/main.go
