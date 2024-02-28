VERSION := 0.6.16

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

bench:
	GOMAXPROCS=1 go test -v -bench=. -benchtime=1000x -count 5 -benchmem -run BenchmarkWSGraphQL ./cmd/api-firewall/tests
	GOMAXPROCS=4 go test -v -bench=. -benchtime=1000x -count 5 -benchmem -run BenchmarkWSGraphQL ./cmd/api-firewall/tests

genmocks:
	mockgen -source ./internal/platform/proxy/chainpool.go -destination ./internal/platform/proxy/httppool_mock.go -package proxy
	mockgen -source ./internal/platform/database/database.go -destination ./internal/platform/database/database_mock.go -package database
	mockgen -source ./cmd/api-firewall/internal/updater/updater.go -destination ./cmd/api-firewall/internal/updater/updater_mock.go -package updater
	mockgen -source ./internal/platform/proxy/ws.go -destination ./internal/platform/proxy/ws_mock.go -package proxy
	mockgen -source ./internal/platform/proxy/wsClient.go -destination ./internal/platform/proxy/wsClient_mock.go -package proxy

update:
	go get -u ./...

fmt:
	gofmt -w ./

vulncheck:
	govulncheck ./...

.PHONY: lint tidy test fmt build genmocks vulncheck