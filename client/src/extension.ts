// Copyright 2022, Pulumi Corporation.  All rights reserved.

"use strict";

import * as path from "path";
import * as process from "process";

import { ExtensionContext, window } from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from "vscode-languageclient/node";

let client: LanguageClient;

export function activate(context: ExtensionContext) {
  const serverOptions: ServerOptions = {
    command: path.join(process.env["GOBIN"], "pulumi-lsp"),
  };

  // Options to control the language client
  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { pattern: "**/Pulumi.yaml" },
      { pattern: "**/Main.yaml" },
    ],
  };

  // Create the language client and start the client.
  const disposable = new LanguageClient(
    "Pulumi LSP",
    serverOptions,
    clientOptions,
  ).start();

  // Push the disposable to the context's subscriptions so that the
  // client can be deactivated on extension deactivation
  context.subscriptions.push(disposable);
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
