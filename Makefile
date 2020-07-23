GO ?= go
GOLANGCI_LINT ?= golangci-lint
GOLANGCI_LINT_VERSION ?= v1.30.0

.PHONY: all
all: lint vet test-race

.PHONY: lint
lint: linter
	$(GOLANGCI_LINT) run

.PHONY: linter
linter:
	which $(GOLANGCI_LINT) || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$($(GO) env GOPATH)/bin $(GOLANGCI_LINT_VERSION)

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
