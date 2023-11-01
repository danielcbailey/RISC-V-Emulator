package assembler

type AssembledResult struct {
	Labels            map[string]uint32 // label name to address (relative to start of program)
	LabelTypes        map[string]string // label name to type
	LabelToLineNumber map[string]int    // label name to line number
	AddressToLine     map[uint32]int    // address (relative) to line number
	ProgramText       []uint32
	ProgramData       []uint32
	Diagnostics       []Diagnostic
	fileContents      []string // each line of the file
	FileName          string   // for reflection
	labelLinkRequests []labelLinkRequest
	currentAddress    uint32
	lineLengthDeltas  map[int]int // the number of characters that were added or removed from each line
}

type labelLinkRequest struct {
	labelName string
	address   uint32 // address relative to start of program
	isBranch  bool
}

type EvaluationType int

const (
	EvaluationTypeIntegerLiteral EvaluationType = iota
	EvaluationTypeUnsignedIntegerLiteral
	EvaluationTypeRegister
	EvaluationTypeLabel
)

type EvaluationResult struct {
	// must be an integer
	Value        int64
	Type         EvaluationType
	MatchedValue string // the string that was matched to get this result
}

type TextPosition struct {
	Line int `json:"line"`
	Char int `json:"character"`
}

type TextRange struct {
	Start TextPosition `json:"start"`
	End   TextPosition `json:"end"`
}

type CodeDescription struct {
	URL string `json:"href"`
}

type DiagnosticSeverity int

const (
	Error       DiagnosticSeverity = 1
	Warning     DiagnosticSeverity = 2
	Information DiagnosticSeverity = 3
	Hint        DiagnosticSeverity = 4
)

type Diagnostic struct {
	Range           TextRange          `json:"range"`
	Message         string             `json:"message"`
	Source          string             `json:"source,omitempty"`
	CodeDescription *CodeDescription   `json:"codeDescription,omitempty"`
	Severity        DiagnosticSeverity `json:"severity,omitempty"`
}
