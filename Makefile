-include .env
export

DB_STRING = $(DB_USER):$(DB_PASSWORD)@tcp($(DB_HOST):$(DB_PORT))/$(DB_NAME)?parseTime=true
GOOSE     = goose -dir migrations mysql "$(DB_STRING)"

.PHONY: run migrate-create migrate-up migrate-down migrate-redo migrate-status

run:
	go run ./cmd/main.go

migrate-create:
	goose -dir migrations -s create $(name) sql

migrate-up:
	$(GOOSE) up

migrate-down:
	$(GOOSE) down

migrate-redo:
	$(GOOSE) redo

migrate-status:
	$(GOOSE) status