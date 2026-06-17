BINARY = terraform-provider-gandi

.PHONY: build install vet fmt lint test testacc tidy generate

build: ## Compile the provider binary
	go build -o $(BINARY) .

install: ## Install the provider into GOBIN
	go install .

vet:
	go vet ./...

fmt:
	gofmt -w .

lint:
	golangci-lint run

test: ## Unit tests (no network)
	go test -race -count=1 -cover ./...

# Acceptance tests hit the real Gandi API. Required env:
#   GANDI_PAT, GANDI_TEST_DOMAIN
testacc:
	TF_ACC=1 go test -count=1 -timeout 20m -v ./internal/provider/

tidy:
	go mod tidy

generate: ## Regenerate docs/ from schema + examples
	go generate ./...
