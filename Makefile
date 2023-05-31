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
	go test ./... -count=1

genmocks:
	mockgen -source ./internal/platform/proxy/chainpool.go -destination ./internal/platform/proxy/httppool_mock.go -package proxy
	mockgen -source ./internal/platform/shadowAPI/shadowAPI.go -destination ./internal/platform/shadowAPI/shadowAPI_mock.go -package shadowAPI
	mockgen -source ./internal/platform/database/database.go -destination ./internal/platform/database/database_mock.go -package database

update:
	go get -u ./...

fmt:
	gofmt -w ./

vulncheck:
	govulncheck ./...

.PHONY: lint tidy test fmt build genmocks vulncheck