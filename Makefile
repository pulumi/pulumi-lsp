GO        := go
EMACS     := emacs
NODE      := node
SHELL     := bash

CONCURRENCY ?= 10

default: install server

build: server client

VERSION      := $(shell pulumictl get version)
LINK_VERSION := -ldflags "-X github.com/pulumi/pulumi-lsp/sdk/version.Version=${VERSION}"

server:
	mkdir -p ./bin
	${GO} build ${LINK_VERSION} -o ./bin -p ${CONCURRENCY} ./cmd/...

install: server
	${GO} install ${LINK_VERSION} ./cmd/...

client: emacs-client vscode-client

emacs-client: editors/emacs/pulumi-yaml.elc
	mkdir -p ./bin
	cp editors/emacs/pulumi-yaml.elc bin/

vscode-build:
	cd editors/vscode && npm install && npm run test-compile && npm run esbuild

# Because vscode bundles embed the LSP server, we need to build the server first.
vscode-client: vscode-build server
	cp LICENSE editors/vscode/LICENSE
	cp bin/pulumi-lsp editors/vscode/
	cd editors/vscode && npm exec vsce -- package --out ../../bin/

clean:
	@rm -rf ./bin editors/node_modules
	@rm -f editors/emacs/{yaml-mode.el,*.elc}
	@rm -rf sdk/yaml/testdata
	@rm -f editors/vscode/LICENSE
	@rm -f editors/vscode/*.vsix
	@rm -f editors/vscode/pulumi-lsp
	@rm -rf editors/emacs/bin

test: get_schemas
	go test ./...

.phony: lint lint-copyright lint-golang
lint:: lint-copyright lint-golang
lint-golang:
	golangci-lint --timeout 5m -c .golangci.yml run
lint-copyright:
	pulumictl copyright

%.elc: %.el
	mkdir -p editors/emacs/bin
	cd editors/emacs && $(EMACS) -Q --batch --eval "(progn (setq package-user-dir \"$$(pwd)/bin\" \
                                                          package-archives '((\"melpa\" . \"https://melpa.org/packages/\") \
                                                                           (\"gnu\" . \"https://elpa.gnu.org/packages/\"))) \
												    (package-initialize) \
                                                    (package-install 'yaml-mode) (package-install 'lsp-mode))" -f batch-byte-compile $(notdir $<)

SCHEMA_PATH := sdk/yaml/testdata
name=$(subst schema-,,$(word 1,$(subst !, ,$@)))
version=$(word 2,$(subst !, ,$@))
schema-%:
	@echo "Ensuring $@ => ${name}, ${version}"
	mkdir -p sdk/yaml/testdata
	@[ -f ${SCHEMA_PATH}/${name}.json ] || \
		curl "https://raw.githubusercontent.com/pulumi/pulumi-${name}/v${version}/provider/cmd/pulumi-resource-${name}/schema.json" \
	 	| jq '.version = "${version}"' >  ${SCHEMA_PATH}/${name}.json
	@FOUND="$$(jq -r '.version' ${SCHEMA_PATH}/${name}.json)" &&                           \
		if ! [ "$$FOUND" = "${version}" ]; then									           \
			echo "${name} required version ${version} but found existing version $$FOUND"; \
			exit 1;																		   \
		fi
get_schemas: schema-aws!4.26.0       \
			 schema-eks!0.37.1       \
             schema-kubernetes!3.7.2
