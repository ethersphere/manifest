GO ?= go
GOLANGCI_LINT ?= golangci-lint
GOLANGCI_LINT_VERSION ?= v1.29.0

.PHONY: all
all: lint vet test-race

.PHONY: lint
lint: linter
	$(GOLANGCI_LINT) run

.PHONY: linter
linter:
	which $(GOLANGCI_LINT) || ( cd /tmp && GO111MODULE=on $(GO) get -u github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) )

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: test-race
test-race:
	$(GO) test -race -v ./...

.PHONY: test
test:
	$(GO) test -v ./...

FORCE:
