BINARY_NAME := "spotter"
BINARY_PATH := "./" + BINARY_NAME
CMD_PATH := "./cmd/" + BINARY_NAME
VERSION := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
BUILD_FLAGS := "-ldflags \"-X main.version=" + VERSION + "\""
AIR_VERSION := "v1.67.4"

build: web-build
    @echo "Building {{BINARY_NAME}}..."
    @go build {{BUILD_FLAGS}} -o {{BINARY_PATH}} {{CMD_PATH}}
    @echo "Build complete: {{BINARY_PATH}}"

# Install frontend dependencies (no-op if there's no web/ directory)
web-install:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -d web ]; then
        cd web && npm install
    fi

# Build the frontend into web/dist (no-op if there's no web/ directory)
web-build: web-install
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -d web ]; then
        cd web && npm run build
    fi

# Run the frontend dev server (proxies /api and /health to :8080)
web-dev:
    @cd web && npm run dev

run: build
    @echo "Running {{BINARY_NAME}}..."
    @{{BINARY_PATH}}

# Download a Protomaps PMTiles extract for the region into data/ (set BASEMAP_PMTILES=data/basemap.pmtiles to use it).
# Requires: go install github.com/protomaps/go-pmtiles@latest
basemap bbox="15,-36,34,-21" maxzoom="12":
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p data
    build=$(date -u -v-1d +%Y%m%d 2>/dev/null || date -u -d yesterday +%Y%m%d)
    go-pmtiles extract "https://build.protomaps.com/${build}.pmtiles" data/basemap.pmtiles --bbox={{bbox}} --maxzoom={{maxzoom}}
    ls -lh data/basemap.pmtiles

# Run Go on :8080 with hot reload (air) and Vite on :5173, with vars from .env; Ctrl-C stops both
dev: web-install
    #!/usr/bin/env bash
    set -euo pipefail
    trap 'kill 0' EXIT INT TERM
    set -a; [ -f .env ] && source .env; set +a
    go run github.com/air-verse/air@{{AIR_VERSION}} &
    (cd web && npm run dev) &
    wait

docker:
    @docker build -t {{BINARY_NAME}} .

docker-run: docker
    @docker run --rm -p 8080:8080 {{BINARY_NAME}}

install: build
    @echo "Installing {{BINARY_NAME}} to /usr/local/bin..."
    @sudo cp {{BINARY_PATH}} /usr/local/bin/{{BINARY_NAME}}
    @echo "Installation complete: /usr/local/bin/{{BINARY_NAME}}"

test:
    @echo "Running tests..."
    @go test ./...

test-verbose:
    @echo "Running tests (verbose)..."
    @go test -v ./...

test-coverage:
    @echo "Running tests with coverage..."
    @go test -v -coverprofile=coverage.out ./...
    @go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report generated: coverage.html"

fmt:
    @echo "Formatting code..."
    @go fmt ./...
    @echo "Format complete"

lint:
    @echo "Running linter..."
    @golangci-lint run ./... 2>/dev/null || echo "golangci-lint not installed. Install with: brew install golangci-lint"

clean:
    @echo "Cleaning build artifacts..."
    @rm -f {{BINARY_PATH}}
    @rm -f coverage.out coverage.html
    @go clean
    @echo "Clean complete"

# Show current version and what the next version would be based on conventional commits
# Requires: go install github.com/caarlos0/svu/v3@v3.4.0
version:
    @echo "Current: $(svu current)"
    @echo "Next:    $(svu next --v0)"

# Create a local git tag for the next version derived from conventional commits
tag:
    #!/usr/bin/env bash
    set -euo pipefail
    NEXT=$(svu next --v0)
    CURRENT=$(svu current)
    if [ "$NEXT" = "$CURRENT" ]; then
        echo "No commits requiring a version bump since $CURRENT"
        exit 0
    fi
    git tag "$NEXT"
    echo "Tagged $NEXT"

