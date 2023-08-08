VERSION := 0.6.12

.DEFAULT_GOAL := build

build:
	docker build --no-cache --build-arg APIFIREWALL_VERSION=$(VERSION) --force-rm -t api-firewall .

lint:
	golangci-lint -v run ./...

tidy:
	go mod tidy
	go mod vendor

test:
	go test ./... -count=1 -race -cover

genmocks:
	mockgen -source ./internal/platform/proxy/chainpool.go -destination ./internal/platform/proxy/httppool_mock.go -package proxy
	mockgen -source ./internal/platform/database/database.go -destination ./internal/platform/database/database_mock.go -package database
	mockgen -source ./cmd/api-firewall/internal/updater/updater.go -destination ./cmd/api-firewall/internal/updater/updater_mock.go -package updater

update:
	go get -u ./...

fmt:
	gofmt -w ./

vulncheck:
	govulncheck ./...

.PHONY: lint tidy test fmt build genmocks vulncheck