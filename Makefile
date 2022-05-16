GO        := go
EMACS     := emacs
NODE      := node

CONCURRENCY ?= 10

default: install server

build: server client

server:
	mkdir -p ./bin
	${GO} build -o ./bin -p ${CONCURRENCY} ./cmd/...

install: build
	${GO} install ./cmd/...

client: emacs-client vscode-client

emacs-client: client/pulumi-yaml.elc

vscode-client:
	cd client && npm install && npm run compile

clean:
	rm -r ./bin client/node_modules || true
	rm client/*.elc || true

test:
	go test ./...

.phony: lint lint-copyright lint-golang
lint:: lint-copyright lint-golang
lint-golang:
	golangci-lint -c .golangci.yml run
lint-copyright:
	pulumictl copyright

%.elc: %.el
	$(EMACS) -Q --batch -L . -f batch-byte-compile $<
