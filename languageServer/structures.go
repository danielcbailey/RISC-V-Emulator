package languageServer

import "github.gatech.edu/ECEInnovation/RISC-V-Emulator/assembler"

type TextDocumentItem struct {
	URI                 DocumentUri `json:"uri"`
	LanguageID          string      `json:"languageId"`
	Version             int         `json:"version"`
	Text                string      `json:"text"`
	lastAssembledResult *assembler.AssembledResult
}

type DocumentUri string

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type TextDocumentIdentifier struct {
	URI DocumentUri `json:"uri"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type VersionedTextDocumentIdentifier struct {
	URI     DocumentUri `json:"uri"`
	Version int         `json:"version"`
}

type TextDocumentContentChangeEvent struct {
	Text string `json:"text"` // only will register the full change capability
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type InitializeParams struct {
	ProcessID int `json:"processId"` // eh don't care about the rest...
}

type DocumentDiagnosticsParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DocumentDiagnosticsReport struct {
	//RelatedFullDocumentDiagnosticReport
	Kind  string                 `json:"kind"` // should always be "full"
	Items []assembler.Diagnostic `json:"items"`
}

type PublishDiagnosticsParams struct {
	URI         DocumentUri            `json:"uri"`
	Version     int                    `json:"version"`
	Diagnostics []assembler.Diagnostic `json:"diagnostics"`
}

type TextEdit struct {
	Range   assembler.TextRange `json:"range"`
	NewText string              `json:"newText"`
}

type DocumentWillSaveWaitUntilParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Reason       int                    `json:"reason"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     assembler.TextPosition `json:"position"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
}

// Capabilities

type DiagnosticOptions struct {
	WorkDoneProgress      bool `json:"workDoneProgress"`
	InterFileDependencies bool `json:"interFileDependencies"`
	WorkspaceDiagnostics  bool `json:"workspaceDiagnostics"`
}

type ServerCapabilities struct {
	TextDocumentSync  int               `json:"textDocumentSync"`
	DiagnosticOptions DiagnosticOptions `json:"diagnosticOptions"`
	HoverProvider     bool              `json:"hoverProvider"`
	// will add more later as implemented
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type DocumentFilter struct {
	Language string `json:"language"`
	Scheme   string `json:"scheme"`
}

type DocumentSelector []DocumentFilter

type TextDocumentRegistrationOptions struct {
	DocumentSelector DocumentSelector `json:"documentSelector"`
}

type Registration struct {
	ID              string      `json:"id"`
	Method          string      `json:"method"`
	RegisterOptions interface{} `json:"registerOptions"`
}

type RegistrationParams struct {
	Registrations []Registration `json:"registrations"`
}
