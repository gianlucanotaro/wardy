.PHONY: dev build build-web build-go release test lint clean

dev:
	@echo "Start backend: make serve"
	@echo "Start frontend: cd web && npm run dev"

serve:
	go run ./cmd/wardrobe serve

build-web:
	cd web && npm run build

build-go: build-web
	go build -o wardrobe ./cmd/wardrobe

build: build-go

release: build-web
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o wardrobe-linux-arm64 ./cmd/wardrobe

test:
	go test ./...
	cd web && npx tsc --noEmit

lint:
	cd web && npm run lint

clean:
	rm -f wardrobe wardrobe-linux-arm64
	rm -rf web/dist
