VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOEXE := $(shell go env GOEXE)

.PHONY: build vet fmt test hooks cross clean npm-publish npm-build

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/ctfcode$(GOEXE) ./cmd/ctfcode

vet:
	go vet ./...

fmt:
	gofmt -w .

test:
	go test ./...

hooks:
	@git config core.hooksPath .githooks
	@echo "installed: core.hooksPath -> .githooks (pre-push runs go vet)"

cross:
	@mkdir -p dist
	@for p in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		echo "build $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o dist/ctfcode-$$os-$$arch$$ext ./cmd/ctfcode; \
	done

npm-build: build
	./scripts/build-npm.sh

npm-publish: npm-build
	@echo "=== 发布平台包 ==="
	@for pkg in npm/platforms/*/; do \
		echo "发布 $$pkg"; \
		(cd "$$pkg" && npm publish --access public) || true; \
	done
	@echo "=== 发布主包 ==="
	cd npm/deepseek-ctfcode && npm publish --access public
	@echo "=== 完成 ==="

clean:
	rm -rf bin dist npm/platforms/*/bin/* npm/deepseek-ctfcode/bin/ctfcode
