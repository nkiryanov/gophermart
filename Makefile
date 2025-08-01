.PHONY: test tests
tests: test
test:
	go test -race -timeout=120s -count 1 ./...

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
	cd cmd/gensecret && go build .
	cd cmd/gophermart && go build .

