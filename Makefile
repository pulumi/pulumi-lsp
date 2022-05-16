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

emacs-client: client/emacs/yaml-mode.el client/emacs/pulumi-yaml.elc
client/emacs/yaml-mode.el:
	curl https://raw.githubusercontent.com/yoshiki/yaml-mode/master/yaml-mode.el > client/emacs/yaml-mode.el

vscode-client:
	cd client/vscode && npm install && npm run compile

clean:
	rm -r ./bin client/node_modules || true
	# Cleaning up emacs
	find . -name "*.elc" -exec rm {} \;
	rm client/emacs/yaml-mode.el

test:
	go test ./...

.phony: lint lint-copyright lint-golang
lint:: lint-copyright lint-golang
lint-golang:
	golangci-lint -c .golangci.yml run
lint-copyright:
	pulumictl copyright

%.elc: %.el
	cd client/emacs && $(EMACS) -Q --batch -L $$(pwd) -f batch-byte-compile $(notdir $<)
