.PHONY: test test-race lint

test:
	@echo "Running tests..."
	@go test ./... -v

test-race:
	@echo "Running tests with race detector..."
	@go test ./... -v -race

lint:
	# Мы добавим линтер позже, пока это заглушка
	@echo "Linter not configured yet."