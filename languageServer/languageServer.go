package languageServer

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"

	"github.com/sourcegraph/jsonrpc2"
	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/util"
)

type stdrwc struct{}

func (stdrwc) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

func (stdrwc) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

func (stdrwc) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}

func ListenAndServe() {
	// using stdin and stdout

	h := handler{}
	<-jsonrpc2.NewConn(context.Background(), jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{}), h).DisconnectNotify()
}

func ListenAndServeTCP() {
	// using tcp mode for jsonrpc2
	listen := func(addr string) (*net.Listener, error) {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("Could not bind to address %s: %v", addr, err)
			return nil, err
		}
		return &listener, nil
	}

	addr := ":2035"
	lis, err := listen(addr)
	if err != nil {
		log.Fatalf("failed to listen for tcp traffic: %v", err)
	}
	defer (*lis).Close()

	log.Println("2035 RISC-V Language Server: listening for TCP connections on", addr)

	connectionCount := 0

	for {
		conn, err := (*lis).Accept()
		if err != nil {
			log.Fatalf("failed to accept incoming connection: %v", err)
		}
		connectionCount = connectionCount + 1
		connectionID := connectionCount
		log.Printf("2035 RISC-V Language Server: received incoming connection #%d\n", connectionID)
		handler := handler{}
		jsonrpc2Connection := jsonrpc2.NewConn(context.Background(), jsonrpc2.NewBufferedStream(conn, jsonrpc2.VSCodeObjectCodec{}), handler)
		go func() {
			<-jsonrpc2Connection.DisconnectNotify()
			if err != nil {
				log.Println(err)
			}
			log.Printf("2035 RISC-V Language Server: connection #%d closed\n", connectionID)
		}()
	}
}

type handler struct{}

func (h handler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	util.LogF("2035 RISC-V Language Server: received request: %s", req.Method)
	switch req.Method {
	case "textDocument/didOpen":
		documentOpenNotification(conn, req)
	case "textDocument/didClose":
		documentCloseNotification(conn, req)
	case "textDocument/didChange":
		documentChangeNotification(conn, req)
	case "initialize":
		handleInitialize(conn, req)
	case "textDocument/diagnostic":
		documentDiagnostics(conn, req)
	case "textDocument/willSaveWaitUntil":
		documentWillSaveWaitUntil(conn, req)
	case "textDocument/hover":
		hoverRequest(conn, req)

	// quitting
	case "shutdown":
		conn.Reply(context.Background(), req.ID, nil)
		conn.Close()
	case "exit":
		conn.Reply(context.Background(), req.ID, nil)
		conn.Close()
	}
}

func handleInitialize(conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// parse req params as InitializeParams
	// return InitializeResult
	decodedParams := InitializeParams{}
	err := json.Unmarshal(*req.Params, &decodedParams)
	if err != nil {
		rpcErr := jsonrpc2.Error{}
		rpcErr.SetError("invalid parameters")
		conn.ReplyWithError(context.Background(), req.ID, &rpcErr)
		return
	}

	result := InitializeResult{}
	result.Capabilities.TextDocumentSync = 1
	result.Capabilities.HoverProvider = true
	conn.Reply(context.Background(), req.ID, result)

	registerRemainingCapabilities(conn)
}

func registerRemainingCapabilities(conn *jsonrpc2.Conn) {
	// send register capability requests for all remaining capabilities
	// textDocumentSync.willSaveWaitUntil

	util.LogF("2035 RISC-V Language Server: registering remaining capabilities")
	params := RegistrationParams{
		Registrations: []Registration{
			{
				ID:     "textDocumentSync.willSaveWaitUntil",
				Method: "textDocument/willSaveWaitUntil",
				RegisterOptions: TextDocumentRegistrationOptions{
					DocumentSelector: []DocumentFilter{
						{
							Scheme:   "file",
							Language: "riscv",
						},
					},
				},
			},
		},
	}

	go conn.Call(context.Background(), "client/registerCapability", params, nil)
	util.LogF("2035 RISC-V Language Server: registered remaining capabilities")
}
