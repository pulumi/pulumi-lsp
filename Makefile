GO        := go
EMACS     := emacs
NODE      := node
SHELL     := bash

default: install

build: bin/pulumi-lsp client

COMMIT       := $(shell git rev-parse --short HEAD)
VERSION      := $(shell git describe --tags --match 'v*.*.*' --dirty=${COMMIT})
LINK_VERSION := -ldflags "-X github.com/pulumi/pulumi-lsp/sdk/version.Version=${VERSION}"

_ := $(shell mkdir -p bin)
_ := $(shell go build -o bin/helpmakego github.com/iwahbe/helpmakego)

server: bin/pulumi-lsp

bin/pulumi-lsp: $(shell bin/helpmakego cmd/pulumi-lsp)
	${GO} build ${LINK_VERSION} -o $@ github.com/pulumi/pulumi-lsp/cmd/pulumi-lsp

.PHONY: install
install: bin/pulumi-lsp
	install $< $(or $(shell ${GO} env GOBIN),$(shell ${GO} env GOPATH)/bin)

client: emacs-client vscode-client

emacs-client: editors/emacs/pulumi-yaml.elc
	mkdir -p ./bin
	cp editors/emacs/pulumi-yaml.elc bin/

vscode-build:
	cd editors/vscode && npm install && npm run test-compile && npm run esbuild

# Because vscode bundles embed the LSP server, we need to build the server first.
vscode-client: vscode-build bin/pulumi-lsp
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

.PHONY: lint lint-copyright lint-golang
lint:: lint-copyright lint-golang
lint-golang:
	golangci-lint run --timeout 5m --config .golangci.yml
lint-copyright:
	pulumictl copyright

%.elc: %.el
	mkdir -p editors/emacs/bin
	cd editors/emacs && $(EMACS) -Q --batch --eval "(progn (setq package-user-dir \"$$(pwd)/bin\" \
                                                          package-archives '((\"melpa\" . \"https://melpa.org/packages/\") \
                                                                           (\"gnu\" . \"https://elpa.gnu.org/packages/\"))) \
												    (package-initialize) \
                                                    (package-install 'yaml-mode) (package-install 'lsp-mode))" -f batch-byte-compile $(notdir $<)


# Awsx has a different directory structure, so it needs to be special cased.
schema-awsx!1.0.0-beta.5: url = "https://raw.githubusercontent.com/pulumi/pulumi-awsx/v${version}/awsx/schema.json"

SCHEMA_PATH := sdk/yaml/testdata
# We replace the '!' with a space, then take the first word
# schema-pkg!x.y.z => schema-pkg
# We then replace 'schema-' with nothing, giving only the package name.
# schema-pkg => pkg
# Recall that `$@` is the target make is trying to build, in our case schema-pkg!x.y.z
name=$(subst schema-,,$(word 1,$(subst !, ,$@)))
# Here we take the second word, just the version
version=$(word 2,$(subst !, ,$@))
schema-%: url ?= "https://raw.githubusercontent.com/pulumi/pulumi-${name}/v${version}/provider/cmd/pulumi-resource-${name}/schema.json"
schema-%:
	@mkdir -p ${SCHEMA_PATH}
	@echo "Ensuring schema ${name}, ${version}"
# Download the package from github, then stamp in the correct version.
	@[ -f ${SCHEMA_PATH}/${name}-${version}.json ] || \
		curl ${url} \
		| jq '.version = "${version}"' >  ${SCHEMA_PATH}/${name}-${version}.json
# Confirm that the correct version is present. If not, error out.
	@FOUND="$$(jq -r '.version' ${SCHEMA_PATH}/${name}-${version}.json)" &&        \
		if ! [ "$$FOUND" = "${version}" ]; then									           \
			echo "${name} required version ${version} but found existing version $$FOUND"; \
			exit 1;																		   \
		fi

# This needs to mirror the list found in pulumi/pulumi
get_schemas: \
			schema-aws!4.26.0           \
			schema-aws!4.36.0           \
			schema-aws!4.37.1           \
			schema-aws!5.4.0            \
			schema-aws!5.16.2           \
			schema-azure-native!1.28.0  \
			schema-azure-native!1.29.0  \
			schema-azure-native!1.56.0  \
			schema-azure!4.18.0         \
			schema-kubernetes!3.0.0     \
			schema-kubernetes!3.7.0     \
			schema-kubernetes!3.7.2     \
			schema-random!4.2.0         \
			schema-random!4.3.1         \
			schema-eks!0.37.1           \
			schema-eks!0.40.0           \
			schema-docker!3.1.0         \
			schema-awsx!1.0.0-beta.5    \
			schema-aws-native!0.13.0    \
			schema-google-native!0.18.2
