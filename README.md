# pulumi-lsp
A [LSP server](https://microsoft.github.io/language-server-protocol/) for
writing [Pulumi YAML](https://github.com/pulumi/pulumi-yaml).

---

## Capabilities

### Analysis
- [X] Parse Errors
- [X] Unused variable warnings
- [X] Missing variable warning
- [X] Duplicate key errors

### Hover
- [ ] Resource info on hover
- [ ] Highlight the variable at point across the file

### Completion
- [ ] When entering input properties for a resource
- [ ] When entering a `type` field (after the package)
- [ ] After typing `.` on a resource variable
- [ ] On the return value for invokes

### Actions
- [ ] Rename variable
- [ ] Fill in input properties

## Platforms
The server is theoretically deployable to any editor that supports LSP.

### VS Code

Because [VS Code](https://code.visualstudio.com) is the most common editor, I
used it for initial testing. There is a `.vscode` folder that contains the
necessary plugin description to run the server.

### Emacs

I hope to have a fully functioning mode for
[emacs](https://www.gnu.org/software/emacs/) by end of project, since then I can
actually use the application. I plan to test using
[emacs-lsp](https://emacs-lsp.github.io/lsp-mode/), but other LSP packages
([eglot](https://github.com/joaotavora/eglot)) should also work.
