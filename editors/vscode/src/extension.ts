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

let client: PulumiLSPClient;

class PulumiLSPClient extends LanguageClient {
  constructor(serverOptions: ServerOptions) {
    // Options to control the language client
    const clientOptions: LanguageClientOptions = {
      documentSelector: [
        { pattern: "**/Pulumi.yaml" },
        { pattern: "**/Main.yaml" },
      ],
    };

    super("pulumi-lsp", "Pulumi LSP", serverOptions, clientOptions);
  }
}

async function getServer(
  context: ExtensionContext,
): Promise<string | undefined> {
  return Promise.resolve(path.join(process.env["GOBIN"], "pulumi-lsp"));
}

export async function activate(
  context: ExtensionContext,
): Promise<PulumiLSPClient> {
  const serverPath = await getServer(context);
  if (serverPath === undefined) {
    return Promise.reject();
  }
  const serverOptions: ServerOptions = {
    command: path.join(),
  };

  // Create the language client and start the client.
  client = new PulumiLSPClient(serverOptions);
  client.start();

  return client;
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }

  return client.stop();
}
