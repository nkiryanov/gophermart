.PHONY: test tests
tests: test
test:
	TZ=UTC go test -race -timeout=60s -count 1 ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint:
	gofmt -l .
	golangci-lint run ./...

.PHONE: generate
generate:
	go generate ./...

.PHONE: build
build:
	cd cmd/gophermart && go build .
