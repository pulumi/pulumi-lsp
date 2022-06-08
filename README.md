# pulumi-lsp

A [LSP server](https://microsoft.github.io/language-server-protocol/) for
writing [Pulumi YAML](https://github.com/pulumi/pulumi-yaml).

[![Go Reference](https://pkg.go.dev/badge/github.com/pulumi/pulumi-lsp.svg)](https://pkg.go.dev/github.com/pulumi/pulumi-lsp)
[![License](https://img.shields.io/github/license/pulumi/pulumi-lsp)](LICENSE)

---

_Note_: The Pulumi YAML LSP Server is in a public beta. If you have suggestions
for features or find bugs, please open an issue.

## Existing Capabilities

### Warnings and Errors

The Pulumi LSP Server should give contextual warnings when:

1. There is a variable that is never referenced.

The Pulumi LSP Server should give contextual errors when:

1. The file is not a valid YAML document.
2. A reference refers to a variable that does not exist.
3. More then one variable/resource share the same name.

### On Hover

When you hover your mouse over a resources type token, you should observe a
popup that describes the resource. Likewise for the type token of a function.

### Completion

You should get semantic completion when:

1. Typing in a predefined key for Pulumi YAML such as "resources" or "properties".
2. Typing in the name of a resource property or function argument..
3. Entering type tokens for resources or functions.
4. Referencing a structured variable. For example if "cluster" is a
   `eks:Cluster`, then "${cluster.awsPr}" will suggest `awsProvider`.

## Planned Capabilities

### Analysis

- [ ] Duplicate key errors

### Hover

- [ ] Highlight the variable at point across the file

### Completion

- [ ] When entering Pulumi YAML builtin keys.
  - [ ] Functions
  - [x] Top level
  - [x] Resources
- [ ] On the return value for invokes

### Actions

- [ ] Rename variable
- [ ] Fill in input properties

## Setting Up Pulumi LSP

The server is theoretically deployable to any editor that supports LSP.

### VS Code

Because [VS Code](https://code.visualstudio.com) is the most common editor, I used it for
initial testing. Running `make install vscode-client` will install the server on your path
and build a `.vsix` file in `./bin`. Running `code ${./bin/pulumi-lsp-client-*.vsix}` will
install the extension. See [the docs](https://vscode-docs.readthedocs.io/en/stable/extensions/install-extension/)
for details.

### Emacs

`pulumi-yaml.el` provides a major mode for editing Pulumi YAML which should be
auto-invoked on relevant documents. It also associates a LSP server
[emacs-lsp](https://emacs-lsp.github.io/lsp-mode/) which can be launched the
usual way: `M-x lsp`. Run `make install emacs-client` to install the server in
`$GOPATH/bin`. A `pulumi-yaml.elc` fill will be generated in `./bin`.
