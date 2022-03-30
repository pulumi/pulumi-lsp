package main

import (
	"io"
	"os"

	"github.com/iwahbe/pulumi-lsp/sdk/lsp"
	"github.com/iwahbe/pulumi-lsp/sdk/yaml"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func main() {
	var cfg plugin.ConfigSource
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	sink := diag.DefaultSink(&stdio{false}, &stdio{false}, diag.FormatOptions{})
	context, err := plugin.NewContext(sink, sink, nil, cfg, pwd, nil, false, nil)
	if err != nil {
		panic(err)
	}
	host, err := plugin.NewDefaultHost(context, nil, nil, false)
	if err != nil {
		panic(err)
	}
	server := lsp.NewServer(yaml.Methods(host), &stdio{false})
	err = server.Run()
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
