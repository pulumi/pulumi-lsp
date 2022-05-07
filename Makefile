GO        := go

CONCURRENCY ?= 10

default: build install

build: vscode-client lsp-server

lsp-server:
	mkdir -p ./bin
	${GO} build -o ./bin -p ${CONCURRENCY} ./cmd/...

vscode-client:
	cd client && npm install && npm run compile

install: build
	${GO} install ./cmd/...

clean:
	rm -r ./bin client/node_modules

test:
	go test ./...

.phony: lint
lint:: lint-copyright lint-golang
lint-golang:
	golangci-lint -c .golangci.yml run
lint-copyright:
	pulumictl copyright
