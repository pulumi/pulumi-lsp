# CHANGELOG

## v0.2.1 (2022-09-12)

### Improvements

### Bug Fixes

## v0.2.0 (2022-09-12)

### Improvements

- [editors/emacs] Automatic downloads of the LSP Server binary.
  [#31](https://github.com/pulumi/pulumi-lsp/pull/31)

- [editors/vscode] Publish VS Code Extension.
  [#32](https://github.com/pulumi/pulumi-lsp/pull/32)

- [completion] Add support for the `defaultProvider` resource field.
  [#47](https://github.com/pulumi/pulumi-lsp/pull/47)

- [completion] Add support for completing `Fn::*`.
  [#48](https://github.com/pulumi/pulumi-lsp/pull/48)

- [editors/vscode] Warn when Red Hat YAML is also installed.
  [#52](https://github.com/pulumi/pulumi-lsp/pull/52)

- [spec] Account for `Options.Version` when completing fields.
  [#51](https://github.com/pulumi/pulumi-lsp/pull/51)

### Bug Fixes

- [completion] Only complete when the cursor is on the text bieng completed.
  [#51](https://github.com/pulumi/pulumi-lsp/pull/51)

## 0.1.3 (2022-06-10)

- [mission] Pulumi has always strived to make authoring production ready _Infrastructure
  as Code_ as easy as possible. For our other languages, we have tapped into existing
  language tooling. We are excited to build best in class tooling for declarative
  infrastructure as code on top of Pulumi YAML, the first _declarative_ language supported
  by Pulumi. The Pulumi LSP Server will help ensure that, no matter what language you use,
  writing Pulumi Programs always comes with best in class tooling.
