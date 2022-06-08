GO        := go
EMACS     := emacs
NODE      := node
SHELL     := bash

CONCURRENCY ?= 10

default: install server

build: server client

server:
	mkdir -p ./bin
	${GO} build -o ./bin -p ${CONCURRENCY} ./cmd/...

install: server
	${GO} install ./cmd/...

client: emacs-client vscode-client

emacs-client: client/emacs/yaml-mode.el client/emacs/pulumi-yaml.elc
	mkdir -p ./bin
	mv client/emacs/pulumi-yaml.elc bin/
client/emacs/yaml-mode.el:
	curl https://raw.githubusercontent.com/yoshiki/yaml-mode/master/yaml-mode.el > client/emacs/yaml-mode.el

vscode-build:
	cd client && npm install && npm run compile
vscode-client: vscode-build
	mkdir -p ./bin
	cp LICENSE client/LICENSE
	cd client && npm exec vsce -- package --out ../bin/

clean:
	@rm -rf ./bin client/node_modules
	@rm -f client/emacs/{yaml-mode.el,*.elc}
	@rm -rf sdk/yaml/testdata
	@rm -f client/LICENSE
	@rm -f client/*.vsix

test: get_schemas
	go test ./...

.phony: lint lint-copyright lint-golang
lint:: lint-copyright lint-golang
lint-golang:
	golangci-lint --timeout 5m -c .golangci.yml run
lint-copyright:
	pulumictl copyright

%.elc: %.el
	cd client/emacs && $(EMACS) -Q --batch -L $$(pwd) -f batch-byte-compile $(notdir $<)

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
