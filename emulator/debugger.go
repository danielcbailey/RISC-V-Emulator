package emulator

import (
	"bufio"
	"bytes"
	"debug/elf"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/assembler"
)

// For live debugging of code, interactive with visual studio code.

type SetBreakpointsRequest struct {
	Source      Source             `json:"source"`
	Breakpoints []SourceBreakpoint `json:"breakpoints"`
}

type CapabilitiesResponse struct {
	SupportsWriteMemoryRequest       bool `json:"supportsWriteMemoryRequest"`
	SupportsReadMemoryRequest        bool `json:"supportsReadMemoryRequest"`
	SupportsTerminateDebuggee        bool `json:"supportsTerminateDebuggee"`
	SupportsConditionalBreakpoints   bool `json:"supportsConditionalBreakpoints"`
	SupportsConfigurationDoneRequest bool `json:"supportsConfigurationDoneRequest"`
	SupportsRestartRequest           bool `json:"supportsRestartRequest"`
	SupportsDataBreakpoints          bool `json:"supportsDataBreakpoints"`
}

type Request struct {
	Seq       int             `json:"seq"`
	Type      string          `json:"type"`
	Command   string          `json:"command"`
	Arguments json.RawMessage `json:"arguments"`
}

type EmptyResponse struct {
}

type ExitEventBody struct {
	ExitCode int `json:"exitCode"`
}

type StoppedEventBody struct {
	Reason        string `json:"reason"`
	Description   string `json:"description"`
	Text          string `json:"text"`
	BreakpointIDs []int  `json:"hitBreakpointIds"`
	ThreadID      int    `json:"threadId"`
}

var liveEmulator *EmulatorInstance
var liveAssembledResult *assembler.AssembledResult
var assembledFilePath string
var continueChan chan bool
var assemblyEntry uint32 = 0
var stdOutBuilder strings.Builder

func RunDebugServer() {
	defer func() {
		if r := recover(); r != nil {
			sendOutput("!!PANIC!! "+fmt.Sprintf("%v\n%v", r, string(debug.Stack())), true)
		}
		panic("debugger server crashed")
	}()

	// uses stdin and out

	contentLength := 0
	reader := bufio.NewReader(os.Stdin)
	for true {
		header, e := reader.ReadString('\r')
		if e != nil {
			return
		}

		fmt.Sscanf(header, "Content-Length: %d", &contentLength)

		// must read contentLength + 3 bytes
		buf := make([]byte, contentLength+3)
		for i := 0; contentLength+3 > i; i++ {
			buf[i], _ = reader.ReadByte()
		}

		jsonData := buf[3:]

		var decodedData Request
		json.Unmarshal(jsonData, &decodedData)

		//sendOutput("got message: "+string(jsonData)+"\n", true)

		messType := decodedData.Type
		if messType == "request" {
			// should always be request
			command := decodedData.Command
			dispatchCommand(command, decodedData.Arguments, decodedData.Seq)
		}
	}
}

func dispatchCommand(command string, data json.RawMessage, seq int) {
	switch command {
	case "initialize":
		handleInitialize(data, seq)
	case "launch":
		handleLaunch(data, seq)
	case "configurationDone":
		handleConfigDone(data, seq)
	case "setBreakpoints":
		handleSetBreakpoints(data, seq)
	case "setExceptionBreakpoints":
		sendResponse("setExceptionBreakpoints", seq, true, EmptyResponse{})
	case "threads":
		handleGetThreads(data, seq)
	case "stackTrace":
		handleGetStacktrace(data, seq)
	case "next":
		handleStepOver(seq)
	case "stepIn":
		handleStepIn(seq)
	case "stepOut":
		handleStepOut(seq)
	case "continue":
		handleContinue(seq)
	case "scopes":
		handleGetScopes(data, seq)
	case "variables":
		handleGetVariables(data, seq)
	case "readMemory":
		handleReadMemory(data, seq)
	case "restart":
		handleRestart(data, seq)
	case "dataBreakpointInfo":
		handleDataBreakpointInfo(data, seq)
	case "setDataBreakpoints":
		handleSetDataBreakpoints(data, seq)
	case "evaluate":
		handleEvaluate(data, seq)
	case "terminate":
		handleTerminate(data, seq)
	case "disconnect":
		handleTerminate(data, seq)
	}
}

func handleInitialize(data json.RawMessage, seq int) {
	// don't really care about the data right now...
	capabilities := CapabilitiesResponse{
		SupportsWriteMemoryRequest:       true,
		SupportsReadMemoryRequest:        true,
		SupportsTerminateDebuggee:        true,
		SupportsConditionalBreakpoints:   true,
		SupportsConfigurationDoneRequest: true,
		SupportsRestartRequest:           true,
		SupportsDataBreakpoints:          true,
	}

	sendResponse("initialize", seq, true, capabilities)
}

func handleLaunch(data json.RawMessage, seq int) {
	var launchInfo map[string]interface{}
	json.Unmarshal(data, &launchInfo)

	sendOutput("Launching RISC-V Emulator with Debugging Enabled", true)

	randomSeed := uint32(0)
	if launchInfo["seed"] != nil {
		randomSeed = uint32(launchInfo["seed"].(float64))
	}

	assignmentPath, _ := launchInfo["assignment"].(string)
	initDebugger(launchInfo["program"].(string), assignmentPath, seq, randomSeed)

	sendResponse("launch", seq, true, EmptyResponse{})
}

func handleRestart(data json.RawMessage, seq int) {
	restartRequest := struct {
		Arguments map[string]interface{} `json:"arguments"`
	}{}

	json.Unmarshal(data, &restartRequest)

	sendOutput("Restarting RISC-V Emulator", true)
	randomSeed := liveEmulator.randomSeed // preserving the seed
	liveEmulator.terminated = true
	continueChan <- true
	time.Sleep(10 * time.Millisecond) // to let the other instance terminate gracefully

	if restartRequest.Arguments["seed"] != nil {
		randomSeed = uint32(restartRequest.Arguments["seed"].(float64))
	}

	assignmentPath, _ := restartRequest.Arguments["assignment"].(string)
	initDebugger(restartRequest.Arguments["program"].(string), assignmentPath, seq, randomSeed)

	sendResponse("restart", seq, true, EmptyResponse{})
}

func handleTerminate(data json.RawMessage, seq int) {
	if liveEmulator == nil {
		return
	}

	sendOutput("Terminating RISC-V Emulator", true)
	liveEmulator.terminated = true
	if continueChan != nil {
		continueChan <- true
	}
	time.Sleep(10 * time.Millisecond) // to let the other instance terminate gracefully
	liveEmulator = nil

	sendResponse("terminate", seq, true, EmptyResponse{})
}

func initDebugger(assemblyPath string, assignmentPath string, seq int, randomSeed uint32) {
	// as part of launching, we need to:
	// load assembly file
	// assemble assembly file
	// load elf file
	// load elf file into memory
	// load assembled code into memory
	// configure emulator
	// initialize breakpoints for debugging
	// start emulator
	// send output to client

	// load assembly file
	fName := assemblyPath
	assembledFilePath = fName
	assignmentFName, hasAssignment := assignmentPath, assignmentPath != ""
	if fName[strings.LastIndex(fName, "."):] != ".asm" {
		sendResponse("launch", seq, false, ErrorBody{Error: ErrorMessage{
			ID:       100,
			Format:   "Invalid File Provided, expected *.asm",
			URL:      "https://www.google.com",
			URLLabel: "Learn More",
		}})
		return
	}

	b, e := os.ReadFile(fName)
	if e != nil {
		sendResponse("launch", seq, false, ErrorBody{Error: ErrorMessage{
			ID:     101,
			Format: "Failed to open the file: " + e.Error(),
		}})
		return
	}

	memoryImage := NewMemoryImage()
	cEnd := uint32(0)
	startAddr := uint32(0)
	globalPointer := uint32(0)
	osCodeStart := uint32(0)
	osCodeEnd := uint32(0)

	if hasAssignment {
		// load elf file
		f, e := elf.Open(assignmentFName)
		if e != nil {
			sendResponse("launch", seq, false, ErrorBody{Error: ErrorMessage{
				ID:     102,
				Format: "Could not open elf file " + assignmentFName + ": " + e.Error(),
			}})
			return
		}

		// load elf file into memory
		sections := f.Sections
		for _, section := range sections {
			if section.Addr == 0 || section.Size == 0 {
				continue // if it doesn't have an address, it's not a section we care about
			}

			if section.Type == elf.SHT_NOBITS {
				// this is a bss section, so we need to allocate memory for it
				for i := uint64(0); i < section.Size; i++ {
					memoryImage.WriteByte(uint32(section.Addr)+uint32(i), 0)
					cEnd = uint32(section.Addr) + uint32(i)
				}
				continue
			}

			// read the section data and write it to memory
			b, e := section.Data()
			if e != nil {
				sendResponse("launch", seq, false, ErrorBody{Error: ErrorMessage{
					ID:     103,
					Format: "Could not read elf section " + section.Name + ": " + e.Error(),
				}})
				return
			}
			for i, v := range b {
				memoryImage.WriteByte(uint32(section.Addr)+uint32(i), v)
				cEnd = uint32(section.Addr) + uint32(i)
			}
		}

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

		osCodeStart = uint32(f.Section(".text").Addr)
		osCodeEnd = uint32(f.Section(".text").Addr) + uint32(f.Section(".text").Size)
	}

	if cEnd != 0 {
		cEnd += 1
		cEnd = (cEnd + 3) & ^uint32(3) // align to 4 bytes
	}

	b, e = os.ReadFile(fName)
	if e != nil {
		log.Fatalf("Could not read assembly file: %v", e)
	}

	// assemble assembly file
	assemblyGlobalPointer := uint32(0)

	assembleRes := assembler.Assemble(string(b))
	numErrors := 0
	for _, diag := range assembleRes.Diagnostics {
		if diag.Severity == assembler.Error {
			numErrors++
		}
	}

	if numErrors > 0 {
		builder := strings.Builder{}
		builder.WriteByte('\n')
		for _, diag := range assembleRes.Diagnostics {
			builder.WriteString(fmt.Sprintf("\t%s:%d:%d: %s\n", filepath.Base(fName), diag.Range.Start.Line+1, diag.Range.Start.Char, diag.Message))
		}

		sendOutput("Could not assemble assembly file: "+builder.String(), true)
		sendResponse("launch", seq, false, ErrorBody{Error: ErrorMessage{
			ID:       104,
			Format:   "Errors occurred while assembling file. Please check output for more details.",
			URL:      "https://www.google.com",
			URLLabel: "Learn More About Debugging",
		}})
		return
	}

	assembleRes.FileName = filepath.Base(fName)
	liveAssembledResult = assembleRes

	// load assembled code into memory
	for i, v := range assembleRes.ProgramText {
		memoryImage.WriteWord(cEnd+uint32(i)*4, v)
	}
	assemblyEntry = cEnd
	assemblyGlobalPointer = cEnd + uint32(len(assembleRes.ProgramText)*4)
	for i, v := range assembleRes.ProgramData {
		memoryImage.WriteWord(assemblyGlobalPointer+uint32(i*4), v)
	}

	// configure emulator
	config := EmulatorConfig{
		StackStartAddress:       0x7FFFFFF0,
		GlobalDataAddress:       assemblyGlobalPointer,
		OSGlobalPointer:         globalPointer,
		HeapStartAddress:        0x10000000,
		Memory:                  memoryImage,
		ProfileIgnoreRangeStart: 0xFFFFFFFF,
		ProfileIgnoreRangeEnd:   0xFFFFFFFF,
		RandomSeed:              randomSeed,
		RuntimeErrorCallback: func(e RuntimeException) {
			sendEvent("stopped", StoppedEventBody{
				Reason:        "exception",
				Description:   e.message,
				Text:          e.message + "\n" + assembleRes.PrettyPrintStacktrace(e.callStack),
				BreakpointIDs: []int{},
				ThreadID:      1,
			})

			continueChan = make(chan bool)
			<-continueChan
		},
		StdOutCallback: func(b byte) {
			if b == '\n' {
				// send output to client
				sendOutput(stdOutBuilder.String(), false)
				stdOutBuilder.Reset()
			} else {
				stdOutBuilder.WriteByte(b)
			}
		},
		RuntimeLimit: 1000000, // 100,000 instructions, which doesn't include the CPP code
	}

	if hasAssignment {
		config.ProfileIgnoreRangeStart = osCodeStart
		config.ProfileIgnoreRangeEnd = osCodeEnd
		config.OSGlobalPointer = globalPointer
	}

	emulator := NewEmulator(config)
	emulator.breakCallback = func(inst *EmulatorInstance, breakpointID int, reason string) {
		eventBody := StoppedEventBody{
			Reason:        reason,
			ThreadID:      1,
			Description:   "Paused on breakpoint.",
			BreakpointIDs: []int{breakpointID},
		}

		if breakpointID == 0 {
			eventBody.Reason = "step"
			eventBody.Description = ""
			eventBody.BreakpointIDs = []int{}
		}

		sendEvent("stopped", eventBody)
		continueChan = make(chan bool)
		<-continueChan
	}

	// start emulator - only if there is an assignment because we need to wait for the configuration to complete before assembly code can be run
	if hasAssignment {
		emulator.Emulate(startAddr)
		config.GlobalDataAddress = assemblyGlobalPointer
		emulator.ResetRegisters(config)
	}

	liveEmulator = emulator
	sendEvent("initialized", EmptyResponse{})
}

func handleConfigDone(data json.RawMessage, seq int) {
	emulateFunc := func() {
		sendOutput("Emulation Started", true)
		emulator := liveEmulator
		emulator.Emulate(assemblyEntry) // pc will be set by the launch code above

		// sending seed
		sendEvent("riscv_context", map[string]interface{}{
			"seed": emulator.randomSeed,
		})
		sendScreenUpdates()

		if emulator.terminated {
			return
		}

		sendOutput(fmt.Sprintf("Emulation Completed.\nDI = %d, SI=%d, Register Usage = %d, Memory Usage = %d", emulator.di, len(liveAssembledResult.ProgramText), emulator.regUsage, int(emulator.memUsage)+len(liveAssembledResult.ProgramData)), true)
		sendEvent("exited", ExitEventBody{
			ExitCode: 0,
		})
		sendEvent("terminated", EmptyResponse{})
	}

	go emulateFunc()
	sendResponse("configurationDone", seq, true, EmptyResponse{})
}

func handleGetThreads(data json.RawMessage, seq int) {
	type Thread struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	threadsRespBody := struct {
		Threads []Thread `json:"threads"`
	}{Threads: []Thread{{
		ID:   1,
		Name: "Main",
	}}}
	sendResponse("threads", seq, true, threadsRespBody)
}

func handleStepOver(seq int) {
	instruction := liveEmulator.memReadWord(liveEmulator.pc, true)

	// decoding instruction
	opcode := assembler.GetOpCode(instruction)

	switch opcode {
	case assembler.OPCODE_JAL:
		_, rd, _ := assembler.DecodeJTypeInstruction(instruction)

		if rd == 0 {
			liveEmulator.breakNext = true
		} else {
			liveEmulator.breakAddr = liveEmulator.pc + 4
		}

		break

	case assembler.OPCODE_JALR:
		liveEmulator.breakAddr = liveEmulator.pc + 4
	default:
		liveEmulator.breakNext = true
	}

	if continueChan != nil {
		continueChan <- true
	}

	sendResponse("next", seq, true, EmptyResponse{})
}

func handleStepIn(seq int) {
	liveEmulator.breakNext = true
	if continueChan != nil {
		continueChan <- true
	}
	sendResponse("stepIn", seq, true, EmptyResponse{})
}

func handleStepOut(seq int) {
	if len(liveEmulator.callStack) < 1 {
		sendOutput("Not currently in a function call; cannot Step Out.", true)
	} else {
		// get line of last callstack frame
		liveEmulator.breakAddr = liveEmulator.callStack[len(liveEmulator.callStack)-1] + 4
		if continueChan != nil {
			continueChan <- true
		}
		sendResponse("stepOut", seq, true, EmptyResponse{})
	}
}

func handleContinue(seq int) {
	if continueChan != nil {
		continueChan <- true
	}
	sendResponse("continue", seq, true, EmptyResponse{})
}

var stackFrameIDCounter = 0

func handleGetStacktrace(data json.RawMessage, seq int) {
	// called whenever the debugger stops
	// so can send over misc. updates
	sendScreenUpdates()

	trace := liveEmulator.callStack

	// building the stack frames
	stackFrames := make([]StackFrame, len(trace)+1)
	for i, v := range trace {
		stackFrames[len(trace)-i].ID = stackFrameIDCounter
		stackFrames[len(trace)-i].Name = liveAssembledResult.GetTextLabelForAddress(v)
		stackFrames[len(trace)-i].Line = liveAssembledResult.GetLineOfAddress(v, assemblyEntry)
		stackFrames[len(trace)-i].Source = Source{
			Name: liveAssembledResult.FileName,
			Path: assembledFilePath,
		}
		stackFrames[len(trace)-i].addr = v
		stackFrameIDCounter++
	}

	// add the current instruction
	stackFrames[0].ID = stackFrameIDCounter
	stackFrames[0].Name = liveAssembledResult.GetTextLabelForAddress(liveEmulator.pc)
	stackFrames[0].Line = liveAssembledResult.GetLineOfAddress(liveEmulator.pc, assemblyEntry)
	stackFrames[0].Source = Source{
		Name: liveAssembledResult.FileName,
		Path: assembledFilePath,
	}
	stackFrames[len(stackFrames)-1].addr = liveEmulator.pc
	stackFrameIDCounter++

	stackTrace := struct {
		StackFrames []StackFrame `json:"stackFrames"`
	}{StackFrames: stackFrames}

	sendResponse("stackTrace", seq, true, stackTrace)
}

var breakpointIDCounter = 1

func handleSetBreakpoints(data json.RawMessage, seq int) {
	reqBody := SetBreakpointsRequest{}
	json.Unmarshal(data, &reqBody)

	if liveEmulator == nil {
		sendResponse("setBreakpoints", seq, false, ErrorBody{Error: ErrorMessage{
			ID:     105,
			Format: "No emulator is running to add breakpoints to.",
		}})
		return
	}

	if reqBody.Source.Name != liveAssembledResult.FileName {
		// don't actually add the breakpoints but don't error either
		sendResponse("setBreakpoints", seq, true, struct {
			Breakpoints []Breakpoint `json:"breakpoints"`
		}{Breakpoints: []Breakpoint{}})
		return
	}

	liveEmulator.RemoveAllBreakpoints()

	// extracting the address for each breakpoint
	breakpoints := make([]Breakpoint, len(reqBody.Breakpoints))
	for i, v := range reqBody.Breakpoints {
		breakpoints[i].ID = breakpointIDCounter
		breakpoints[i].Line = v.Line
		breakpoints[i].Source = reqBody.Source
		breakpoints[i].condition = v.Condition

		breakpointIDCounter++

		// find the line in the assembled result
		addr := liveAssembledResult.GetAddressOfLine(v.Line)
		if addr == 0xFFFFFFFF {
			// no address found
			breakpoints[i].Verified = false
			breakpoints[i].Message = "No instruction found on this line."
			continue
		}

		breakpoints[i].Verified = true
		breakpoints[i].addr = addr + assemblyEntry

		// add the breakpoint to the emulator
		liveEmulator.AddBreakpoint(addr+assemblyEntry, breakpoints[i])
	}

	breakpointRespBody := struct {
		Breakpoints []Breakpoint `json:"breakpoints"`
	}{Breakpoints: breakpoints}
	sendResponse("setBreakpoints", seq, true, breakpointRespBody)
}

func handleGetScopes(data json.RawMessage, seq int) {
	scopesRequest := struct {
		FrameID int `json:"frameId"`
	}{}

	json.Unmarshal(data, &scopesRequest)

	// for now only doing a register scope
	scopes := []Scope{{
		Name:               "registers",
		PresentationHint:   "registers",
		VariablesReference: 32,
	}, {
		Name:               "memory",
		PresentationHint:   "memory",
		VariablesReference: 500,
	}}

	scopesResponse := struct {
		Scopes []Scope `json:"scopes"`
	}{Scopes: scopes}

	sendResponse("scopes", seq, true, scopesResponse)
}

func handleReadMemory(data json.RawMessage, seq int) {
	request := struct {
		MemoryReference string `json:"memoryReference"` // memory reference to base location to read data
		Offset          int    `json:"offset"`          // offset in bytes to memory reference
		Count           int    `json:"count"`           // number of bytes to read at given location
	}{}
	json.Unmarshal(data, &request)

	response := struct {
		Address         string `json:"address"`         // address of first byte
		UnreadableBytes string `json:"unreadableBytes"` // unreadable bytes after last successfully read byte
		Data            string `json:"data"`            // resulting bytes encoded in base64
	}{}

	// ignoring edge cases for now
	response.Address = "0"
	response.UnreadableBytes = ""
	response.Data = ""

	blockVal, ok := liveEmulator.memory.Blocks[uint32(request.Offset>>12)]

	// Pages are lazily loaded, so unless a stack operation has occurred we can't guarantee the memory exists
	if !ok {
		sendResponse("readMemory", seq, true, response)
		return
	}

	block := blockVal.Block

	buf := new(bytes.Buffer)

	start := (request.Offset & 0xFFF) >> 2
	end := (start + request.Count)

	if end >= len(block) {
		end = len(block) - 1
	}

	// encode block into base64
	for i := start; i <= end; i++ {
		binary.Write(buf, binary.BigEndian, block[i])
	}

	bufBytes := buf.Bytes()

	base64Encoded := base64.StdEncoding.EncodeToString(bufBytes)

	response.Data = base64Encoded

	sendResponse("readMemory", seq, true, response)
}

func handleGetVariables(data json.RawMessage, seq int) {
	variablesRequest := struct {
		VariablesReference int `json:"variablesReference"`
	}{}

	json.Unmarshal(data, &variablesRequest)

	var variables []Variable
	if variablesRequest.VariablesReference == 32 {
		// getting the list of all variables
		regsMap := liveEmulator.lastUsedRegisters

		// converting the regsMap to a slice of register numbers sorted from least to greatest
		regs := make([]int, len(regsMap))
		i := 0
		//sendOutput("number used registers: "+strconv.Itoa(len(regsMap))+"\n", true)
		for k := range regsMap {
			regs[i] = k
			i++
		}

		// sorting the regs slice
		sort.Slice(regs, func(i, j int) bool {
			return regs[i] < regs[j]
		})

		variables = make([]Variable, len(regs))

		// building the variables that are returned to the client
		for i, v := range regs {
			attributes := []string{}
			if v == 0 {
				attributes = append(attributes, "readOnly", "constant")
			}

			friendlyName := ""
			for fName, nName := range assembler.RegisterNameMap {
				if v == nName && fName[0] != 'x' {
					friendlyName = fName
					break
				}
			}
			variables[i] = Variable{
				Name:               fmt.Sprintf("x%d (%s)", v, friendlyName),
				EvaluateName:       fmt.Sprintf("x%d", v),
				Value:              fmt.Sprintf("%d (0x%X)", int32(liveEmulator.registers[v]), liveEmulator.registers[v]),
				VariablesReference: v,
				Type:               "int32_t",
				PresentationHint: VariablePresentationHint{
					Kind:       "data",
					Attributes: attributes,
				},
			}
		}
	} else if variablesRequest.VariablesReference < 32 {
		// getting the list of variables for a specific register
		regNum := variablesRequest.VariablesReference
		regVal := liveEmulator.registers[regNum]

		variables = []Variable{{
			Name:  "Value",
			Value: fmt.Sprintf("%d", int32(regVal)),
			Type:  "int32_t",
			PresentationHint: VariablePresentationHint{
				Kind: "data",
			},
		}, {
			Name:  "Hex Value",
			Value: fmt.Sprintf("0x%X", regVal),
			Type:  "int32_t",
			PresentationHint: VariablePresentationHint{
				Kind: "data",
			},
		}}
	}

	variablesResponse := struct {
		Variables []Variable `json:"variables"`
	}{Variables: variables}

	sendResponse("variables", seq, true, variablesResponse)
}

func handleDataBreakpointInfo(data json.RawMessage, seq int) {
	request := struct {
		VariablesReference int    `json:"variablesReference"`
		Name               string `json:"name"`
	}{}

	json.Unmarshal(data, &request)

	response := struct {
		DataId      string `json:"dataId"`
		Description string `json:"description"`
	}{}

	response.DataId = strconv.Itoa(request.VariablesReference)
	if request.VariablesReference < 32 {
		response.Description = fmt.Sprintf("Watchpoint on register x%d", request.VariablesReference)
	} else if request.VariablesReference >= 2035 {
		response.Description = fmt.Sprintf("Watchpoint on variable %s", request.Name)
	} else {
		response.DataId = "0"
	}

	sendResponse("dataBreakpointInfo", seq, true, response)
}

func handleSetDataBreakpoints(data json.RawMessage, seq int) {
	request := struct {
		Breakpoints []DataBreakpoint `json:"breakpoints"`
	}{}

	json.Unmarshal(data, &request)

	liveEmulator.RemoveAllMemoryBreakpoints()
	liveEmulator.RemoveAllRegisterBreakpoints()
	breakpoints := make([]Breakpoint, len(request.Breakpoints))

	for i, v := range request.Breakpoints {
		dataID, _ := strconv.Atoi(v.DataID)

		breakpoint := Breakpoint{
			ID:       breakpointIDCounter,
			Verified: true,
			//condition: v.Condition,
		}

		breakpointIDCounter++

		if dataID < 32 {
			// register watchpoint
			liveEmulator.AddRegisterBreakpoint(dataID, breakpoint)
		} else {
			// variable watchpoint
			liveEmulator.AddMemoryBreakpoint(uint32(dataID-2035), breakpoint) // why not make it 2035?
		}

		breakpoints[i] = breakpoint
	}

	response := struct {
		Breakpoints []Breakpoint `json:"breakpoints"`
	}{Breakpoints: breakpoints}

	sendResponse("setDataBreakpoints", seq, true, response)
}

func handleEvaluate(data json.RawMessage, seq int) {
	request := struct {
		Expression string `json:"expression"`
	}{}

	json.Unmarshal(data, &request)

	res, err := EvaluateExpression(request.Expression)
	if err != nil {
		sendResponse("evaluate", seq, false, ErrorBody{Error: ErrorMessage{
			ID:     106,
			Format: err.Error(),
		}})
		return
	}

	response := struct {
		Result          string `json:"result"`
		MemoryReference string `json:"memoryReference"`
	}{Result: res.String}

	if res.isValidAddress {
		response.MemoryReference = strconv.FormatUint(uint64(res.address)+2035, 10)
	} else if res.isRegister {
		response.MemoryReference = strconv.Itoa(int(res.address))
	}

	sendResponse("evaluate", seq, true, response)
}

var seqCounter = 1

func sendOutput(str string, isDebugger bool) {
	outputEvt := OutputEventBody{Category: "stdout", Output: str + "\n\r"}
	if isDebugger {
		outputEvt.Category = "console"
	}

	sendEvent("output", outputEvt)
}

func sendResponse(command string, reqSeq int, success bool, body interface{}) {
	fullResp := Response{Command: command, Body: body, Type: "response", Seq: seqCounter, RequestSeq: reqSeq, Success: success}
	seqCounter++
	// b, _ := json.Marshal(fullResp)
	// sendOutput("reponse: "+string(b)+"\n", true)
	sendToClient(fullResp)
}

func sendEvent(evtType string, evt interface{}) {
	fullEvt := Event{Event: evtType, Body: evt, Type: "event", Seq: seqCounter}
	seqCounter++
	sendToClient(fullEvt)
}

func sendToClient(data interface{}) {
	b, _ := json.Marshal(data)
	fmt.Print("Content-Length: " + strconv.Itoa(len(b)) + "\r\n\r\n" + string(b))
}

func getMemoryBase64(offset int, inverted bool, blockCount int) string {
	count := 1024

	initialBlockIndex := uint32(offset >> 12)

	buf := new(bytes.Buffer)

	for i := 0; i < blockCount; i++ {
		blockVal, ok := liveEmulator.memory.Blocks[uint32(i)+initialBlockIndex]

		if !ok {
			sendOutput("Failed to read gp memory.", true)
			return ""
		}

		block := blockVal.Block

		start := 0

		if i == 0 {
			start = (offset & 0xFFF) >> 2
		}

		end := (start + count)

		if inverted && end > 1020 {
			end = 1020
		}

		if end >= len(block) {
			end = len(block) - 1
		}

		// encode block into base64
		for j := start; j <= end; j++ {
			binary.Write(buf, binary.BigEndian, block[j])
		}
	}

	bufBytes := buf.Bytes()

	base64Encoded := base64.StdEncoding.EncodeToString(bufBytes)

	return base64Encoded
}

func sendScreenUpdates() {
	statusString := "running"
	if liveEmulator.solutionValidity == 1 {
		statusString = "failed"
	} else if liveEmulator.solutionValidity == 2 {
		statusString = "passed"
	} else if liveEmulator.pc == 0x20352035 || liveEmulator.pc == 0x20352034 {
		// magic number to end the emulator
		statusString = "finished"
	}

	type ScreenUpdate struct {
		Width   int                    `json:"width"`
		Height  int                    `json:"height"`
		Updates []VirtualDisplayUpdate `json:"updates"`
		Status  string                 `json:"status"`
		Stats   map[string]int         `json:"stats"`
		Memory  map[string]string      `json:"memory"`
	}

	// Get current gp memory and stack memory
	// for memory: start at gp
	// for stack: magic number 0x7FFFFFF0
	gp := liveEmulator.registers[3]
	sp := liveEmulator.registers[2]

	mainMemory := getMemoryBase64(int(gp), false, 2)
	stackMemory := getMemoryBase64(int(sp), true, 1)

	packet := ScreenUpdate{
		Width:   liveEmulator.display.width,
		Height:  liveEmulator.display.height,
		Updates: liveEmulator.display.GetEntireScreen(),
		Status:  statusString,
		Stats: map[string]int{
			"di":        int(liveEmulator.di),
			"mem":       int(liveEmulator.memUsage) + len(liveAssembledResult.ProgramData),
			"allocated": int(len(liveAssembledResult.ProgramData)),
			"reg":       int(liveEmulator.regUsage),
			"si":        len(liveAssembledResult.ProgramText),
			"pc":        int(liveEmulator.pc),
		},
		Memory: map[string]string{
			"main":  mainMemory,
			"stack": stackMemory,
		},
	}

	sendEvent("riscv_screen", packet)
	//sendOutput(fmt.Sprintf("PC: %d", int(liveEmulator.pc)), true)
	//sendOutput("sent screen update!", true)
}
