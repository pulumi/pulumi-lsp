// Copyright 2022, Pulumi Corporation.  All rights reserved.

"use strict";

import * as process from "process";
import * as fs from "fs";

import * as vscode from "vscode";
import * as lc from "vscode-languageclient/node";

let client: PulumiLSPClient;

class PulumiLSPClient extends lc.LanguageClient {
  constructor(serverOptions: lc.ServerOptions) {
    // Options to control the language client
    const clientOptions: lc.LanguageClientOptions = {
      documentSelector: [
        { pattern: "**/Pulumi.yaml" },
        { pattern: "**/Main.yaml" },
      ],
    };

    super("pulumi-lsp", "Pulumi LSP", serverOptions, clientOptions);
  }
}

class Config {
  readonly rootPath: string = "pulumi-lsp";
  constructor() {
    vscode.workspace.onDidChangeConfiguration(this.onDidChangeConfiguration);
  }

  serverPath() {
    return vscode.workspace.getConfiguration(this.rootPath).get<string | null>(
      "server.path",
    );
  }

  onDidChangeConfiguration(
    event: vscode.ConfigurationChangeEvent,
  ) {
    if (event.affectsConfiguration(this.rootPath)) {
      outputChannel().replace(
        "Restart the Pulumi LSP extension for configuration changes to take effect.",
      );
    }
  }
}

let OUTPUT_CHANNEL: vscode.OutputChannel | null = null;
export function outputChannel() {
  if (!OUTPUT_CHANNEL) {
    OUTPUT_CHANNEL = vscode.window.createOutputChannel(
      "Pulumi LSP Server",
    );
  }
  return OUTPUT_CHANNEL;
}

async function getServer(
  context: vscode.ExtensionContext,
  config: Config,
): Promise<string | undefined> {
  const explicitPath = config.serverPath();
  if (explicitPath) {
    if (fs.existsSync(explicitPath)) {
      outputChannel().replace(
        `Launching server from explicitly provided path: ${explicitPath}`,
      );
      return Promise.resolve(explicitPath);
    }
    const msg =
      `${config.rootPath}.server.path specified a path, but the file ${explicitPath} does not exist.`;
    outputChannel().replace(msg);
    outputChannel().show();
    return Promise.reject(msg);
  }

  const ext = process.platform === "win32" ? ".exe" : "";
  const bundled = vscode.Uri.joinPath(
    context.extensionUri,
    `pulumi-lsp${ext}`,
  );
  const bundledExists = await vscode.workspace.fs.stat(bundled).then(
    () => true,
    () => false,
  );

  if (bundledExists) {
    const path = bundled.fsPath;
    outputChannel().replace(`Launching built-in Pulumi LSP Server`);
    return Promise.resolve(path);
  }

  outputChannel().replace(`Could not find a bundled Pulumi LSP Server.
Please specify a pulumi-lsp binary via settings.json at the "${config.rootPath}.server.path" key.
If you think this is an error, please report it at https://github.com/pulumi/pulumi-lsp/issues.`);
  outputChannel().show();

  return Promise.reject("No binary found");
}

export async function activate(
  context: vscode.ExtensionContext,
): Promise<PulumiLSPClient> {
  const config = new Config();
  const serverPath = await getServer(context, config);
  if (serverPath === undefined) {
    outputChannel().append("\nFailed to find LSP executable");
    return Promise.reject();
  }
  const serverOptions: lc.ServerOptions = {
    command: serverPath,
  };

  // Create the language client and start the client.
  client = new PulumiLSPClient(serverOptions);
  client.start();

  // Ensure that we are not running at the same time as 'Red Hat YAML' without warning the
  // user.
  const shouldCheck = vscode.workspace.getConfiguration("pulumi-lsp").get(
    "detectExtensionConflicts",
  );
  if (shouldCheck) {
    let isDisplayed = false;
    const interval = setInterval(function () {
      const rhYaml = vscode.extensions.getExtension("redhat.vscode-yaml");
      if (rhYaml && rhYaml.isActive && !isDisplayed) {
        isDisplayed = true;
        vscode.window
          .showWarningMessage(
            "You have both the Red Hat YAML extension and " +
              "Pulumi YAML extension enabled. Red Hat YAML " +
              "conflict with Pulumi YAML code completion.",
            "Disable Red Hat YAML",
            "Never show this warning",
          )
          .then((selection) => {
            if (selection == "Disable Red Hat YAML") {
              const promise = vscode.commands.executeCommand(
                "workbench.extensions.uninstallExtension",
                "redhat.vscode-yaml",
              );
              vscode.window.showInformationMessage(
                "Red Hat YAML has been uninstalled in this workspace. " +
                  "You will need to reload VSCode for this to take effect.",
                "Restart Now",
                "Restart Later",
              ).then((selection) => {
                isDisplayed = false;
                if (selection == "Restart Now") {
                  promise.then(() =>
                    vscode.commands.executeCommand(
                      "workbench.action.reloadWindow",
                    )
                  );
                }
              });
            } else if (selection == "Never show this warning") {
              vscode.workspace.getConfiguration("pulumi-lsp").update(
                "detectExtensionConflicts",
                false,
                vscode.ConfigurationTarget.Global,
              );
              clearInterval(interval);
              isDisplayed = false;
            } else {
              isDisplayed = false;
            }
          });
      }
    }, 5000);
  }

  return client;
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }

  return client.stop();
}
