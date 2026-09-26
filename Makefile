.DEFAULT_GOAL := help
.PHONY: help install dev build package test lint fmt check clean

NPM := npm --prefix frontend

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  make %-8s %s\n", $$1, $$2}'

install: ## Install frontend deps (incl. the DBX plugin CLI) and download Go modules
	$(NPM) install
	cd backend && go mod download

dev: ## Dev host on :5190 with the real sidecar, rebuilding on change
	$(NPM) run dev

build: ## Typecheck and bundle the UI into ui/
	$(NPM) run build

package: ## Build UI + sidecar into dist/*.dbxp for this machine
	$(NPM) run package

test: ## Go tests with the race detector (embedded nats-server)
	cd backend && go test -race ./...

lint: ## gofmt check, go vet, UI typecheck
	@out="$$(cd backend && gofmt -l main.go internal)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	cd backend && go vet ./...
	$(NPM) run typecheck

fmt: ## Format Go code
	cd backend && gofmt -w main.go internal

check: lint test build ## Everything CI's frontend and backend jobs run

clean: ## Remove build output
	rm -rf ui dist .dbx-dev
