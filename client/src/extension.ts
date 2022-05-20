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
    command: "pulumi-lsp",
  };

  // Options to control the language client
  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { pattern: "**/Pulumi.yaml" },
      { pattern: "**/Main.yaml" },
    ],
  };

  // Create the language client and start the client.
  client = new LanguageClient(
    "pulumi-lsp",
    "Pulumi LSP",
    serverOptions,
    clientOptions,
  );

  client.start();
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }

  return client.stop();
}
