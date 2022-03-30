package lsp

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"go.lsp.dev/protocol"
)

// Methods provides the interface to define methods for the LSP server.
type Methods struct {
	// A pointer back to the server
	server *Server
	// And a channel to indicate that the server has exited
	closer chan<- struct{}

	InitializeFunc                func(client Client, params *protocol.InitializeParams) (result *protocol.InitializeResult, err error)
	InitializedFunc               func(client Client, params *protocol.InitializedParams) (err error)
	ShutdownFunc                  func(client Client) (err error)
	ExitFunc                      func(client Client) (err error)
	WorkDoneProgressCancelFunc    func(client Client, params *protocol.WorkDoneProgressCancelParams) (err error)
	LogTraceFunc                  func(client Client, params *protocol.LogTraceParams) (err error)
	SetTraceFunc                  func(client Client, params *protocol.SetTraceParams) (err error)
	CodeActionFunc                func(client Client, params *protocol.CodeActionParams) (result []protocol.CodeAction, err error)
	CodeLensFunc                  func(client Client, params *protocol.CodeLensParams) (result []protocol.CodeLens, err error)
	CodeLensResolveFunc           func(client Client, params *protocol.CodeLens) (result *protocol.CodeLens, err error)
	ColorPresentationFunc         func(client Client, params *protocol.ColorPresentationParams) (result []protocol.ColorPresentation, err error)
	CompletionFunc                func(client Client, params *protocol.CompletionParams) (result *protocol.CompletionList, err error)
	CompletionResolveFunc         func(client Client, params *protocol.CompletionItem) (result *protocol.CompletionItem, err error)
	DeclarationFunc               func(client Client, params *protocol.DeclarationParams) (result []protocol.Location, err error)
	DefinitionFunc                func(client Client, params *protocol.DefinitionParams) (result []protocol.Location, err error)
	DidChangeFunc                 func(client Client, params *protocol.DidChangeTextDocumentParams) (err error)
	DidChangeConfigurationFunc    func(client Client, params *protocol.DidChangeConfigurationParams) (err error)
	DidChangeWatchedFilesFunc     func(client Client, params *protocol.DidChangeWatchedFilesParams) (err error)
	DidChangeWorkspaceFoldersFunc func(client Client, params *protocol.DidChangeWorkspaceFoldersParams) (err error)
	DidCloseFunc                  func(client Client, params *protocol.DidCloseTextDocumentParams) (err error)
	DidOpenFunc                   func(client Client, params *protocol.DidOpenTextDocumentParams) (err error)
	DidSaveFunc                   func(client Client, params *protocol.DidSaveTextDocumentParams) (err error)
	DocumentColorFunc             func(client Client, params *protocol.DocumentColorParams) (result []protocol.ColorInformation, err error)
	DocumentHighlightFunc         func(client Client, params *protocol.DocumentHighlightParams) (result []protocol.DocumentHighlight, err error)
	DocumentLinkFunc              func(client Client, params *protocol.DocumentLinkParams) (result []protocol.DocumentLink, err error)
	DocumentLinkResolveFunc       func(client Client, params *protocol.DocumentLink) (result *protocol.DocumentLink, err error)
	DocumentSymbolFunc            func(client Client, params *protocol.DocumentSymbolParams) (result []interface{}, err error)
	ExecuteCommandFunc            func(client Client, params *protocol.ExecuteCommandParams) (result interface{}, err error)
	FoldingRangesFunc             func(client Client, params *protocol.FoldingRangeParams) (result []protocol.FoldingRange, err error)
	FormattingFunc                func(client Client, params *protocol.DocumentFormattingParams) (result []protocol.TextEdit, err error)
	HoverFunc                     func(client Client, params *protocol.HoverParams) (result *protocol.Hover, err error)
	ImplementationFunc            func(client Client, params *protocol.ImplementationParams) (result []protocol.Location, err error)
	OnTypeFormattingFunc          func(client Client, params *protocol.DocumentOnTypeFormattingParams) (result []protocol.TextEdit, err error)
	PrepareRenameFunc             func(client Client, params *protocol.PrepareRenameParams) (result *protocol.Range, err error)
	RangeFormattingFunc           func(client Client, params *protocol.DocumentRangeFormattingParams) (result []protocol.TextEdit, err error)
	ReferencesFunc                func(client Client, params *protocol.ReferenceParams) (result []protocol.Location, err error)
	RenameFunc                    func(client Client, params *protocol.RenameParams) (result *protocol.WorkspaceEdit, err error)
	SignatureHelpFunc             func(client Client, params *protocol.SignatureHelpParams) (result *protocol.SignatureHelp, err error)
	SymbolsFunc                   func(client Client, params *protocol.WorkspaceSymbolParams) (result []protocol.SymbolInformation, err error)
	TypeDefinitionFunc            func(client Client, params *protocol.TypeDefinitionParams) (result []protocol.Location, err error)
	WillSaveFunc                  func(client Client, params *protocol.WillSaveTextDocumentParams) (err error)
	WillSaveWaitUntilFunc         func(client Client, params *protocol.WillSaveTextDocumentParams) (result []protocol.TextEdit, err error)
	ShowDocumentFunc              func(client Client, params *protocol.ShowDocumentParams) (result *protocol.ShowDocumentResult, err error)
	WillCreateFilesFunc           func(client Client, params *protocol.CreateFilesParams) (result *protocol.WorkspaceEdit, err error)
	DidCreateFilesFunc            func(client Client, params *protocol.CreateFilesParams) (err error)
	WillRenameFilesFunc           func(client Client, params *protocol.RenameFilesParams) (result *protocol.WorkspaceEdit, err error)
	DidRenameFilesFunc            func(client Client, params *protocol.RenameFilesParams) (err error)
	WillDeleteFilesFunc           func(client Client, params *protocol.DeleteFilesParams) (result *protocol.WorkspaceEdit, err error)
	DidDeleteFilesFunc            func(client Client, params *protocol.DeleteFilesParams) (err error)
	CodeLensRefreshFunc           func(client Client) (err error)
	PrepareCallHierarchyFunc      func(client Client, params *protocol.CallHierarchyPrepareParams) (result []protocol.CallHierarchyItem, err error)
	IncomingCallsFunc             func(client Client, params *protocol.CallHierarchyIncomingCallsParams) (result []protocol.CallHierarchyIncomingCall, err error)
	OutgoingCallsFunc             func(client Client, params *protocol.CallHierarchyOutgoingCallsParams) (result []protocol.CallHierarchyOutgoingCall, err error)
	SemanticTokensFullFunc        func(client Client, params *protocol.SemanticTokensParams) (result *protocol.SemanticTokens, err error)
	SemanticTokensFullDeltaFunc   func(client Client, params *protocol.SemanticTokensDeltaParams) (result interface{}, err error)
	SemanticTokensRangeFunc       func(client Client, params *protocol.SemanticTokensRangeParams) (result *protocol.SemanticTokens, err error)
	SemanticTokensRefreshFunc     func(client Client) (err error)
	LinkedEditingRangeFunc        func(client Client, params *protocol.LinkedEditingRangeParams) (result *protocol.LinkedEditingRanges, err error)
	MonikerFunc                   func(client Client, params *protocol.MonikerParams) (result []protocol.Moniker, err error)
	RequestFunc                   func(client Client, method string, params interface{}) (result interface{}, err error)
}

// Guess what capabilities should be enabled from what functions are registered.
//
// This function will panic if a `InitializeFunc` is already set.
func (m Methods) DefaultInitializer(name, version string) *Methods {
	contract.Assertf(m.InitializeFunc == nil, "Won't override an already set initializer")
	m.InitializeFunc = func(client Client, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
		var completion *protocol.CompletionOptions
		if m.CompletionFunc != nil || m.CompletionResolveFunc != nil {
			completion = &protocol.CompletionOptions{
				ResolveProvider:   m.CompletionResolveFunc != nil,
				TriggerCharacters: []string{".", "::"}, // TODO: How should this be provided
			}
		}
		var hover *protocol.HoverOptions
		if m.HoverFunc != nil {
			hover = &protocol.HoverOptions{
				WorkDoneProgressOptions: protocol.WorkDoneProgressOptions{
					WorkDoneProgress: m.WorkDoneProgressCancelFunc != nil,
				},
			}
		}
		return &protocol.InitializeResult{
			Capabilities: protocol.ServerCapabilities{
				TextDocumentSync: &protocol.TextDocumentSyncOptions{
					OpenClose:         m.DidOpenFunc != nil || m.DidCloseFunc != nil,
					Change:            protocol.TextDocumentSyncKindIncremental, // TODO: Can we figure out how do derive this
					WillSave:          m.WillSaveFunc != nil,
					WillSaveWaitUntil: m.WillSaveWaitUntilFunc != nil,
					Save: &protocol.SaveOptions{
						IncludeText: false, // TODO: how should this information be passed
					},
				},
				CompletionProvider: completion,
				HoverProvider:      hover,
				// SignatureHelpProvider:            &protocol.SignatureHelpOptions{},
				// DeclarationProvider:              nil,
				// DefinitionProvider:               nil,
				// TypeDefinitionProvider:           nil,
				// ImplementationProvider:           nil,
				// ReferencesProvider:               nil,
				// DocumentHighlightProvider:        nil,
				// DocumentSymbolProvider:           nil,
				// CodeActionProvider:               nil,
				// CodeLensProvider:                 &protocol.CodeLensOptions{},
				// DocumentLinkProvider:             &protocol.DocumentLinkOptions{},
				// ColorProvider:                    nil,
				// WorkspaceSymbolProvider:          nil,
				// DocumentFormattingProvider:       nil,
				// DocumentRangeFormattingProvider:  nil,
				// DocumentOnTypeFormattingProvider: &protocol.DocumentOnTypeFormattingOptions{},
				// RenameProvider:                   name,
				// FoldingRangeProvider:             nil,
				// SelectionRangeProvider:           nil,
				// ExecuteCommandProvider:           &protocol.ExecuteCommandOptions{},
				// CallHierarchyProvider:            nil,
				// LinkedEditingRangeProvider:       nil,
				// SemanticTokensProvider:           nil,
				// Workspace:                        &protocol.ServerCapabilitiesWorkspace{},
				// MonikerProvider:                  m,
				// Experimental:                     nil,
			},
			ServerInfo: &protocol.ServerInfo{
				Name:    name,
				Version: version,
			},
		}, nil
	}
	return &m
}

func (m *methods) client(ctx context.Context) Client {
	return Client{
		inner: m.server.client,
		ctx:   ctx,
	}
}

func (m *Methods) serve() *methods {
	return &methods{m}
}

// The actual implementer of the protocol.Server trait. We do this to prevent
// calling a method on `Methods`, and to keep auto-complete uncluttered.
type methods struct {
	*Methods
}

func (m *methods) warnUninitialized(name string) {
	m.server.Logger.Debugf("'%s' was called but no handler was provided", name)
}

func (m *methods) Initialize(ctx context.Context, params *protocol.InitializeParams) (result *protocol.InitializeResult, err error) {
	if m.InitializeFunc != nil {
		result, err = m.InitializeFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("initialize")
	}
	m.server.isInitialized = true
	return
}
func (m *methods) Initialized(ctx context.Context, params *protocol.InitializedParams) (err error) {
	if m.InitializedFunc != nil {
		err = m.InitializedFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("initialized")
	}
	return
}
func (m *methods) Shutdown(ctx context.Context) (err error) {
	if m.ShutdownFunc != nil {
		err = m.ShutdownFunc(m.client(ctx))
	} else {
		m.warnUninitialized("shutdown")
	}
	return
}
func (m *methods) Exit(ctx context.Context) (err error) {
	if m.ExitFunc != nil {
		err = m.ExitFunc(m.client(ctx))
	} else {
		m.warnUninitialized("exit")
	}
	m.closer <- struct{}{}
	return
}
func (m *methods) WorkDoneProgressCancel(ctx context.Context, params *protocol.WorkDoneProgressCancelParams) (err error) {
	if m.WorkDoneProgressCancelFunc != nil {
		err = m.WorkDoneProgressCancelFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("*methods) workDoneProgressCancel")
	}
	return
}
func (m *methods) LogTrace(ctx context.Context, params *protocol.LogTraceParams) (err error) {
	if m.LogTraceFunc != nil {
		err = m.LogTraceFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("logTrace")
	}
	return
}
func (m *methods) SetTrace(ctx context.Context, params *protocol.SetTraceParams) (err error) {
	if m.SetTraceFunc != nil {
		err = m.SetTraceFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("setTrace")
	}
	return
}
func (m *methods) CodeAction(ctx context.Context, params *protocol.CodeActionParams) (result []protocol.CodeAction, err error) {
	if m.CodeActionFunc != nil {
		result, err = m.CodeActionFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("codeAction")
	}
	return
}
func (m *methods) CodeLens(ctx context.Context, params *protocol.CodeLensParams) (result []protocol.CodeLens, err error) {
	if m.CodeLensFunc != nil {
		result, err = m.CodeLensFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("codeLens")
	}
	return
}
func (m *methods) CodeLensResolve(ctx context.Context, params *protocol.CodeLens) (result *protocol.CodeLens, err error) {
	if m.CodeLensResolveFunc != nil {
		result, err = m.CodeLensResolveFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("codeLensResolve")
	}
	return
}
func (m *methods) ColorPresentation(ctx context.Context, params *protocol.ColorPresentationParams) (result []protocol.ColorPresentation, err error) {
	if m.ColorPresentationFunc != nil {
		result, err = m.ColorPresentationFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("colorPresentation")
	}
	return
}
func (m *methods) Completion(ctx context.Context, params *protocol.CompletionParams) (result *protocol.CompletionList, err error) {
	if m.CompletionFunc != nil {
		result, err = m.CompletionFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("completion")
	}
	return
}
func (m *methods) CompletionResolve(ctx context.Context, params *protocol.CompletionItem) (result *protocol.CompletionItem, err error) {
	if m.CompletionResolveFunc != nil {
		result, err = m.CompletionResolveFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("completionResolve")
	}
	return
}
func (m *methods) Declaration(ctx context.Context, params *protocol.DeclarationParams) (result []protocol.Location, err error) {
	if m.DeclarationFunc != nil {
		result, err = m.DeclarationFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("declaration")
	}
	return
}
func (m *methods) Definition(ctx context.Context, params *protocol.DefinitionParams) (result []protocol.Location, err error) {
	if m.DefinitionFunc != nil {
		result, err = m.DefinitionFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("definition")
	}
	return
}
func (m *methods) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) (err error) {
	if m.DidChangeFunc != nil {
		err = m.DidChangeFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("didChange")
	}
	return
}
func (m *methods) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) (err error) {
	if m.DidChangeConfigurationFunc != nil {
		err = m.DidChangeConfigurationFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("didChangeConfiguration")
	}
	return
}
func (m *methods) DidChangeWatchedFiles(ctx context.Context, params *protocol.DidChangeWatchedFilesParams) (err error) {
	if m.DidChangeWatchedFilesFunc != nil {
		err = m.DidChangeWatchedFilesFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("didChangeWatchedFiles")
	}
	return
}
func (m *methods) DidChangeWorkspaceFolders(ctx context.Context, params *protocol.DidChangeWorkspaceFoldersParams) (err error) {
	if m.DidChangeWorkspaceFoldersFunc != nil {
		err = m.DidChangeWorkspaceFoldersFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("didChangeWorkspaceFolders")
	}
	return
}
func (m *methods) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) (err error) {
	if m.DidCloseFunc != nil {
		err = m.DidCloseFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("didClose")
	}
	return
}
func (m *methods) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) (err error) {
	if m.DidOpenFunc != nil {
		err = m.DidOpenFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("didOpen")
	}
	return
}
func (m *methods) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) (err error) {
	if m.DidSaveFunc != nil {
		err = m.DidSaveFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("didSave")
	}
	return
}
func (m *methods) DocumentColor(ctx context.Context, params *protocol.DocumentColorParams) (result []protocol.ColorInformation, err error) {
	if m.DocumentColorFunc != nil {
		result, err = m.DocumentColorFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("documentColor")
	}
	return
}
func (m *methods) DocumentHighlight(ctx context.Context, params *protocol.DocumentHighlightParams) (result []protocol.DocumentHighlight, err error) {
	if m.DocumentHighlightFunc != nil {
		result, err = m.DocumentHighlightFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("documentHighlight")
	}
	return
}
func (m *methods) DocumentLink(ctx context.Context, params *protocol.DocumentLinkParams) (result []protocol.DocumentLink, err error) {
	if m.DocumentLinkFunc != nil {
		result, err = m.DocumentLinkFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("documentLink")
	}
	return
}
func (m *methods) DocumentLinkResolve(ctx context.Context, params *protocol.DocumentLink) (result *protocol.DocumentLink, err error) {
	if m.DocumentLinkResolveFunc != nil {
		result, err = m.DocumentLinkResolveFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("documentLinkResolve")
	}
	return
}
func (m *methods) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) (result []interface{}, err error) {
	if m.DocumentSymbolFunc != nil {
		result, err = m.DocumentSymbolFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("documentSymbol")
	}
	return
}
func (m *methods) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (result interface{}, err error) {
	if m.ExecuteCommandFunc != nil {
		result, err = m.ExecuteCommandFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("executeCommand")
	}
	return
}
func (m *methods) FoldingRanges(ctx context.Context, params *protocol.FoldingRangeParams) (result []protocol.FoldingRange, err error) {
	if m.FoldingRangesFunc != nil {
		result, err = m.FoldingRangesFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("foldingRanges")
	}
	return
}
func (m *methods) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) (result []protocol.TextEdit, err error) {
	if m.FormattingFunc != nil {
		result, err = m.FormattingFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("formatting")
	}
	return
}
func (m *methods) Hover(ctx context.Context, params *protocol.HoverParams) (result *protocol.Hover, err error) {
	if m.HoverFunc != nil {
		result, err = m.HoverFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("hover")
	}
	return
}
func (m *methods) Implementation(ctx context.Context, params *protocol.ImplementationParams) (result []protocol.Location, err error) {
	if m.ImplementationFunc != nil {
		result, err = m.ImplementationFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("implementation")
	}
	return
}
func (m *methods) OnTypeFormatting(ctx context.Context, params *protocol.DocumentOnTypeFormattingParams) (result []protocol.TextEdit, err error) {
	if m.OnTypeFormattingFunc != nil {
		result, err = m.OnTypeFormattingFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("onTypeFormatting")
	}
	return
}
func (m *methods) PrepareRename(ctx context.Context, params *protocol.PrepareRenameParams) (result *protocol.Range, err error) {
	if m.PrepareRenameFunc != nil {
		result, err = m.PrepareRenameFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("prepareRename")
	}
	return
}
func (m *methods) RangeFormatting(ctx context.Context, params *protocol.DocumentRangeFormattingParams) (result []protocol.TextEdit, err error) {
	if m.RangeFormattingFunc != nil {
		result, err = m.RangeFormattingFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("rangeFormatting")
	}
	return
}
func (m *methods) References(ctx context.Context, params *protocol.ReferenceParams) (result []protocol.Location, err error) {
	if m.ReferencesFunc != nil {
		result, err = m.ReferencesFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("references")
	}
	return
}
func (m *methods) Rename(ctx context.Context, params *protocol.RenameParams) (result *protocol.WorkspaceEdit, err error) {
	if m.RenameFunc != nil {
		result, err = m.RenameFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("rename")
	}
	return
}
func (m *methods) SignatureHelp(ctx context.Context, params *protocol.SignatureHelpParams) (result *protocol.SignatureHelp, err error) {
	if m.SignatureHelpFunc != nil {
		result, err = m.SignatureHelpFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("signatureHelp")
	}
	return
}
func (m *methods) Symbols(ctx context.Context, params *protocol.WorkspaceSymbolParams) (result []protocol.SymbolInformation, err error) {
	if m.SymbolsFunc != nil {
		result, err = m.SymbolsFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("symbols")
	}
	return
}
func (m *methods) TypeDefinition(ctx context.Context, params *protocol.TypeDefinitionParams) (result []protocol.Location, err error) {
	if m.TypeDefinitionFunc != nil {
		result, err = m.TypeDefinitionFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("typeDefinition")
	}
	return
}
func (m *methods) WillSave(ctx context.Context, params *protocol.WillSaveTextDocumentParams) (err error) {
	if m.WillSaveFunc != nil {
		err = m.WillSaveFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("willSave")
	}
	return
}
func (m *methods) WillSaveWaitUntil(ctx context.Context, params *protocol.WillSaveTextDocumentParams) (result []protocol.TextEdit, err error) {
	if m.WillSaveWaitUntilFunc != nil {
		result, err = m.WillSaveWaitUntilFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("willSaveWaitUntil")
	}
	return
}
func (m *methods) ShowDocument(ctx context.Context, params *protocol.ShowDocumentParams) (result *protocol.ShowDocumentResult, err error) {
	if m.ShowDocumentFunc != nil {
		result, err = m.ShowDocumentFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("showDocument")
	}
	return
}
func (m *methods) WillCreateFiles(ctx context.Context, params *protocol.CreateFilesParams) (result *protocol.WorkspaceEdit, err error) {
	if m.WillCreateFilesFunc != nil {
		result, err = m.WillCreateFilesFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("willCreateFiles")
	}
	return
}
func (m *methods) DidCreateFiles(ctx context.Context, params *protocol.CreateFilesParams) (err error) {
	if m.DidCreateFilesFunc != nil {
		err = m.DidCreateFilesFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("didCreateFiles")
	}
	return
}
func (m *methods) WillRenameFiles(ctx context.Context, params *protocol.RenameFilesParams) (result *protocol.WorkspaceEdit, err error) {
	if m.WillRenameFilesFunc != nil {
		result, err = m.WillRenameFilesFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("willRenameFiles")
	}
	return
}
func (m *methods) DidRenameFiles(ctx context.Context, params *protocol.RenameFilesParams) (err error) {
	if m.DidRenameFilesFunc != nil {
		err = m.DidRenameFilesFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("didRenameFiles")
	}
	return
}
func (m *methods) WillDeleteFiles(ctx context.Context, params *protocol.DeleteFilesParams) (result *protocol.WorkspaceEdit, err error) {
	if m.WillDeleteFilesFunc != nil {
		result, err = m.WillDeleteFilesFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("willDeleteFiles")
	}
	return
}
func (m *methods) DidDeleteFiles(ctx context.Context, params *protocol.DeleteFilesParams) (err error) {
	if m.DidDeleteFilesFunc != nil {
		err = m.DidDeleteFilesFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("didDeleteFiles")
	}
	return
}
func (m *methods) CodeLensRefresh(ctx context.Context) (err error) {
	if m.CodeLensRefreshFunc != nil {
		err = m.CodeLensRefreshFunc(m.client(ctx))
	} else {
		m.warnUninitialized("codeLensRefresh")
	}
	return
}
func (m *methods) PrepareCallHierarchy(ctx context.Context, params *protocol.CallHierarchyPrepareParams) (result []protocol.CallHierarchyItem, err error) {
	if m.PrepareCallHierarchyFunc != nil {
		result, err = m.PrepareCallHierarchyFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("prepareCallHierarchy")
	}
	return
}
func (m *methods) IncomingCalls(ctx context.Context, params *protocol.CallHierarchyIncomingCallsParams) (result []protocol.CallHierarchyIncomingCall, err error) {
	if m.IncomingCallsFunc != nil {
		result, err = m.IncomingCallsFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("incomingCalls")
	}
	return
}
func (m *methods) OutgoingCalls(ctx context.Context, params *protocol.CallHierarchyOutgoingCallsParams) (result []protocol.CallHierarchyOutgoingCall, err error) {
	if m.OutgoingCallsFunc != nil {
		result, err = m.OutgoingCallsFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("outgoingCalls")
	}
	return
}
func (m *methods) SemanticTokensFull(ctx context.Context, params *protocol.SemanticTokensParams) (result *protocol.SemanticTokens, err error) {
	if m.SemanticTokensFullFunc != nil {
		result, err = m.SemanticTokensFullFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("semanticTokensFull")
	}
	return
}
func (m *methods) SemanticTokensFullDelta(ctx context.Context, params *protocol.SemanticTokensDeltaParams) (result interface{}, err error) {
	if m.SemanticTokensFullDeltaFunc != nil {
		result, err = m.SemanticTokensFullDeltaFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("semanticTokensFullDelta")
	}
	return
}
func (m *methods) SemanticTokensRange(ctx context.Context, params *protocol.SemanticTokensRangeParams) (result *protocol.SemanticTokens, err error) {
	if m.SemanticTokensRangeFunc != nil {
		result, err = m.SemanticTokensRangeFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("semanticTokensRange")
	}
	return
}
func (m *methods) SemanticTokensRefresh(ctx context.Context) (err error) {
	if m.SemanticTokensRefreshFunc != nil {
		err = m.SemanticTokensRefreshFunc(m.client(ctx))
	} else {
		m.warnUninitialized("semanticTokensRefresh")
	}
	return
}
func (m *methods) LinkedEditingRange(ctx context.Context, params *protocol.LinkedEditingRangeParams) (result *protocol.LinkedEditingRanges, err error) {
	if m.LinkedEditingRangeFunc != nil {
		result, err = m.LinkedEditingRangeFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("linkedEditingRange")
	}
	return
}
func (m *methods) Moniker(ctx context.Context, params *protocol.MonikerParams) (result []protocol.Moniker, err error) {
	if m.MonikerFunc != nil {
		result, err = m.MonikerFunc(m.client(ctx), params)
	} else {
		m.warnUninitialized("moniker")
	}
	return
}
func (m *methods) Request(ctx context.Context, method string, params interface{}) (result interface{}, err error) {
	if m.RequestFunc != nil {
		result, err = m.RequestFunc(m.client(ctx), method, params)
	} else {
		m.warnUninitialized("request")
	}
	return
}
