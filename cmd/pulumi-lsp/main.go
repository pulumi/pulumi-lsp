// Copyright 2022, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi-lsp/sdk/version"
	"github.com/pulumi/pulumi-lsp/sdk/yaml"
)

func main() {
	defer panicHandler()
	if err := newLSPCommand().Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		// We ignore the error, since there is nothing to do with it
		os.Exit(1)
	}
}

func newLSPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pulumi-lsp",
		Short: "A LSP for Pulumi YAML",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			host, err := defaultPluginHost()
			if err != nil {
				panic(err)
			}
			defer func() {
				if err := host.Close(); err != nil {
					panic(err)
				}
			}()
			server := lsp.NewServer(yaml.Methods(host), &stdio{false})
			err = server.Run(context.Background())
			if err != nil {
				panic(err)
			}
		},
	}

	cmd.AddCommand(newVersionCmd())
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Pulumi's version number",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			fmt.Printf("%v\n", version.Version)
		},
	}
}

func panicHandler() {
	if panicPayload := recover(); panicPayload != nil {
		stack := string(debug.Stack())
		fmt.Fprintln(os.Stderr, "================================================================================")
		fmt.Fprintln(os.Stderr, "Pulumi LSP encountered a fatal error. This is a bug!")
		fmt.Fprintln(os.Stderr, "We would appreciate a report: https://github.com/pulumi/pulumi-lsp/issues/")
		fmt.Fprintln(os.Stderr, "Please provide all of the below text in your report.")
		fmt.Fprintln(os.Stderr, "================================================================================")
		fmt.Fprintf(os.Stderr, "pulumi-lsp Version:   %s\n", version.Version)
		fmt.Fprintf(os.Stderr, "Go Version:           %s\n", runtime.Version())
		fmt.Fprintf(os.Stderr, "Go Compiler:          %s\n", runtime.Compiler)
		fmt.Fprintf(os.Stderr, "Architecture:         %s\n", runtime.GOARCH)
		fmt.Fprintf(os.Stderr, "Operating System:     %s\n", runtime.GOOS)
		fmt.Fprintf(os.Stderr, "Panic:                %s\n\n", panicPayload)
		fmt.Fprintln(os.Stderr, stack)
		os.Exit(1)
	}
}

func defaultPluginHost() (plugin.Host, error) {
	var cfg plugin.ConfigSource
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	sink := diag.DefaultSink(&stdio{false}, &stdio{false}, diag.FormatOptions{
		Color: colors.Never,
	})
	context, err := plugin.NewContext(sink, sink, nil, cfg, pwd, nil, false, nil)
	if err != nil {
		return nil, err
	}
	return plugin.NewDefaultHost(context, nil, nil, false)
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
