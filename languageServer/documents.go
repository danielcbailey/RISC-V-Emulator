package languageServer

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sourcegraph/jsonrpc2"
	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/assembler"
	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/util"
)

var documentMap = make(map[string]TextDocumentItem) // map from uri to document

func assembleAndReportDiagnostics(conn *jsonrpc2.Conn, uri DocumentUri) []assembler.Diagnostic {
	doc := documentMap[string(uri)]

	assembledRes := assembler.Assemble(doc.Text)
	if assembledRes.Diagnostics == nil {
		assembledRes.Diagnostics = make([]assembler.Diagnostic, 0)
	}
	doc.lastAssembledResult = assembledRes
	documentMap[string(uri)] = doc
	return assembledRes.Diagnostics
}

func documentOpenNotification(conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// parse req params as DidOpenTextDocumentParams
	// add document to documents map
	decodedParams := DidOpenTextDocumentParams{}
	err := json.Unmarshal(*req.Params, &decodedParams)
	if err != nil {
		rpcErr := jsonrpc2.Error{}
		rpcErr.SetError("invalid parameters")
		conn.ReplyWithError(context.Background(), req.ID, &rpcErr)
		return
	}

	documentMap[string(decodedParams.TextDocument.URI)] = decodedParams.TextDocument

	diagnostics := assembleAndReportDiagnostics(conn, decodedParams.TextDocument.URI)
	conn.Notify(context.Background(), "textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         decodedParams.TextDocument.URI,
		Diagnostics: diagnostics,
	})
}

func documentCloseNotification(conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// parse req params as DidCloseTextDocumentParams
	// remove document from documents map
	decodedParams := DidCloseTextDocumentParams{}
	err := json.Unmarshal(*req.Params, &decodedParams)
	if err != nil {
		rpcErr := jsonrpc2.Error{}
		rpcErr.SetError("invalid parameters")
		conn.ReplyWithError(context.Background(), req.ID, &rpcErr)
		return
	}

	delete(documentMap, string(decodedParams.TextDocument.URI))
}

func documentChangeNotification(conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// parse req params as DidChangeTextDocumentParams
	// update document in documents map
	decodedParams := DidChangeTextDocumentParams{}
	err := json.Unmarshal(*req.Params, &decodedParams)
	if err != nil {
		rpcErr := jsonrpc2.Error{}
		rpcErr.SetError("invalid parameters")
		conn.ReplyWithError(context.Background(), req.ID, &rpcErr)
		return
	}

	doc := documentMap[string(decodedParams.TextDocument.URI)]
	doc.Text = decodedParams.ContentChanges[0].Text
	doc.Version = decodedParams.TextDocument.Version
	documentMap[string(decodedParams.TextDocument.URI)] = doc

	diagnostics := assembleAndReportDiagnostics(conn, decodedParams.TextDocument.URI)
	conn.Notify(context.Background(), "textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         decodedParams.TextDocument.URI,
		Version:     doc.Version,
		Diagnostics: diagnostics,
	})
}

func documentDiagnostics(conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// parse req params as DocumentDiagnosticsParams
	// assemble document and return diagnostics
	decodedParams := DocumentDiagnosticsParams{}
	err := json.Unmarshal(*req.Params, &decodedParams)
	if err != nil {
		rpcErr := jsonrpc2.Error{}
		rpcErr.SetError("invalid parameters")
		conn.ReplyWithError(context.Background(), req.ID, &rpcErr)
		return
	}

	diagnostics := assembleAndReportDiagnostics(conn, decodedParams.TextDocument.URI)
	conn.Reply(context.Background(), req.ID, DocumentDiagnosticsReport{
		Kind:  "full",
		Items: diagnostics,
	})
}

func reformatDocument(uri DocumentUri) string {
	doc := documentMap[string(uri)]
	assembledRes := assembler.Assemble(doc.Text)

	// for all lines that don't start with a label, will add spaces until the number of whitespaces equals the length of the longest label
	lines := strings.Split(doc.Text, "\n")
	maxLabelLength := 0
	for label := range assembledRes.Labels {
		if len(label) > maxLabelLength {
			maxLabelLength = len(label)
		}
	}

	for i, line := range lines {
		withoutComment := strings.Split(line, "#")[0]
		withComment := ""
		if strings.Contains(line, "#") {
			withComment = "#" + strings.SplitN(line, "#", 2)[1]
		}
		lineWithoutWhitespace := strings.TrimLeft(withoutComment, " \t")
		lineWithoutWhitespace = strings.ReplaceAll(lineWithoutWhitespace, "\t", " ")
		// removing duplicate whitespaces between tokens
		for strings.Contains(lineWithoutWhitespace, "  ") {
			lineWithoutWhitespace = strings.ReplaceAll(lineWithoutWhitespace, "  ", " ")
		}

		if strings.HasPrefix(lineWithoutWhitespace, ".") {
			lines[i] = lineWithoutWhitespace + withComment
		} else if strings.Contains(withoutComment, ":") {
			// removing whitespace from after the label
			afterLabel := lineWithoutWhitespace[strings.Index(lineWithoutWhitespace, ":")+1:]
			for j := 0; j < len(afterLabel); j++ {
				if afterLabel[j] != ' ' && afterLabel[j] != '\t' {
					lineWithoutWhitespace = lineWithoutWhitespace[:strings.Index(lineWithoutWhitespace, ":")+1] + afterLabel[j:]
					break
				}
			}
			lines[i] = lineWithoutWhitespace[:strings.Index(lineWithoutWhitespace, ":")+1] + " " + lineWithoutWhitespace[strings.Index(lineWithoutWhitespace, ":")+1:] + withComment
			continue
		} else {
			// add spaces until the number of whitespaces equals the length of the longest label
			lines[i] = strings.Repeat(" ", maxLabelLength+2) + lineWithoutWhitespace + withComment
		}
	}
	return strings.Join(lines, "\n")
}

func documentWillSaveWaitUntil(conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// parse req params as DocumentWillSaveWaitUntilParams
	// assemble document and return edits
	decodedParams := DocumentWillSaveWaitUntilParams{}
	err := json.Unmarshal(*req.Params, &decodedParams)
	if err != nil {
		rpcErr := jsonrpc2.Error{}
		rpcErr.SetError("invalid parameters")
		conn.ReplyWithError(context.Background(), req.ID, &rpcErr)
		return
	}

	lines := strings.Split(documentMap[string(decodedParams.TextDocument.URI)].Text, "\n")

	edits := make([]TextEdit, 0)
	edits = append(edits, TextEdit{
		Range: assembler.TextRange{
			Start: assembler.TextPosition{Line: 0, Char: 0},
			End:   assembler.TextPosition{Line: len(lines) - 1, Char: len(lines[len(lines)-1])},
		},
		NewText: reformatDocument(decodedParams.TextDocument.URI),
	})

	conn.Reply(context.Background(), req.ID, edits)
	util.LogF("2035 RISC-V Language Server: reformated document")
}
