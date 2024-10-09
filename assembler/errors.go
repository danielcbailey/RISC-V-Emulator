package assembler

import (
	"math"
	"strconv"
)

func AdjustRange(r TextRange, errorText string) (TextRange, string) {
	// Removes the leading and training whitespace from the error text, and adjusts the range accordingly
	text := errorText
	for len(text) > 0 && text[0] == ' ' {
		text = text[1:]
		r.Start.Char += 1
	}

	for len(text) > 0 && text[len(text)-1] == ' ' {
		text = text[:len(text)-1]
		r.End.Char -= 1
	}

	return r, text
}

// Errors
type assemblyError struct{}

var Errors assemblyError

func (assemblyError) InvalidDataSectionValue(value string, r TextRange) Diagnostic {
	r, value = AdjustRange(r, value)
	return Diagnostic{
		Range:    r,
		Message:  "Invalid data section value: \"" + value + "\"",
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) InvalidDataSection(sectionType string, r TextRange) Diagnostic {
	return Diagnostic{
		Range:    r,
		Message:  "Invalid data section: \"" + sectionType + "\"",
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) InvalidSymbolName(symbolName, context string, r TextRange) Diagnostic {
	r, symbolName = AdjustRange(r, symbolName)
	return Diagnostic{
		Range:    r,
		Message:  "Invalid symbol name: \"" + symbolName + "\", " + context,
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) UnresolvedSymbolName(symbolName string, r TextRange) Diagnostic {
	r, symbolName = AdjustRange(r, symbolName)
	return Diagnostic{
		Range:    r,
		Message:  "Unresolved symbol name: \"" + symbolName + "\", ",
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) InvalidIntegerLiteral(literal string, r TextRange) Diagnostic {
	r, literal = AdjustRange(r, literal)
	return Diagnostic{
		Range:    r,
		Message:  "Expected integer literal, got: \"" + literal + "\"",
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) InvalidUnsignedIntegerLiteral(literal string, r TextRange) Diagnostic {
	r, literal = AdjustRange(r, literal)
	return Diagnostic{
		Range:    r,
		Message:  "Expected unsigned integer literal, got: \"" + literal + "\"",
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) InvalidRegister(register string, r TextRange) Diagnostic {
	r, register = AdjustRange(r, register)
	return Diagnostic{
		Range:    r,
		Message:  "Expected register, got: \"" + register + "\"",
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) ImmediateOverflow(value string, maxSize int, r TextRange) Diagnostic {
	r, value = AdjustRange(r, value)
	return Diagnostic{
		Range:    r,
		Message:  "Immediate value \"" + value + "\" is out of range of " + strconv.Itoa(maxSize) + " bits [-" + strconv.Itoa(int(math.Pow(2, float64(maxSize-1)))) + ", " + strconv.Itoa(int(math.Pow(2, float64(maxSize-1)))) + ")",
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) UnsignedImmediateOverflow(value string, maxSize int, r TextRange) Diagnostic {
	r, value = AdjustRange(r, value)
	return Diagnostic{
		Range:    r,
		Message:  "Immediate value \"" + value + "\" is too large. Must be less than " + strconv.Itoa(maxSize) + " bits (" + strconv.Itoa(int(math.Pow(2, float64(maxSize)))) + ")",
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) InvalidInstructionFormat(format string, opcode string, r TextRange) Diagnostic {
	return Diagnostic{
		Range:    r,
		Message:  "Invalid instruction format for " + opcode + "\nFormat: " + format,
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) InvalidInstruction(instruction string, r TextRange) Diagnostic {
	r, instruction = AdjustRange(r, instruction)
	return Diagnostic{
		Range:    r,
		Message:  "Invalid instruction: \"" + instruction + "\"",
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) InvalidExpression(expression string, r TextRange) Diagnostic {
	r, expression = AdjustRange(r, expression)
	return Diagnostic{
		Range:    r,
		Message:  "Invalid expression: \"" + expression + "\"",
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) AnonymousError(message string, r TextRange) Diagnostic {
	return Diagnostic{
		Range:    r,
		Message:  message,
		Source:   "Assembler",
		Severity: Error,
	}
}

func (assemblyError) LabelTooFar(label string, r TextRange) Diagnostic {
	r, label = AdjustRange(r, label)
	return Diagnostic{
		Range:    r,
		Message:  "Label \"" + label + "\" is too far away and the immediate value overflows. Use jal or auipc instead",
		Source:   "Assembler",
		Severity: Error,
	}
}

// Warnings
type assemblyWarning struct{}

var Warnings assemblyWarning

func (assemblyWarning) UnusedLabel(label string, r TextRange) Diagnostic {
	r, label = AdjustRange(r, label)
	return Diagnostic{
		Range:    r,
		Message:  "Unused label: \"" + label + "\"",
		Source:   "Assembler",
		Severity: Warning,
	}
}

func (assemblyWarning) UnintendedSignExtension(value string, r TextRange) Diagnostic {
	r, value = AdjustRange(r, value)
	return Diagnostic{
		Range:    r,
		Message:  "Possible unintended sign extension of \"" + value + "\"",
		Source:   "Assembler",
		Severity: Warning,
	}
}

func (assemblyWarning) ExplicitNumberLiteralForLabel(r TextRange) Diagnostic {
	return Diagnostic{
		Range:    r,
		Message:  "Explicit number literal used instead of label",
		Source:   "Assembler",
		Severity: Warning,
	}
}

func (assemblyWarning) LabelUsedForNumberLiteral(r TextRange) Diagnostic {
	return Diagnostic{
		Range:    r,
		Message:  "Label used instead of numeric literal for instructions expecting a numeric literal",
		Source:   "Assembler",
		Severity: Warning,
	}
}

func (assemblyWarning) ImmediateBitsWillBeDiscarded(value string, r TextRange) Diagnostic {
	r, value = AdjustRange(r, value)
	return Diagnostic{
		Range:    r,
		Message:  "Lower 12 bits of \"" + value + "\" will be discarded",
		Source:   "Assembler",
		Severity: Warning,
	}
}

// Evaluate-Specific Errors
type evaluationErrors struct{}

var EvaluationErrors evaluationErrors

type EvaluationUnresolvedSymbol struct {
	symbolName string
}

func (e *EvaluationUnresolvedSymbol) Error() string {
	return "Unresolved symbol: " + e.symbolName
}

type EvaluationInvalidNumberLiteral struct {
	expression string
}

func (e *EvaluationInvalidNumberLiteral) Error() string {
	return "Invalid number literal: " + e.expression
}

type EvaluationInvalidExpression struct {
	expression string
}

func (e *EvaluationInvalidExpression) Error() string {
	return "Invalid expression: " + e.expression
}

func (evaluationErrors) InvalidExpression(expression string) *EvaluationInvalidExpression {
	return &EvaluationInvalidExpression{expression: expression}
}

func (evaluationErrors) UnresolvedSymbol(symbolName string) *EvaluationUnresolvedSymbol {
	return &EvaluationUnresolvedSymbol{symbolName: symbolName}
}

func (evaluationErrors) InvalidNumberLiteral(expression string) *EvaluationInvalidNumberLiteral {
	return &EvaluationInvalidNumberLiteral{expression: expression}
}

func (evaluationErrors) IsUnresolvedSymbolError(err error) bool {
	_, ok := err.(*EvaluationUnresolvedSymbol)
	return ok
}

func (evaluationErrors) IsInvalidNumberLiteralError(err error) bool {
	_, ok := err.(*EvaluationInvalidNumberLiteral)
	return ok
}

func (evaluationErrors) IsInvalidExpressionError(err error) bool {
	_, ok := err.(*EvaluationInvalidExpression)
	return ok
}
