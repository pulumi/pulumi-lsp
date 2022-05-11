# pulumi-lsp

A [LSP server](https://microsoft.github.io/language-server-protocol/) for
writing [Pulumi YAML](https://github.com/pulumi/pulumi-yaml).

---

## Capabilities

### Analysis

- [x] Parse Errors
- [x] Unused variable warnings
- [x] Missing variable warning
- [x] Duplicate key errors

### Hover

- [x] Resource info on hover
- [x] Invoke info on hover
- [ ] Highlight the variable at point across the file

### Completion

- [x] When entering Pulumi YAML builtin keys.
- [x] When entering input properties for a resource
- [x] When entering a `type` field
- [x] After typing `.` on a resource variable
- [ ] On the return value for invokes

### Actions

- [ ] Rename variable
- [ ] Fill in input properties

## Platforms

The server is theoretically deployable to any editor that supports LSP.

### VS Code

Because [VS Code](https://code.visualstudio.com) is the most common editor, I
used it for initial testing. So far, I launch the VS Code plugin by opening
`client/src/extension.ts` in VS Code and hitting F5.

### Emacs

`pulumi-yaml.el` provides a major mode for editing Pulumi YAML which should be
auto-invoked on relevant documents. It also associates a LSP server
[emacs-lsp](https://emacs-lsp.github.io/lsp-mode/) which can be launched the
usual way: `M-x lsp`.
