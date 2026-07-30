.PHONY: dev frontend build release test lint

dev: frontend
	@trap 'kill 0' EXIT; \
	go run ./cmd/wardrobe & \
	(cd web && npm run dev) & \
	wait

frontend:
	cd web && npm install && npm run build

build: frontend
	CGO_ENABLED=0 go build -o wardrobe ./cmd/wardrobe

release: frontend
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o wardrobe ./cmd/wardrobe

test: frontend
	go test ./...

lint: frontend
	golangci-lint run
	cd web && npm run lint
