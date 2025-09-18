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
	DATABASE_URL="$(DB_URL)" migrate -path database/schema -database "$(DB_URL)" down --all

migrate-redo:
	DATABASE_URL="$(DB_URL)" migrate -path database/schema -database "$(DB_URL)" down 1
	DATABASE_URL="$(DB_URL)" migrate -path database/schema -database "$(DB_URL)" up 1

migrate-version:
	DATABASE_URL="$(DB_URL)" migrate -path database/schema -database "$(DB_URL)" version

sqlc:
	cd database && sqlc generate

run:
	go run cmd/server/main.go

# ---- Config ----
PORT      ?= 8080
BASE_URL  ?= http://localhost:$(PORT)
SERVER    ?= go run cmd/server/main.go
HEALTH_EP ?= /health    # falls back to "/" if /health is missing

# ---- Stress test script & flags (override on CLI) ----
STRESSPY  ?= python scripts/python/stress_test.py
USERS               ?= 2
TABLES_PER_USER     ?= 2
ROWS_PER_TABLE      ?= 20
ROW_CONCURRENCY     ?= 20
SEARCH_REQUESTS     ?= 20
SEARCH_CONCURRENCY  ?= 50
SEED                ?=
OUTPUT              ?=
STRESS_OPTS         ?=   # free-form extra flags appended as-is

# Build the flag line (only include optional flags if set)
STRESS_FLAGS = \
  --base-url $(BASE_URL) \
  --users $(USERS) \
  --tables-per-user $(TABLES_PER_USER) \
  --rows-per-table $(ROWS_PER_TABLE) \
  --row-concurrency $(ROW_CONCURRENCY) \
  --search-requests $(SEARCH_REQUESTS) \
  --search-concurrency $(SEARCH_CONCURRENCY) \
  $(if $(SEED),--seed $(SEED),) \
  $(if $(OUTPUT),--output $(OUTPUT),) \
  $(STRESS_OPTS)

.PHONY: run-stress-test migrate-up migrate-down help

help:
	@echo "run-stress-test with configurable options. Examples:"
	@echo "  make run-stress-test USERS=5 TABLES_PER_USER=3 ROWS_PER_TABLE=200 SEARCH_REQUESTS=500"
	@echo "  make run-stress-test PORT=8081 STRESS_OPTS=\"--output metrics.json\""
	@echo "  make run-stress-test ROW_CONCURRENCY=8 SEARCH_CONCURRENCY=12 SEED=42"
	@echo ""
	@echo "Variables:"
	@echo "  PORT, BASE_URL, SERVER, HEALTH_EP"
	@echo "  USERS, TABLES_PER_USER, ROWS_PER_TABLE, ROW_CONCURRENCY"
	@echo "  SEARCH_REQUESTS, SEARCH_CONCURRENCY, SEED, OUTPUT, STRESS_OPTS"

run-stress-test:
	@set -euo pipefail; \
	read -p "⚠️  This will reset your DB and run a stress test. Are you sure? (y/N) " yn; \
	case $$yn in [Yy]*) ;; *) echo "❌ Aborted."; exit 1;; esac; \
	# Check if port is free
	if lsof -i :$(PORT) -sTCP:LISTEN >/dev/null 2>&1; then \
	  echo "❌ Port $(PORT) is already in use. Stop the process using it or run with PORT=<other>. e.g. lsof -i :$(PORT) ....then...  kill -9 <PID>   "; \
	  exit 1; \
	fi; \
	# Start migrations + server
	trap 'if [[ -n "$$SERVER_PID" ]]; then echo "🔻 Stopping server $$SERVER_PID"; kill $$SERVER_PID >/dev/null 2>&1 || true; fi' EXIT; \
	echo "🔻 migrate-down"; \
	$(MAKE) migrate-down; \
	echo "🔺 migrate-up"; \
	$(MAKE) migrate-up; \
	echo "▶️  starting server: $(SERVER)"; \
	$(SERVER) & \
	SERVER_PID=$$!; \
	echo "🆔 server pid: $$SERVER_PID"; \
	echo "⏳ waiting for server at $(BASE_URL)"; \
	for i in {1..60}; do \
	  if curl -fsS "$(BASE_URL)$(HEALTH_EP)" >/dev/null 2>&1 || curl -fsS "$(BASE_URL)/" >/dev/null 2>&1; then \
	    echo "✅ server is up"; break; \
	  fi; \
	  if ! kill -0 $$SERVER_PID 2>/dev/null; then echo "❌ server exited early"; exit 1; fi; \
	  sleep 0.5; \
	  if [[ $$i -eq 60 ]]; then echo "⏱️  timeout waiting for server"; exit 1; fi; \
	done; \
	echo "🏃 running stress test with flags:"; \
	echo "    $(STRESS_FLAGS)"; \
	$(STRESSPY) $(STRESS_FLAGS); \
	echo "✅ stress test complete"
