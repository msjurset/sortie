VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

generate:
	go generate ./internal/manpage/

build: generate
	go build $(LDFLAGS) -o sortie ./cmd/sortie

run:
	go run ./cmd/sortie

test:
	go test -v ./...

vet:
	go vet ./...

clean:
	rm -rf dist/
	rm -f sortie

release: clean generate test
	@mkdir -p dist
	cp sortie.1 dist/
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/sortie ./cmd/sortie && \
		tar -czf dist/sortie-$(VERSION)-linux-amd64.tar.gz -C dist sortie sortie.1 && rm dist/sortie
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o dist/sortie ./cmd/sortie && \
		tar -czf dist/sortie-$(VERSION)-linux-arm64.tar.gz -C dist sortie sortie.1 && rm dist/sortie
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/sortie ./cmd/sortie && \
		tar -czf dist/sortie-$(VERSION)-darwin-amd64.tar.gz -C dist sortie sortie.1 && rm dist/sortie
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/sortie ./cmd/sortie && \
		tar -czf dist/sortie-$(VERSION)-darwin-arm64.tar.gz -C dist sortie sortie.1 && rm dist/sortie
	rm dist/sortie.1

deploy: build install-man install-completion
	cp sortie ~/.local/bin/

install-man:
	install -d /usr/local/share/man/man1
	install -m 644 sortie.1 /usr/local/share/man/man1/sortie.1

install-completion:
	install -d ~/.oh-my-zsh/custom/completions
	install -m 644 completions/_sortie ~/.oh-my-zsh/custom/completions/_sortie
	@echo "Refreshing zsh completions..."
	@zsh -c 'autoload -U compinit && rm -f ~/.zcompdump* && compinit' 2>/dev/null || true

.PHONY: generate build run test vet clean release deploy install-man install-completion
