.PHONY: all test test-cover test-race bench bench-stress lint clean docs

# Default target
all: test lint

# Run all unit tests
test:
	go test ./... -count=1 -timeout=60s

# Run tests with coverage report
test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic -count=1
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Run tests with race detector
test-race:
	go test ./... -race -count=1 -timeout=120s

# Run benchmarks
bench:
	go test ./... -bench=. -benchmem -count=5 -timeout=300s | tee bench.txt

# Run stress tests
bench-stress:
	go test ./... -bench=. -benchmem -benchtime=10s -count=10 -timeout=600s | tee bench_stress.txt

# Run linter
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -f coverage.out coverage.html bench.txt bench_stress.txt

# Generate API documentation
docs:
	@mkdir -p docs/api
	@for pkg in ecad ecfr ecmd ecee eni; do \
		echo "Generating docs/$$pkg.txt ..."; \
		go doc -all ./$$pkg > docs/api/$$pkg.txt; \
	done
	@for pkg in internal/marshalling internal/sim internal/link/udp; do \
		name=$$(basename $$pkg); \
		echo "Generating docs/$$name.txt ..."; \
		go doc -all ./$$pkg > docs/api/$$name.txt; \
	done
	@echo "Documentation generated in docs/api/"