// Copyright 2022, Pulumi Corporation.  All rights reserved.

// Implements the LSP server itself, providing the jsonrpc2 to lsp protocol.
// This module does not handle business logic for the actual language.

package lsp

import (
	"context"
	"io"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"
)

type Server struct {
	methods       *Methods
	conn          io.ReadWriteCloser
	cancel        <-chan struct{}
	isInitialized bool
	client        protocol.Client

	Logger *zap.SugaredLogger
}

// Create a new server backed by `Methods`.
func NewServer(methods *Methods, conn io.ReadWriteCloser) Server {
	return Server{
		methods: methods,
		conn:    conn,
	}
}

func (s *Server) Run() error {
	if s.Logger == nil {
		logger, err := zap.NewDevelopment()
		if err != nil {
			return err
		}
		s.Logger = logger.Sugar()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.run(ctx)
	<-s.cancel
	return nil
}

type ConnectOpts struct {
	Address string
	Network string
}

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
