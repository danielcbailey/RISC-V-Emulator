package assembler_test

import (
	"testing"

	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/assembler"
)

func TestProgramIType(t *testing.T) {
	source := `
	.text
		addi x1, x0, 1
		addi x2, x0, 2
	`
	expected := []uint32{
		0x00100093,
		0x00200113,
	}

	program := assembler.Assemble(source)
	validateResult(t, program, expected, nil, nil)
}

func TestProgramBranchesAndLabels(t *testing.T) {
	source := `
	.text
		label1: addi x1, x0, 1
		addi x2, x0, 2
		beq x1, x2, label1 # should evaluate to -8
	`

	expected := []uint32{
		0x00100093,
		0x00200113,
		0xfe208ce3,
	}

	program := assembler.Assemble(source)
	validateResult(t, program, expected, nil, nil)
}

func TestProgramJumps(t *testing.T) {
	source := `
	.text
		jal x1, label1
		addi x2, x0, 2
		label1: addi x3, x0, 3
	`

	expected := []uint32{
		0x008000ef,
		0x00200113,
		0x00300193,
	}

	program := assembler.Assemble(source)
	validateResult(t, program, expected, nil, nil)
}

func TestDataWord(t *testing.T) {
	source := `
	.data
	MyWord: .word 0x12345678
	`

	expectedText := []uint32{}
	expectedData := []uint32{0x12345678}

	program := assembler.Assemble(source)
	validateResult(t, program, expectedText, expectedData, nil)
}

func TestDataString(t *testing.T) {
	source := `
	.data
	MyString: .ascii "Hello World!"
	`

	expectedText := []uint32{}
	expectedData := []uint32{0x6c6c6548, 0x6f57206f, 0x21646c72, 0x00000000}

	program := assembler.Assemble(source)
	validateResult(t, program, expectedText, expectedData, nil)
}

func TestDataInProgram(t *testing.T) {
	source := `
	.data
	MyWord: .word 0x12345678
	.text
	lw x1, MyWord(gp)
	`

	expectedText := []uint32{
		0x0001a083,
	}

	expectedData := []uint32{0x12345678}

	program := assembler.Assemble(source)
	validateResult(t, program, expectedText, expectedData, nil)
}

func validateResult(t *testing.T, program *assembler.AssembledResult, expectedText []uint32, expectedData []uint32, expectedDiagnostics []assembler.Diagnostic) {
	if len(program.Diagnostics) != len(expectedDiagnostics) {
		t.Fatalf("Expected %d diagnostics, got %d (%v)", len(expectedDiagnostics), len(program.Diagnostics), program.Diagnostics)
	}

	for i, diagnostic := range program.Diagnostics {
		if diagnostic.Severity != expectedDiagnostics[i].Severity {
			t.Errorf("Expected diagnostic %d to have severity %d, got %d", i, expectedDiagnostics[i].Severity, diagnostic.Severity)
		}

		if diagnostic.Range.Start.Line != expectedDiagnostics[i].Range.Start.Line {
			t.Errorf("Expected diagnostic %d to start on line %d, got %d", i, expectedDiagnostics[i].Range.Start.Line, diagnostic.Range.Start.Line)
		}

		if diagnostic.Range.Start.Char != expectedDiagnostics[i].Range.Start.Char {
			t.Errorf("Expected diagnostic %d to start on char %d, got %d", i, expectedDiagnostics[i].Range.Start.Char, diagnostic.Range.Start.Char)
		}

		if diagnostic.Range.End.Line != expectedDiagnostics[i].Range.End.Line {
			t.Errorf("Expected diagnostic %d to end on line %d, got %d", i, expectedDiagnostics[i].Range.End.Line, diagnostic.Range.End.Line)
		}

		if diagnostic.Range.End.Char != expectedDiagnostics[i].Range.End.Char {
			t.Errorf("Expected diagnostic %d to end on char %d, got %d", i, expectedDiagnostics[i].Range.End.Char, diagnostic.Range.End.Char)
		}

		if diagnostic.Message != expectedDiagnostics[i].Message {
			t.Errorf("Expected diagnostic %d to be \"%s\", got \"%s\"", i, expectedDiagnostics[i].Message, diagnostic.Message)
		}
	}

	if len(program.ProgramText) != len(expectedText) {
		t.Fatalf("Expected %d instructions, got %d", len(expectedText), len(program.ProgramText))
	}

	if len(program.ProgramData) != len(expectedData) {
		t.Fatalf("Expected %d data words, got %d", len(expectedData), len(program.ProgramData))
	}

	for i, instruction := range program.ProgramText {
		if instruction != expectedText[i] {
			t.Errorf("Expected instruction %d to be 0x%08x, got 0x%08x", i, expectedText[i], instruction)
		}
	}

	for i, data := range program.ProgramData {
		if data != expectedData[i] {
			t.Errorf("Expected data word %d to be 0x%08x, got 0x%08x", i, expectedData[i], data)
		}
	}
}
