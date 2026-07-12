.PHONY: server worker cron docker-build docker-server docker-worker docker-cron docker-migrate migrate-up migrate-down migrate-version release-prepare release-check release-tag release-push release project-branch test test-unit test-integration test-db-integration test-redis-integration test-kafka-integration test-s3-integration test-media-integration test-ci test-verbose test-auth test-mq test-support test-consumption-integration

GO ?= go
DOCKER ?= docker
GIT ?= git
IMAGE_NAME ?= go-backend-template
IMAGE_TAG ?= dev
VERSION ?=
PROJECT ?=

all: test server worker cron

################################################################################
################################################################# Build commands

server:
	$(GO) build -o build/server.exe ./cmd/server

worker:
	$(GO) build -o build/worker.exe ./cmd/worker

cron:
	$(GO) build -o build/cron.exe ./cmd/cron

docker-build: docker-server docker-worker docker-cron docker-migrate

docker-server:
	$(DOCKER) build --build-arg APP=server -t $(IMAGE_NAME)-server:$(IMAGE_TAG) .

docker-worker:
	$(DOCKER) build --build-arg APP=worker -t $(IMAGE_NAME)-worker:$(IMAGE_TAG) .

docker-cron:
	$(DOCKER) build --build-arg APP=cron -t $(IMAGE_NAME)-cron:$(IMAGE_TAG) .

docker-migrate:
	$(DOCKER) build --build-arg APP=migrate -t $(IMAGE_NAME)-migrate:$(IMAGE_TAG) .

##############################################################################
########################################################## git release commands

release-prepare:
	$(GIT) switch main
	$(GIT) pull --ff-only origin main

release-check:
	$(if $(strip $(VERSION)),,$(error VERSION is required, for example: make release VERSION=0.1.0))
	$(if $(filter main,$(shell $(GIT) branch --show-current)),,$(error release must run from main))
	$(if $(strip $(shell $(GIT) status --porcelain)),$(error worktree must be clean before release),)
	$(if $(strip $(shell $(GIT) tag --list template/v$(VERSION))),$(error tag template/v$(VERSION) already exists),)

release-tag: release-check
	$(GIT) tag -a template/v$(VERSION) -m "Template v$(VERSION)"


release-push:
	$(if $(strip $(VERSION)),,$(error VERSION is required, for example: make release-push VERSION=0.1.0))
	$(if $(strip $(shell $(GIT) tag --list template/v$(VERSION))),,$(error tag template/v$(VERSION) does not exist))
	$(GIT) push origin template/v$(VERSION)

release:
	$(MAKE) release-prepare
	$(MAKE) release-tag VERSION=$(VERSION)
	$(MAKE) release-push VERSION=$(VERSION)

project-branch:
	$(if $(strip $(VERSION)),,$(error VERSION is required, for example: make project-branch VERSION=0.1.0 PROJECT=example-service))
	$(if $(strip $(PROJECT)),,$(error PROJECT is required))
	$(GIT) switch -c project/$(PROJECT) template/v$(VERSION)

############################################################################
################################################ Database migration commands

migrate-up:
	$(GO) run ./cmd/migrate -direction up

migrate-down:
	$(GO) run ./cmd/migrate -direction down -allow-destructive

migrate-version:
	$(GO) run ./cmd/migrate -version

############################################################################
############################################################## Tests Commands

test: test-unit

test-unit:
	$(GO) test ./...

test-integration: test-db-integration test-redis-integration test-kafka-integration test-s3-integration test-media-integration

test-db-integration:
	$(GO) test -tags=integration ./internal/infra/database ./internal/repository/...

test-redis-integration:
	$(GO) test -tags=integration ./internal/infra/cache ./pkg/captcha ./pkg/ratelimit

test-kafka-integration:
	$(GO) test -tags=integration ./internal/infra/mq

test-s3-integration:
	$(GO) test -tags=integration ./internal/infra/storage

test-media-integration:
	$(GO) test -tags=integration ./internal/usecase/cms

test-ci: test-unit test-integration

test-verbose:
	$(GO) test -v ./...

test-auth:
	$(GO) test ./internal/usecase/auth

test-mq:
	$(GO) test ./internal/infra/mq

test-support:
	$(GO) test ./internal/usecase/support

test-consumption-integration:
	$(GO) test -v -tags=integration ./internal/repository/consumption

############################################################################
