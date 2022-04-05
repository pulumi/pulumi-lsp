// Copyright 2022, Pulumi Corporation.  All rights reserved.

// The lsp package implements a convenience wrapper around the
// go.lsp.dev/protocol package. It handles setting up a server that replies to
// only some lsp requests, as well as providing other helpful LSP intrinsics.

package lsp

import (
	"context"
	"io"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"
)

// A Server combines a set of LSP methods with the infrastructure needed to
// fullfill the server side of the LSP contract.
type Server struct {
	methods       *Methods
	conn          io.ReadWriteCloser
	cancel        <-chan struct{}
	isInitialized bool
	client        protocol.Client

	// The logger used by the server.
	Logger *zap.SugaredLogger
}

// Create a new server backed by `Methods`. The server reads requests and writes
// responses via `conn`.
func NewServer(methods *Methods, conn io.ReadWriteCloser) Server {
	return Server{
		methods: methods,
		conn:    conn,
	}
}

// Synchronously run the server. The server is rooted in the given context,
// which can be used to cancel the server.
func (s *Server) Run(ctx context.Context) error {
	if s.Logger == nil {
		logger, err := zap.NewDevelopment()
		if err != nil {
			return err
		}
		s.Logger = logger.Sugar()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.run(ctx)
	<-s.cancel
	return nil
}

// Actually kick off the server
func (s *Server) run(ctx context.Context) context.Context {

	closer := make(chan struct{})

	s.cancel = closer
	s.methods.server = s
	s.methods.closer = closer

	stream := jsonrpc2.NewStream(s.conn)
	go func() {
		ctx, _, s.client = protocol.NewServer(ctx, s.methods.serve(), stream, s.Logger.Desugar())
	}()
	return ctx
}
