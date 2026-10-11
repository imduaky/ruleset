.PHONY: lint
lint: fmt
	@golangci-lint run --timeout=30s ./...

.PHONY: fmt
fmt:
	@golangci-lint fmt ./...

.PHONY: test
test: lint
	@go test ./...