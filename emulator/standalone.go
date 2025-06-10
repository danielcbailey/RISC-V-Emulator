package emulator

import (
	"debug/elf"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/assembler"
)

// The emulator normally interfaces with VSCode for stdout and its virtual display, but for development,
// there needs to be a way to run the emulator on cpp code without VSCode. This file contains the code
// to run the emulator without VSCode. To provide the peripheral support, this will host a web server on
// port 2035 that will serve the virtual display, mouse, keyboard, and console.
func runStandaloneEmulator(elfFilePath string, assemblyPath string, conn *websocket.Conn, emInst **EmulatorInstance) {
	fmt.Println("Running standalone emulator...")
	f, e := elf.Open(elfFilePath)
	if e != nil {
		log.Fatalf("Could not open elf file %s: %v", elfFilePath, e)
	}

	memoryImage := NewMemoryImage()

	sections := f.Sections
	cEnd := uint32(0)
	for _, section := range sections {
		if section.Addr == 0 {
			continue // if it doesn't have an address, it's not a section we care about
		}

		// read the section data and write it to memory
		b, e := section.Data()
		if e != nil {
			log.Fatalf("Could not read section %s: %v", section.Name, e)
		}
		for i, v := range b {
			memoryImage.WriteByte(uint32(section.Addr)+uint32(i), v)
			cEnd = uint32(section.Addr) + uint32(i)
		}
	}

	startAddr := uint32(0)
	globalPointer := uint32(0)
	symbols, e := f.Symbols()
	if e != nil {
		log.Fatalf("Could not read symbols: %v", e)
	}
	for _, symbol := range symbols {
		if symbol.Name == "_start" {
			startAddr = uint32(symbol.Value)
		} else if symbol.Name == "__global_pointer$" {
			globalPointer = uint32(symbol.Value)
		}
	}

	assemblyEntry := uint32(0)
	assemblyGlobalPointer := uint32(0)
	if assemblyPath != "" {
		b, e := os.ReadFile(assemblyPath)
		if e != nil {
			log.Fatalf("Could not read assembly file: %v", e)
		}

		assembleRes := assembler.Assemble(string(b))
		if len(assembleRes.Diagnostics) > 0 {
			builder := strings.Builder{}
			builder.WriteByte('\n')
			for _, diag := range assembleRes.Diagnostics {
				builder.WriteString(fmt.Sprintf("\t%s:%d:%d: %s\n", filepath.Base(assemblyPath), diag.Range.Start.Line+1, diag.Range.Start.Char, diag.Message))
			}

			log.Printf("Could not assemble assembly file: %s\n", builder.String())
			conn.WriteJSON(struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{Type: "console", Text: fmt.Sprintf("Could not assemble assembly file: %s\n", builder.String())})
			return
		}

		cEnd += 1
		cEnd = (cEnd + 3) & ^uint32(3) // align to 4 bytes
		for i, v := range assembleRes.ProgramText {
			memoryImage.WriteWord(cEnd+uint32(i)*4, v)
		}
		assemblyEntry = cEnd
		assemblyGlobalPointer = cEnd + uint32(len(assembleRes.ProgramText)*4)
		for i, v := range assembleRes.ProgramData {
			memoryImage.WriteWord(assemblyGlobalPointer+uint32(i*4), v)
		}
	}

	wsMutex := sync.Mutex{}
	// create the emulator
	config := EmulatorConfig{
		StackStartAddress:       0x7FFFFFF0,
		GlobalDataAddress:       globalPointer,
		OSGlobalPointer:         globalPointer,
		HeapStartAddress:        0x10000000,
		Memory:                  memoryImage,
		ProfileIgnoreRangeStart: uint32(f.Section(".text").Addr),
		ProfileIgnoreRangeEnd:   uint32(f.Section(".text").Addr) + uint32(f.Section(".text").Size),
		RuntimeErrorCallback: func(e RuntimeException) {
			log.Fatalf("Runtime exception: %s", e.message)
		},
		StdOutCallback: func(b byte) {
			consoleMessage := struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{Type: "console", Text: string(b)}

			messageBytes, e := json.Marshal(consoleMessage)
			if e != nil {
				log.Fatalf("Could not marshal console message: %v", e)
			}

			wsMutex.Lock()
			conn.WriteMessage(websocket.TextMessage, messageBytes)
			wsMutex.Unlock()
		},
		RuntimeLimit: 1000000, // 1,000,000 instructions, which doesn't include the CPP code
	}

	emulator := NewEmulator(config)
	*emInst = emulator

	displayWatcher := func() {
		prevWrites := int64(0)
		for !emulator.terminated {
			time.Sleep(250 * time.Millisecond)
			if prevWrites != emulator.display.displayWrites {
				prevWrites = emulator.display.displayWrites

				// send the display data
				displayMessage := struct {
					Type   string `json:"type"`
					Data   string `json:"data"`
					Width  int    `json:"width"`
					Height int    `json:"height"`
				}{Type: "display", Width: emulator.display.width, Height: emulator.display.height}

				data := emulator.display.data
				dBytes := make([]byte, 0, len(data)*4)
				// converting data to bytes by breaking up the uint32s into 4 bytes
				for i := 0; i < emulator.display.width*emulator.display.height; i++ {
					dBytes = append(dBytes, byte(data[i]))
					dBytes = append(dBytes, byte(data[i]>>8))
					dBytes = append(dBytes, byte(data[i]>>16))
					dBytes = append(dBytes, byte(data[i]>>24))
				}

				displayMessage.Data = base64.StdEncoding.EncodeToString(dBytes)

				messageBytes, e := json.Marshal(displayMessage)
				if e != nil {
					log.Fatalf("Could not marshal display message: %v", e)
				}

				wsMutex.Lock()
				conn.WriteMessage(websocket.TextMessage, messageBytes)
				wsMutex.Unlock()
			}
		}
	}
	go displayWatcher()

	emulator.Emulate(startAddr)

	if assemblyEntry != 0 {
		config.GlobalDataAddress = assemblyGlobalPointer
		emulator.ResetRegisters(config)
		emulator.Emulate(assemblyEntry)
		conn.WriteJSON(struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{Type: "console", Text: fmt.Sprintf("Emulator completed DI=%d\n", emulator.GetDynamicInstructionCount())})
		fmt.Printf("Emulator completed.\n")
	} else {
		fmt.Printf("Emulator completed with exit code %d\n", emulator.GetExitCode())
	}

	time.Sleep(100 * time.Millisecond)
	fmt.Printf("Emulator ran %d instructions\n", emulator.GetTotalInstructionsExecuted())
}

func RunStandaloneWebserver(elfFilePath string, assemblyPath string) {
	// open a websocket on port 2035 and listen for commands
	// commands will be:
	// - run: run the emulator with the given elf file and assembly file
	// - stop: stop the emulator
	// - keyboard: send keyboard input to the emulator
	// - mouse: send mouse input to the emulator

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}

		var emInst *EmulatorInstance

		// listen on conn for messages
		for {
			// read in a message
			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				if emInst != nil {
					emInst.Terminate()
				}
				break
			}

			message := make(map[string]interface{})
			err = json.Unmarshal(messageBytes, &message)
			if err != nil {
				log.Println("json:", err)
				break
			}

			mType := message["type"].(string)
			switch mType {
			case "run":
				go runStandaloneEmulator(elfFilePath, assemblyPath, conn, &emInst)
			case "stop":
				if emInst != nil {
					emInst.Terminate()
				}
			case "keyboard":
				// TODO

			case "mouse":
				// TODO

			default:
				log.Printf("Unknown message type: %s", mType)
			}
		}
	}

	http.HandleFunc("/ws", handler)
	http.HandleFunc("/", handleGetPage)
	log.Println("Connect to the emulator at http://localhost:2035")
	http.ListenAndServe(":2035", nil)
}

func handleGetPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(htmlPage))
}

var htmlPage = `<html>
<head>
	<title>RISCV Emulator</title>
</head>
<body style="background-color: #1E1E1E;">
	<h1 style="color: white; display: inline-block;">RISCV Emulator</h1>
	<button id="runButton" style="margin-left: 50px; height: 40px; width: 80px;">RUN</button>
	<br/>
	<canvas width="1000px" height="700px" style="border: 2px solid white;" id="display"></canvas>
	<h2 style="color: white;">Console</h2>
	<div style="width: 980px; padding: 10px; color: white; font-size: 1.2em; font-family: monospace; background-color: black; height: 300px; overflow-y: 'auto'; overflow-x: 'wrap'; border: 2px solid white;" id="console"></div>

	<script>
		// Connect to the websocket
		var socket = new WebSocket("ws://localhost:2035/ws");

		var screenWidth = 1000;
		var screenHeight = 700;

		var consoleText = "";

		// When the socket is opened, listen for messages
		socket.onopen = function() {
			socket.onmessage = function(event) {
				var data = JSON.parse(event.data);
				if (data.type == "console") {
					consoleText += data.text.replaceAll("\n", "<br/>");
					document.getElementById("console").innerHTML = consoleText;
				} else if (data.type == "display") {
					let canvas = document.getElementById("display");

					let dataWidth = data.width;
					let dataHeight = data.height;

					// if the canvas is not the same size as the data, resize it
					if (canvas.width != dataWidth || canvas.height != dataHeight) {
						canvas.width = dataWidth;
						canvas.height = dataHeight;
					}

					var raw = window.atob(data.data);
					var rawLength = raw.length;
					var array = new Uint8Array(new ArrayBuffer(rawLength));

					for(i = 0; i < rawLength; i++) {
						array[i] = raw.charCodeAt(i);
					}

					var ctx = canvas.getContext("2d");
					var imageData = ctx.createImageData(dataWidth, dataHeight);
					for (var i = 0; i < dataWidth * dataHeight * 4; i++) {
						imageData.data[i * 4 + 0] = array[i * 4 + 0]; // red
						imageData.data[i * 4 + 1] = array[i * 4 + 1]; // green
						imageData.data[i * 4 + 2] = array[i * 4 + 2]; // blue
						imageData.data[i * 4 + 3] = array[i * 4 + 3]; // alpha
					}
					ctx.putImageData(imageData, 0, 0);
				}
			};
		};

		// when the socket closes, try to reconnect every 3 seconds
		socket.onclose = function() {
			setTimeout(function() {
				socket = new WebSocket("ws://localhost:2035/ws");
			}, 3000);
		};

		// when the run button is clicked, send a message to the emulator to start running
		document.getElementById("runButton").onclick = function() {
			consoleText = "";
			socket.send(JSON.stringify({
				type: "run"
			}));
		};

	</script>
</body>
</html>`
