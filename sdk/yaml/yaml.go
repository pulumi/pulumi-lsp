package yaml

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/iwahbe/pulumi-lsp/sdk/lsp"
)

type server struct {
}

func Methods() *lsp.Methods {
	server := &server{}
	return &lsp.Methods{
		InitializeFunc:  server.initialize,
		InitializedFunc: server.initialized,
	}
}

func (s *server) initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{},
		ServerInfo: &protocol.ServerInfo{
			Name:    "pulumi-yaml-lsp",
			Version: "0.1.0",
		},
	}, nil
}

func (s *server) initialized(ctx context.Context, params *protocol.InitializedParams) error {
	return nil
}
