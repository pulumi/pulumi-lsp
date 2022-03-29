package main

import (
	"io"
	"os"

	"github.com/iwahbe/pulumi-lsp/sdk/lsp"
	"github.com/iwahbe/pulumi-lsp/sdk/yaml"
)

func main() {
	server := lsp.NewServer(yaml.Methods(), &stdio{false})
	err := server.Run()
	if err != nil {
		panic(err)
	}
}

// An io.ReadWriteCloser, whose value indicates if the closer is closed.
type stdio struct{ bool }

func (s *stdio) Read(p []byte) (n int, err error) {
	if s.bool {
		return 0, io.EOF
	}
	return os.Stdin.Read(p)
}

func (s *stdio) Write(p []byte) (n int, err error) {
	if s.bool {
		return 0, io.EOF
	}
	return os.Stdout.Write(p)
}

func (s *stdio) Close() error {
	s.bool = true
	return nil
}
