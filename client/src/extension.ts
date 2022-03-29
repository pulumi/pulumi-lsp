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
    command: path.join(process.env["GOBIN"], "pulumi-yaml-language-server"),
  };

  // Options to control the language client
  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { pattern: "**/Pulumi.yaml" },
      { pattern: "**/Main.yaml" },
    ],
  };

  window.showInformationMessage("I have loaded something!");

  // Create the language client and start the client.
  const disposable = new LanguageClient(
    "Language Server Example",
    serverOptions,
    clientOptions
  ).start();

  // Push the disposable to the context's subscriptions so that the
  // client can be deactivated on extension deactivation
  context.subscriptions.push(disposable);
}
