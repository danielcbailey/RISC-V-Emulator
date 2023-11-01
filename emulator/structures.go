package emulator

import "sync"

type MemoryPage struct {
	Block       [1024]uint32
	StartAddr   uint32
	Initialized [1024]bool
}

type MemoryImage struct {
	Blocks map[uint32]*MemoryPage
}

type EmulatorConfig struct {
	GlobalDataAddress       uint32
	OSGlobalPointer         uint32
	StackStartAddress       uint32
	HeapStartAddress        uint32
	Memory                  *MemoryImage
	ProfileIgnoreRangeStart uint32
	RuntimeLimit            uint32
	ProfileIgnoreRangeEnd   uint32
	RuntimeErrorCallback    func(RuntimeException)
	StdOutCallback          func(byte)
	RandomSeed              uint32
}

type RuntimeException struct {
	regs      [32]uint32
	pc        uint32
	callStack []uint32
	message   string
}

type VirtualDisplay struct {
	data            [2097152]uint32
	updateRegions   [8200]bool // for each 16x16 pixel group, whether it has been updated
	dataMutex       sync.Mutex
	width           int
	height          int
	shapeDrawParams [4]uint32
	displayWrites   int64
}

type VirtualFileSystem struct {
	data []uint32
}

type Interrupt struct {
	ID   uint32
	Data []uint32

	// context from before the interrupt
	registers [32]uint32
	pc        uint32
	callStack []uint32
}

type EmulatorInstance struct {
	registers               [32]uint32
	memory                  *MemoryImage
	pc                      uint32
	regInit                 uint32
	iCache                  *MemoryPage
	dCache                  *MemoryPage
	runtimeLimit            uint32 // Note: this does not apply inside the profile ignore range
	osEntry                 uint32
	osGlobalPointer         uint32
	osInterruptHandlerEntry uint32
	executedInstructions    uint64
	userGlobalPointer       uint32
	isInOSCode              bool
	exitCode                int
	heapPointer             uint32 // incremented as additional sbrks are called
	wasEcall                bool   // signals that modified registers must be writen to
	oldFramePointer         uint32
	registerPreservation    [32]uint32

	// assignments
	randomSeed       uint32
	solutionValidity uint32

	// peripherals
	display   *VirtualDisplay
	fs        *VirtualFileSystem
	interrupt *Interrupt

	// statistics
	profileIgnoreRangeStart uint32
	profileIgnoreRangeEnd   uint32
	di                      uint32
	memUsage                uint32
	regUsage                uint32
	errors                  []RuntimeException

	// debugging
	callStack            []uint32
	breakpoints          map[uint32]Breakpoint
	registerBreakpoints  map[int]Breakpoint
	memoryBreakpoints    map[uint32]Breakpoint
	breakAddr            uint32 // for step over and step out
	breakNext            bool   // for step into
	stdOutCallback       func(byte)
	runtimeErrorCallback func(RuntimeException)
	breakCallback        func(*EmulatorInstance, int, string) // int is breakpoint ID, string is reason
	terminated           bool
	lastUsedRegisters    map[int]int // we only want a set, but go only exposes a map so we have the superfluous int value
}

// Debugging
type Source struct {
	Name             string `json:"name"`
	Path             string `json:"path"`
	SourceReference  int    `json:"sourceReference"`
	PresentationHint string `json:"presentationHint"` // should be normal for all assembled code (deemphasize for loaded binary)
}

type Breakpoint struct {
	ID                   int    `json:"id"`
	Verified             bool   `json:"verified"`
	Message              string `json:"message"` // leave empty for valid breakpoints
	Source               Source `json:"source"`
	Line                 int    `json:"line"`
	InstructionReference string `json:"instructionReference"` // a label
	Offset               int    `json:"offset"`
	addr                 uint32
	hits                 uint32
	condition            string
	hitCount             int // must be positive and non-zero
}

type DataBreakpoint struct {
	DataID string `json:"dataId"`
	//Condition string `json:"condition"`
}

type SourceBreakpoint struct {
	Line         int    `json:"line"`
	Condition    string `json:"condition"`
	HitCondition string `json:"hitCondition"`
}

type StackFrame struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Line   int    `json:"line"`
	Column int    `json:"column"` // always zero...
	Source Source `json:"source"`
	addr   uint32
}

type Event struct {
	Event string      `json:"event"`
	Type  string      `json:"type"`
	Seq   int         `json:"seq"`
	Body  interface{} `json:"body"`
}

type Response struct {
	Command    string      `json:"command"`
	Type       string      `json:"type"`
	Seq        int         `json:"seq"`
	RequestSeq int         `json:"request_seq"`
	Body       interface{} `json:"body"`
	Success    bool        `json:"success"`
}

type ErrorMessage struct {
	ID       int    `json:"id"`
	Format   string `json:"format"`
	URL      string `json:"url,omitempty"`
	URLLabel string `json:"urlLabel,omitempty"`
}

type ErrorBody struct {
	Error ErrorMessage `json:"error"`
}

type OutputEventBody struct {
	Category string `json:"category"`
	Output   string `json:"output"`
}

type Scope struct {
	Name               string `json:"name"`
	PresentationHint   string `json:"presentationHint"` // should have same value as name
	VariablesReference int    `json:"variablesReference"`
}

type VariablePresentationHint struct {
	Kind       string   `json:"kind"`
	Attributes []string `json:"attributes,omitempty"`
}

type Variable struct {
	Name               string                   `json:"name"`
	EvaluateName       string                   `json:"evaluateName,omitempty"`
	Value              string                   `json:"value"`
	VariablesReference int                      `json:"variablesReference"`
	Type               string                   `json:"type"`
	PresentationHint   VariablePresentationHint `json:"presentationHint"`
}
