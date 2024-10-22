VERSION := 0.8.3
NAMESPACE := github.com/wallarm/api-firewall

.DEFAULT_GOAL := build

build:
	docker build --no-cache --build-arg APIFIREWALL_NAMESPACE=$(NAMESPACE) --build-arg APIFIREWALL_VERSION=$(VERSION) --force-rm -t api-firewall .

lint:
	golangci-lint -v run ./...

tidy:
	go mod tidy
	go mod vendor

test:
	go test ./... -count=1 -race -cover -run '^Test[^W]'
	go test ./cmd/api-firewall/tests/main_dns_test.go

bench:
	GOMAXPROCS=1 go test -v -bench=. -benchtime=1000x -count 5 -benchmem -run BenchmarkWSGraphQL ./cmd/api-firewall/tests
	GOMAXPROCS=4 go test -v -bench=. -benchtime=1000x -count 5 -benchmem -run BenchmarkWSGraphQL ./cmd/api-firewall/tests

genmocks:
	mockgen -source ./internal/platform/proxy/chainpool.go -destination ./internal/platform/proxy/chainpool_mock.go -package proxy
	mockgen -source ./internal/platform/proxy/dnscache.go -destination ./internal/platform/proxy/dnscache_mock.go -package proxy
	mockgen -source ./internal/platform/storage/storage.go -destination ./internal/platform/storage/storage_mock.go -package storage
	mockgen -source ./internal/platform/storage/updater/updater.go -destination ./internal/platform/storage/updater/updater_mock.go -package updater
	mockgen -source ./internal/platform/proxy/ws.go -destination ./internal/platform/proxy/ws_mock.go -package proxy
	mockgen -source ./internal/platform/proxy/wsClient.go -destination ./internal/platform/proxy/wsClient_mock.go -package proxy

update:
	go get -u ./...

fmt:
	gofmt -w ./

vulncheck:
	govulncheck ./...

stop_k6_tests:
	@docker-compose -f resources/test/docker-compose-api-mode.yml down

run_k6_tests: stop_k6_tests
	@docker-compose -f resources/test/docker-compose-api-mode.yml up --build --detach --force-recreate
	docker run --rm -i --network host grafana/k6 run -v - <resources/test/specification/script.js || true
	$(MAKE) stop_k6_tests

.PHONY: lint tidy test fmt build genmocks vulncheck run_k6_tests stop_k6_tests
