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
	cd client && npm install && npm run compile

clean:
	rm -r ./bin client/node_modules || true
	# Cleaning up emacs
	find . -name "*.elc" -exec rm {} \;
	rm client/emacs/yaml-mode.el || true
	rm -r sdk/yaml/testdata || true

test: get_schemas
	go test ./...

.phony: lint lint-copyright lint-golang
lint:: lint-copyright lint-golang
lint-golang:
	golangci-lint -c .golangci.yml run
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
