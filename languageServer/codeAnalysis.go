package languageServer

import (
	"context"
	"encoding/json"

	"github.com/sourcegraph/jsonrpc2"
)

func hoverRequest(conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// parse req params as HoverParams
	decodedParams := TextDocumentPositionParams{}
	err := json.Unmarshal(*req.Params, &decodedParams)
	if err != nil {
		rpcErr := jsonrpc2.Error{}
		rpcErr.SetError("invalid parameters")
		conn.ReplyWithError(context.Background(), req.ID, &rpcErr)
		return
	}

	// get document from documents map
	doc := documentMap[string(decodedParams.TextDocument.URI)]
	text, ok := doc.lastAssembledResult.EvaluateHover(decodedParams.Position)
	if !ok {
		conn.Reply(context.Background(), req.ID, nil)
		return
	}

	// return HoverResponse
	conn.Reply(context.Background(), req.ID, Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: text,
		},
	})
}
