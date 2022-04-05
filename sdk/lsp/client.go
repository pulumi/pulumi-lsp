// Copyright 2022, Pulumi Corporation.  All rights reserved.

package lsp

import (
	"context"
	"fmt"

	"go.lsp.dev/protocol"
)

// Client represents a LSP client to a Server. It is passed to all methods and
// is used to post non-requested responses to the server.
type Client struct {
	inner protocol.Client
	ctx   context.Context
}

func (c *Client) Progress(params *protocol.ProgressParams) error {
	return c.inner.Progress(c.ctx, params)
}
func (c *Client) WorkDoneProgressCreate(params *protocol.WorkDoneProgressCreateParams) error {
	return c.inner.WorkDoneProgressCreate(c.ctx, params)
}

// Publish diagnostic messages to the user. This is how errors and warnings are
// displayed. Every time diagnostics are published, the complete list of current
// diagnostics must be published. Diagnostics persist until a new set of
// diagnostics are published.
//
// To clear all diagnostics, publish an empty list of diagnostics.
func (c *Client) PublishDiagnostics(params *protocol.PublishDiagnosticsParams) error {
	return c.inner.PublishDiagnostics(c.ctx, params)
}
func (c *Client) ShowMessage(params *protocol.ShowMessageParams) error {
	return c.inner.ShowMessage(c.ctx, params)
}
func (c *Client) ShowMessageRequest(params *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	return c.inner.ShowMessageRequest(c.ctx, params)
}
func (c *Client) Telemetry(params interface{}) error {
	return c.inner.Telemetry(c.ctx, params)
}
func (c *Client) RegisterCapability(params *protocol.RegistrationParams) error {
	return c.inner.RegisterCapability(c.ctx, params)
}
func (c *Client) UnregisterCapability(params *protocol.UnregistrationParams) error {
	return c.inner.UnregisterCapability(c.ctx, params)
}
func (c *Client) ApplyEdit(params *protocol.ApplyWorkspaceEditParams) (bool, error) {
	return c.inner.ApplyEdit(c.ctx, params)
}
func (c *Client) Configuration(params *protocol.ConfigurationParams) ([]interface{}, error) {
	return c.inner.Configuration(c.ctx, params)
}
func (c *Client) WorkspaceFolders() ([]protocol.WorkspaceFolder, error) {
	return c.inner.WorkspaceFolders(c.ctx)
}

func (c *Client) logMessage(level protocol.MessageType, txt string) error {
	err := c.inner.LogMessage(c.ctx, &protocol.LogMessageParams{
		Message: txt,
		Type:    level,
	})

	if err != nil {
		err = c.inner.LogMessage(c.ctx, &protocol.LogMessageParams{
			Message: fmt.Sprintf(`Failed to send message "%s" at level %s: %s`,
				txt, level.String(), err.Error()),
			Type: protocol.MessageTypeError,
		})
	}
	return err
}

func (c *Client) LogErrorf(msg string, args ...interface{}) error {
	return c.logMessage(protocol.MessageTypeError, fmt.Sprintf(msg, args...))
}

func (c *Client) LogWarningf(msg string, args ...interface{}) error {
	return c.logMessage(protocol.MessageTypeWarning, fmt.Sprintf(msg, args...))
}

func (c *Client) LogInfof(msg string, args ...interface{}) error {
	return c.logMessage(protocol.MessageTypeInfo, fmt.Sprintf(msg, args...))
}

func (c *Client) LogDebugf(msg string, args ...interface{}) error {
	return c.logMessage(protocol.MessageTypeLog, fmt.Sprintf(msg, args...))
}

// Retrieve the Context of method that Client was passed with.
func (c *Client) Context() context.Context {
	return c.ctx
}
