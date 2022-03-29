GO          := go
CONCURRENCY ?= 10

build: vscode-client lsp-server

lsp-server:
	mkdir -p ./bin
	${GO} build -o ./bin -p ${CONCURRENCY} ./cmd/...

vscode-client:
	cd client && npm run compile

install: build
	${GO} install ./cmd/...

clean:
	rm -r ./bin client/node_modules
