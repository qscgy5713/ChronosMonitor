.PHONY: build build-frontend run dev-backend dev-frontend test clean

BINARY := chronosmonitor

# Builds the Vue dashboard into internal/webui/dist, then compiles a single
# Go binary with that dashboard embedded via go:embed.
build: build-frontend
	go build -o $(BINARY) ./cmd/server

test:
	go test ./... -race

build-frontend:
	cd web && npm install && npm run build
	# vite's emptyOutDir wipes internal/webui/dist/.gitkeep on every build;
	# restore its committed content (a plain `touch` would leave it empty,
	# which still shows as modified in `git status`).
	git checkout -- internal/webui/dist/.gitkeep 2>/dev/null || true

run: build
	./$(BINARY)

# Two-process local dev setup: Go API on :8080, Vite dev server on :5173
# (proxies /api to :8080 and hot-reloads the frontend).
dev-backend:
	go run ./cmd/server

dev-frontend:
	cd web && npm run dev

clean:
	rm -f $(BINARY)
	rm -rf internal/webui/dist/*
	git checkout -- internal/webui/dist/.gitkeep 2>/dev/null || true
