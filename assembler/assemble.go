package assembler

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

var assemblerConfig AssemblerConfig

func GetConfig() AssemblerConfig {
	return assemblerConfig
}

func SetConfig(config AssemblerConfig) {
	assemblerConfig = config
}

func trimAndGetFrontDiffCount(str, cutset string) (string, int) {
	strOut := strings.Trim(str, cutset)
	return strOut, len(str) - len(strings.TrimLeft(str, cutset))
}

func checkValidSymbolName(str string) (bool, string) {
	str = strings.TrimSpace(str)
	if len(str) == 0 {
		return false, "symbol names must not be empty"
	}

	// must only contain alphanumeric characters and underscores
	for _, char := range str {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_') {
			return false, "symbol names must only contain alphanumeric characters and underscores"
		}
	}

	return true, ""
}

func (a *AssembledResult) Evaluate(str string, fieldWidth int, signed bool) (EvaluationResult, error) {
	str = strings.TrimSpace(str)
	if len(str) == 0 {
		return EvaluationResult{}, EvaluationErrors.InvalidExpression(str)
	}
	// can only be a label, base 10 integer, base 16 integer, or a register
	// first must check if it is a macro and will expand it if necessary
	macEval, ok := MacroMap[strings.ToLower(str)]
	if ok {
		str = macEval
	}

	// check if it is a label
	for label, value := range a.Labels {
		if label == str {
			return EvaluationResult{Value: int64(value), Type: EvaluationTypeLabel, MatchedValue: str}, nil
		}
	}

	// check if it is a register
	reg, ok := RegisterNameMap[strings.ToLower(str)]
	if ok {
		return EvaluationResult{Value: int64(reg), Type: EvaluationTypeRegister, MatchedValue: str}, nil
	}

	// check if it is a base 16 integer (must have 0x prefix)
	if len(str) > 2 && str[0] == '0' && (str[1] == 'x' || str[1] == 'X') {
		// check if too many digits for the fieldWidth
		if len(str[2:]) > fieldWidth/4 {
			return EvaluationResult{}, EvaluationErrors.ImmOverflow(str)
		}
		// check if the rest of the string is valid
		for _, char := range str[2:] {
			if !((char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') || (char >= '0' && char <= '9')) {
				return EvaluationResult{}, EvaluationErrors.InvalidNumberLiteral(str)
			}
		}
		// convert to base 10
		hexdigits := str[2:]
		MSHdigit := hexdigits[0:1]                         // most significant hex digit
		MSDecimal, e := strconv.ParseInt(MSHdigit, 16, 64) // decimal equivalent of MSHdigit
		if signed && e == nil && MSDecimal > 7 && len(hexdigits) == fieldWidth/4 {
			switch fieldWidth {
			case 12:
				hexdigits = "F" + hexdigits
			case 20:
				hexdigits = "FFF" + hexdigits
			}
			value, err := strconv.ParseInt(hexdigits, 16, 64)
			if err != nil {
				return EvaluationResult{}, err
			}

			switch fieldWidth {
			case 12:
				value = int64(int16(value))
			case 20:
				value = int64(int32(value))
			}

			return EvaluationResult{Value: value, Type: EvaluationTypeIntegerLiteral, MatchedValue: str}, nil

		}
		value, err := strconv.ParseInt(hexdigits, 16, 64)
		if err != nil {
			return EvaluationResult{}, err
		}
		return EvaluationResult{Value: value, Type: EvaluationTypeUnsignedIntegerLiteral, MatchedValue: str}, nil
	}

	// check if it is a base 10 integer
	for _, char := range str {
		if !(char >= '0' && char <= '9' || char == '-') {
			return EvaluationResult{}, EvaluationErrors.UnresolvedSymbol(str)
		}
	}
	// convert to base 10
	value, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return EvaluationResult{}, err
	}

	if value >= 0 {
		return EvaluationResult{Value: value, Type: EvaluationTypeUnsignedIntegerLiteral, MatchedValue: str}, nil
	}

	return EvaluationResult{Value: value, Type: EvaluationTypeIntegerLiteral, MatchedValue: str}, nil
}

func (a *AssembledResult) EvaluateAndReportErrors(str string, fieldWidth int, signed bool, line, charPos int) (EvaluationResult, bool) {
	result, err := a.Evaluate(str, fieldWidth, signed)
	if err != nil && EvaluationErrors.IsUnresolvedSymbolError(err) {
		a.Diagnostics = append(a.Diagnostics, Errors.UnresolvedSymbolName(str, TextRange{
			Start: TextPosition{Line: line, Char: charPos}, End: TextPosition{Line: line, Char: charPos + len(str)},
		}))
		return EvaluationResult{}, false
	} else if err != nil && EvaluationErrors.IsInvalidNumberLiteralError(err) {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidIntegerLiteral(str, TextRange{
			Start: TextPosition{Line: line, Char: charPos}, End: TextPosition{Line: line, Char: charPos + len(str)},
		}))
		return EvaluationResult{}, false
	} else if err != nil && EvaluationErrors.IsInvalidExpressionError(err) {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidExpression(str, TextRange{
			Start: TextPosition{Line: line, Char: charPos}, End: TextPosition{Line: line, Char: charPos + len(str)},
		}))
		return EvaluationResult{}, false
	} else if err != nil && EvaluationErrors.IsImmOverflowError(err) {
		a.Diagnostics = append(a.Diagnostics, Errors.ImmediateOverflow(str, fieldWidth, TextRange{
			Start: TextPosition{Line: line, Char: charPos}, End: TextPosition{Line: line, Char: charPos + len(str)},
		}))
		return EvaluationResult{}, false
	} else if err != nil {
		a.Diagnostics = append(a.Diagnostics, Errors.AnonymousError(err.Error(), TextRange{
			Start: TextPosition{Line: line, Char: charPos}, End: TextPosition{Line: line, Char: charPos + len(str)},
		}))
		return EvaluationResult{}, false
	}
	return result, true
}

func (a *AssembledResult) extractLabels() {
	for i, line := range a.fileContents {
		// removing whitespaces
		line, diff := trimAndGetFrontDiffCount(line, " \t\r")
		if strings.Contains(line, "#") {
			// remove comments
			line = line[:strings.Index(line, "#")]
		}
		if strings.Contains(line, ":") {
			colonIndex := strings.Index(line, ":")
			labelName := line[:colonIndex]
			if valid, reason := checkValidSymbolName(labelName); !valid {
				a.Diagnostics = append(a.Diagnostics, Errors.InvalidSymbolName(labelName, reason, TextRange{
					Start: TextPosition{Line: i, Char: diff}, End: TextPosition{Line: i, Char: diff + colonIndex + 1},
				}))
				continue
			}
			a.Labels[labelName] = uint32(i) // line number for now, we will link against this later during code generation
			a.LabelToLineNumber[labelName] = i
			a.fileContents[i] = line[colonIndex+1:] // remove the label from the line
			a.lineLengthDeltas[i] = colonIndex + 1
		}
	}
}

func (a *AssembledResult) parseRTypeInstruction(line string, diff, lineNum int, opcode string) (uint32, bool) {
	// diff is the difference between the start of line and the start of the actual line in the file

	// format is <opcode> <operand1 reg>, <operand2 reg>, <operand3 reg>
	// split by commas
	parts := strings.Split(line, ",")
	if len(parts) != 3 {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <reg>, <reg>, <reg>", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: diff + len(line)},
		}))
		return 0, false
	}

	if !strings.Contains(parts[0], " ") {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <reg>, <reg>, <reg>", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: diff + len(parts[0])},
		}))
		return 0, false
	}

	operand1 := parts[0][strings.Index(parts[0], " ")+1:]

	// parse operand 1
	dest, err := a.Evaluate(operand1, 0, false)
	if err != nil || dest.Type != EvaluationTypeRegister {
		offset := diff + len(opcode) + 1
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(operand1, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand1)},
		}))
		return 0, false
	} else if slices.Contains(assemblerConfig.SpecialRegisters, operand1) {
		// Attempting to modify a special register; throw warning
		offset := len(opcode) + 1 + diff

		a.Diagnostics = append(a.Diagnostics, Warnings.ModifyingSpecialRegister(operand1, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand1)},
		}))

		return 0, false
	}

	// parse operand 2
	op1, err := a.Evaluate(parts[1], 0, false)
	if err != nil || op1.Type != EvaluationTypeRegister {
		offset := diff + len(opcode) + 1 + len(operand1) + 1
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(parts[1], TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
		}))
		return 0, false
	}

	// parse operand 3
	op2, err := a.Evaluate(parts[2], 0, false)
	if err != nil || op2.Type != EvaluationTypeRegister {
		offset := diff + len(opcode) + 1 + len(operand1) + 1 + len(parts[1]) + 1
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(parts[2], TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[2])},
		}))
		return 0, false
	}

	opNum := uint32(0)
	func7 := uint32(0)
	func3 := uint32(0)
	switch strings.ToLower(opcode) {
	case "add":
		opNum = 0b0110011
		func7 = 0b0000000
		func3 = 0b000
	case "sub":
		opNum = 0b0110011
		func7 = 0b0100000
		func3 = 0b000
	case "xor":
		opNum = 0b0110011
		func7 = 0b0000000
		func3 = 0b100
	case "or":
		opNum = 0b0110011
		func7 = 0b0000000
		func3 = 0b110
	case "and":
		opNum = 0b0110011
		func7 = 0b0000000
		func3 = 0b111
	case "sll":
		opNum = 0b0110011
		func7 = 0b0000000
		func3 = 0b001
	case "srl":
		opNum = 0b0110011
		func7 = 0b0000000
		func3 = 0b101
	case "sra":
		opNum = 0b0110011
		func7 = 0b0100000
		func3 = 0b101
	case "slt":
		opNum = 0b0110011
		func7 = 0b0000000
		func3 = 0b010
	case "sltu":
		opNum = 0b0110011
		func7 = 0b0000000
		func3 = 0b011
	case "mul":
		opNum = 0b0110011
		func7 = 0b0000001
		func3 = 0b000
	case "div":
		opNum = 0b0110011
		func7 = 0b0000001
		func3 = 0b100
	case "divu":
		opNum = 0b0110011
		func7 = 0b0000001
		func3 = 0b101
	case "rem":
		opNum = 0b0110011
		func7 = 0b0000001
		func3 = 0b110
	case "remu":
		opNum = 0b0110011
		func7 = 0b0000001
		func3 = 0b111
	case "mulu": // this is believed to be a typo in the spec card, so the below is more accurate
		opNum = 0b0110011
		func7 = 0b0000001
		func3 = 0b011
	case "mulhu":
		opNum = 0b0110011
		func7 = 0b0000001
		func3 = 0b011
	case "mulh":
		opNum = 0b0110011
		func7 = 0b0000001
		func3 = 0b001
	case "mulhsu":
		opNum = 0b0110011
		func7 = 0b0000001
		func3 = 0b010
	}

	a.AddressToLine[a.currentAddress] = lineNum
	a.currentAddress += 4
	return makeRTypeInstruction(opNum, uint32(dest.Value), uint32(op1.Value), uint32(op2.Value), func7, func3), true
}

func (a *AssembledResult) parseITypeInstruction(line string, diff, lineNum int, opcode string) (uint32, bool) {
	// diff is the difference between the start of line and the start of the actual line in the file

	// format is <opcode> <operand1 reg>, <operand2 reg>, <operand3 imm>
	// split by commas
	parts := strings.Split(line, ",")
	if len(parts) != 3 {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <reg>, <reg>, <imm>", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: diff + len(line)},
		}))
		return 0, false
	}

	if !strings.Contains(parts[0], " ") {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <reg>, <reg>, <imm>", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: diff + len(parts[0])},
		}))
		return 0, false
	}

	operand1 := parts[0][strings.Index(parts[0], " ")+1:]

	deOp := uint32(0)
	func3 := uint32(0)
	unsigned := false
	isSRA := false
	switch strings.ToLower(opcode) {
	case "addi":
		deOp = 0b0010011
		func3 = 0b000
	case "xori":
		deOp = 0b0010011
		func3 = 0b100
	case "ori":
		deOp = 0b0010011
		func3 = 0b110
	case "andi":
		deOp = 0b0010011
		func3 = 0b111
	case "slli":
		deOp = 0b0010011
		func3 = 0b001
		unsigned = true
	case "srli":
		deOp = 0b0010011
		func3 = 0b101
		unsigned = true
	case "srai":
		deOp = 0b0010011
		func3 = 0b101
		isSRA = true
		unsigned = true
	case "slti":
		deOp = 0b0010011
		func3 = 0b010
	case "sltiu":
		deOp = 0b0010011
		func3 = 0b011
		unsigned = true
	case "jalr":
		deOp = 0b1100111
		func3 = 0b000
	}

	// parse operand 1
	dest, err := a.Evaluate(operand1, 0, false)
	if err != nil || dest.Type != EvaluationTypeRegister {
		offset := diff + len(opcode) + 1
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(operand1, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand1)},
		}))
		return 0, false
	}

	// parse operand 2
	op1, err := a.Evaluate(parts[1], 0, false)
	if err != nil || op1.Type != EvaluationTypeRegister {
		offset := diff + len(opcode) + 1 + len(operand1) + 1
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(parts[1], TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
		}))
		return 0, false
	}

	// parse operand 3
	op2, err := a.Evaluate(parts[2], 12, !unsigned)
	immOverflow := (err != nil && EvaluationErrors.IsImmOverflowError((err)))
	immTypeValid := true
	if unsigned && op2.Type != EvaluationTypeUnsignedIntegerLiteral && op2.Type != EvaluationTypeLabel {
		immTypeValid = false
	} else if !unsigned && op2.Type != EvaluationTypeIntegerLiteral && op2.Type != EvaluationTypeUnsignedIntegerLiteral && op2.Type != EvaluationTypeLabel {
		immTypeValid = false
	}
	// check for invalid integers given as immediates (but handle immediate
	// overflow errors below so target range can be given in error msg)
	if (err != nil && !immOverflow) || !immTypeValid {
		offset := diff + len(opcode) + 1 + len(operand1) + 1 + len(parts[1]) + 1
		if !unsigned {
			a.Diagnostics = append(a.Diagnostics, Errors.InvalidIntegerLiteral(parts[2], TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[2])},
			}))
		} else {
			a.Diagnostics = append(a.Diagnostics, Errors.InvalidUnsignedIntegerLiteral(parts[2], TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[2])},
			}))
		}
		return 0, false
	}

	// checking for immediate overflow
	if !unsigned && op2.Type == EvaluationTypeUnsignedIntegerLiteral {
		op2.Type = EvaluationTypeIntegerLiteral
	}
	if op2.Type == EvaluationTypeIntegerLiteral || op2.Type == EvaluationTypeLabel {
		// maximum of 12 bits
		if immOverflow || op2.Value > 2047 || op2.Value < -2048 {
			offset := diff + len(opcode) + 1 + len(operand1) + 1 + len(parts[1]) + 1
			a.Diagnostics = append(a.Diagnostics, Errors.ImmediateOverflow(parts[2], 12, TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[2])},
			}))
			return 0, false
		}
	} else if op2.Type == EvaluationTypeUnsignedIntegerLiteral {
		// maximum of 12 bits
		if immOverflow || op2.Value > 4095 {
			offset := diff + len(opcode) + 1 + len(operand1) + 1 + len(parts[1]) + 1
			a.Diagnostics = append(a.Diagnostics, Errors.UnsignedImmediateOverflow(parts[2], 12, TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[2])},
			}))
			return 0, false
		} else if immOverflow || (!unsigned && op2.Value > 2047) {
			offset := diff + len(opcode) + 1 + len(operand1) + 1 + len(parts[1]) + 1
			a.Diagnostics = append(a.Diagnostics, Warnings.UnintendedSignExtension(parts[2], TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[2])},
			}))
		}
	}

	// if the immediate is a label, must add a link request
	if op2.Type == EvaluationTypeLabel {
		a.labelLinkRequests = append(a.labelLinkRequests, labelLinkRequest{
			address:   a.currentAddress,
			labelName: op2.MatchedValue,
			isBranch:  false,
		})
	}

	imm := uint32(op2.Value)
	if isSRA {
		imm |= 0b010000000000 // this marks it as an SRA instruction as opposed to an SRL instruction
	}

	a.AddressToLine[a.currentAddress] = lineNum
	a.currentAddress += 4 // preparing for the next instruction
	return makeITypeInstruction(deOp, uint32(dest.Value), uint32(op1.Value), imm, func3), true
}

func (a *AssembledResult) parseITypeMemInstruction(line string, diff, lineNum int, opcode string) (uint32, bool) {
	// format is <opcode> <operand1 reg>, <operand2 imm>(<operand3 reg>)

	// getting opcode
	parts := strings.Split(line, ",")
	if len(parts) != 2 {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <reg>, <imm>(<reg>)", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: diff + len(line)},
		}))
		return 0, false
	}

	if !strings.Contains(parts[0], " ") {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <reg>, <imm>(<reg>)", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: diff + len(parts[0])},
		}))
		return 0, false
	}

	operand1 := parts[0][strings.Index(parts[0], " ")+1:]

	if !strings.Contains(parts[1], "(") || !strings.Contains(parts[1], ")") {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <reg>, <imm>(<reg>)", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff + len(parts[0]) + 1}, End: TextPosition{Line: lineNum, Char: diff + len(parts[0]) + 1 + len(parts[1])},
		}))
		return 0, false
	}

	operand2 := parts[1][:strings.Index(parts[1], "(")]
	operand3 := parts[1][strings.Index(parts[1], "(")+1 : strings.Index(parts[1], ")")]

	// parse operand 1
	dest, e := a.Evaluate(operand1, 0, false)
	if e != nil {
		offset := diff + len(opcode) + 1
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(operand1, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand1)},
		}))
		return 0, false
	}

	// parse operand 2

	op2, ok := a.EvaluateAndReportErrors(operand2, 12, true, lineNum, diff+len(parts[0])+1)
	if !ok {
		return 0, false
	} else if op2.Type == EvaluationTypeRegister {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidIntegerLiteral(operand2, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff + len(parts[0]) + 1}, End: TextPosition{Line: lineNum, Char: diff + len(parts[0]) + 1 + len(operand2)},
		}))
		return 0, false
	}

	// parse operand 3
	op3, e := a.Evaluate(operand3, 0, false)
	if e != nil || op3.Type != EvaluationTypeRegister {
		offset := diff + strings.Index(line, "(") + 1
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(operand3, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand3)},
		}))
		return 0, false
	}

	// check if immediate is in range
	/*
		if op2.Type == EvaluationTypeUnsignedIntegerLiteral {
			// maximum of 12 bits
			if op2.Value > 4095 {
				offset := diff + len(parts[0]) + 1
				a.Diagnostics = append(a.Diagnostics, Errors.ImmediateOverflow(parts[1], 12, TextRange{
					Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
				}))
				return 0, false
			}
		} else */
	if op2.Type == EvaluationTypeIntegerLiteral || op2.Type == EvaluationTypeLabel || op2.Type == EvaluationTypeUnsignedIntegerLiteral {
		// maximum of 12 bits
		if op2.Value < -2048 || op2.Value > 2047 {
			offset := diff + len(parts[0]) + 1
			a.Diagnostics = append(a.Diagnostics, Errors.ImmediateOverflow(operand2, 12, TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand2)},
			}))
			return 0, false
		}
	}

	// if the immediate is a label, must add a link request
	if op2.Type == EvaluationTypeLabel {
		a.labelLinkRequests = append(a.labelLinkRequests, labelLinkRequest{
			address:   a.currentAddress,
			labelName: op2.MatchedValue,
			isBranch:  false,
		})
	}

	deOp := uint32(0)
	func3 := uint32(0)
	switch opcode {
	case "lb":
		deOp = 0b0000011
		func3 = 0b000
	case "lh":
		deOp = 0b0000011
		func3 = 0b001
	case "lw":
		deOp = 0b0000011
		func3 = 0b010
	case "lbu":
		deOp = 0b0000011
		func3 = 0b100
	case "lhu":
		deOp = 0b0000011
		func3 = 0b101
	}

	a.AddressToLine[a.currentAddress] = lineNum
	a.currentAddress += 4 // preparing for the next instruction
	return makeITypeInstruction(deOp, uint32(dest.Value), uint32(op3.Value), uint32(op2.Value), func3), true
}

func (a *AssembledResult) parseSTypeInstruction(line string, diff int, lineNum int, opcode string) (uint32, bool) {
	// format is <opcode> <operand1 reg>, <operand2 imm>(<operand3 reg>)

	// getting opcode
	parts := strings.Split(line, ",")
	if len(parts) != 2 {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <reg>, <imm>(<reg>)", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: diff + len(line)},
		}))
		return 0, false
	}

	if !strings.Contains(parts[0], " ") {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <reg>, <imm>(<reg>)", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: diff + len(parts[0])},
		}))
		return 0, false
	}

	operand1 := parts[0][strings.Index(parts[0], " ")+1:]

	if !strings.Contains(parts[1], "(") || !strings.Contains(parts[1], ")") {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <reg>, <imm>(<reg>)", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff + len(parts[0]) + 1}, End: TextPosition{Line: lineNum, Char: diff + len(parts[0]) + 1 + len(parts[1])},
		}))
		return 0, false
	}

	operand2 := parts[1][:strings.Index(parts[1], "(")]
	operand3 := parts[1][strings.Index(parts[1], "(")+1 : strings.Index(parts[1], ")")]

	// parse operand 1
	src, e := a.Evaluate(operand1, 0, false)
	if e != nil {
		offset := diff + len(opcode) + 1
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(operand1, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand1)},
		}))
		return 0, false
	}

	// parse operand 2
	op2, ok := a.EvaluateAndReportErrors(operand2, 12, true, lineNum, diff+len(parts[0])+1)
	if !ok {
		return 0, false
	} else if op2.Type == EvaluationTypeRegister {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidIntegerLiteral(operand2, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff + len(parts[0]) + 1}, End: TextPosition{Line: lineNum, Char: diff + len(parts[0]) + 1 + len(operand2)},
		}))
		return 0, false
	}

	// parse operand 3
	op3, e := a.Evaluate(operand3, 0, false)
	if e != nil || op3.Type != EvaluationTypeRegister {
		offset := diff + strings.Index(line, "(") + 1
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(operand3, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand3)},
		}))
		return 0, false
	}

	// check if immediate is in range
	/*
		if op2.Type == EvaluationTypeUnsignedIntegerLiteral {
			// maximum of 12 bits
			if op2.Value > 4095 {
				offset := diff + len(parts[0]) + 1
				a.Diagnostics = append(a.Diagnostics, Errors.ImmediateOverflow(parts[1], 12, TextRange{
					Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
				}))
				return 0, false
			}
		} else */
	if op2.Type == EvaluationTypeIntegerLiteral || op2.Type == EvaluationTypeLabel || op2.Type == EvaluationTypeUnsignedIntegerLiteral {
		// maximum of 12 bits
		if op2.Value < -2048 || op2.Value > 2047 {
			offset := diff + len(parts[0]) + 1
			a.Diagnostics = append(a.Diagnostics, Errors.ImmediateOverflow(operand2, 12, TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand2)},
			}))
			return 0, false
		}
	}

	// if the immediate is a label, must add a link request
	if op2.Type == EvaluationTypeLabel {
		a.labelLinkRequests = append(a.labelLinkRequests, labelLinkRequest{
			address:   a.currentAddress,
			labelName: op2.MatchedValue,
			isBranch:  false,
		})
	}

	deOp := uint32(0)
	func3 := uint32(0)
	switch opcode {
	case "sb":
		deOp = 0b0100011
		func3 = 0b000
	case "sh":
		deOp = 0b0100011
		func3 = 0b001
	case "sw":
		deOp = 0b0100011
		func3 = 0b010
	}

	a.AddressToLine[a.currentAddress] = lineNum
	a.currentAddress += 4 // preparing for the next instruction
	return makeSTypeInstruction(deOp, uint32(op3.Value), uint32(src.Value), uint32(op2.Value), func3), true
}

func (a *AssembledResult) parseBTypeInstruction(line string, diff, lineNum int, opcode string) (uint32, bool) {
	parts := strings.Split(line, ",")
	if len(parts) != 3 {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <reg>, <reg>, <imm>", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: len(line) + diff},
		}))
		return 0, false
	}

	operand1 := parts[0][strings.Index(parts[0], " ")+1:]

	// parse operand 1
	src, e := a.Evaluate(operand1, 0, false)
	if e != nil {
		offset := len(opcode) + 1 + diff
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(operand1, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand1)},
		}))
		return 0, false
	}

	// parse operand 2
	op2, e := a.Evaluate(parts[1], 0, false)
	if e != nil || op2.Type != EvaluationTypeRegister {
		offset := len(parts[0]) + 1 + diff
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(parts[1], TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
		}))
		return 0, false
	}

	// parse operand 3
	op3, ok := a.EvaluateAndReportErrors(parts[2], 12, true, lineNum, diff+len(parts[0])+1+len(parts[1])+1)
	if !ok {
		return 0, false
	} else if op3.Type == EvaluationTypeRegister {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidIntegerLiteral(parts[2], TextRange{
			Start: TextPosition{Line: lineNum, Char: len(parts[0]) + 1 + diff + len(parts[1]) + 1}, End: TextPosition{Line: lineNum, Char: len(parts[0]) + 1 + len(parts[1]) + len(parts[2]) + diff},
		}))
		return 0, false
	}

	// check if immediate is in range
	/*
		if op3.Type == EvaluationTypeUnsignedIntegerLiteral {
			// maximum of 13 bits
			if op3.Value > 2047 {
				offset := len(parts[0]) + 1 + diff + len(parts[1]) + 1
				a.Diagnostics = append(a.Diagnostics, Errors.ImmediateOverflow(parts[2], 12, TextRange{
					Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[2])},
				}))
				return 0, false
			}
		} else */
	if op3.Type == EvaluationTypeIntegerLiteral || op3.Type == EvaluationTypeLabel || op3.Type == EvaluationTypeUnsignedIntegerLiteral {
		// maximum of 13 bits
		if op3.Value < -4096 || op3.Value > 4095 {
			offset := len(parts[0]) + 1 + diff + len(parts[1]) + 1
			a.Diagnostics = append(a.Diagnostics, Errors.ImmediateOverflow(parts[2], 13, TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[2])},
			}))
			return 0, false
		}
	}

	// if the immediate is a label, must add a link request
	if op3.Type == EvaluationTypeLabel {
		a.labelLinkRequests = append(a.labelLinkRequests, labelLinkRequest{
			address:   a.currentAddress,
			labelName: op3.MatchedValue,
			isBranch:  true,
		})
	}

	deOp := uint32(0)
	func3 := uint32(0)
	switch opcode {
	case "beq":
		deOp = 0b1100011
		func3 = 0b000
	case "bne":
		deOp = 0b1100011
		func3 = 0b001
	case "blt":
		deOp = 0b1100011
		func3 = 0b100
	case "bge":
		deOp = 0b1100011
		func3 = 0b101
	case "bltu":
		deOp = 0b1100011
		func3 = 0b110
	case "bgeu":
		deOp = 0b1100011
		func3 = 0b111
	}

	a.AddressToLine[a.currentAddress] = lineNum
	a.currentAddress += 4 // preparing for the next instruction
	return makeBTypeInstruction(deOp, uint32(src.Value), uint32(op2.Value), uint32(op3.Value), func3), true
}

func (a *AssembledResult) parseJTypeInstruction(line string, diff, lineNum int, opcode string) (uint32, bool) {
	// format is <opcode> <register>, <label>
	parts := strings.Split(line, ",")
	if len(parts) != 2 {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <register>, <imm>", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: len(line) + diff},
		}))
		return 0, false
	}

	operand1 := parts[0][strings.Index(parts[0], " ")+1:]

	// parse operand 1
	src, e := a.Evaluate(operand1, 0, false)
	if e != nil {
		offset := len(opcode) + 1 + diff
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(operand1, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand1)},
		}))
		return 0, false
	} else if src.Type != EvaluationTypeRegister {
		offset := len(opcode) + 1 + diff
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(operand1, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand1)},
		}))
		return 0, false
	}

	// parse operand 2
	op2, ok := a.EvaluateAndReportErrors(parts[1], 20, true, lineNum, diff+len(parts[0])+1)
	if !ok {
		return 0, false
	} else if op2.Type == EvaluationTypeIntegerLiteral || op2.Type == EvaluationTypeUnsignedIntegerLiteral {
		offset := len(parts[0]) + 1 + diff
		a.Diagnostics = append(a.Diagnostics, Warnings.ExplicitNumberLiteralForLabel(TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
		}))
		return 0, false
	} else if op2.Type != EvaluationTypeLabel {
		offset := len(parts[0]) + 1 + diff
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidIntegerLiteral(parts[1], TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
		}))
		return 0, false
	}

	// if the immediate is a label, must add a link request
	if op2.Type == EvaluationTypeLabel {
		a.labelLinkRequests = append(a.labelLinkRequests, labelLinkRequest{
			address:   a.currentAddress,
			labelName: op2.MatchedValue,
			isBranch:  true,
		})
	} else {
		// otherwise, issue a warning for not using a label
		offset := len(parts[0]) + 1 + diff
		a.Diagnostics = append(a.Diagnostics, Warnings.ExplicitNumberLiteralForLabel(TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
		}))
	}

	deOp := uint32(0b1101111)

	a.AddressToLine[a.currentAddress] = lineNum
	a.currentAddress += 4 // preparing for the next instruction
	return makeJTypeInstruction(deOp, uint32(src.Value), uint32(op2.Value)), true
}

func (a *AssembledResult) parseUTypeInstruction(line string, diff, lineNum int, opcode string) (uint32, bool) {
	// format is <opcode> <register>, <imm>
	parts := strings.Split(line, ",")
	if len(parts) != 2 {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode> <register>, <imm>", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: len(line) + diff},
		}))
		return 0, false
	}

	operand1 := parts[0][strings.Index(parts[0], " ")+1:]

	// parse operand 1
	src, e := a.Evaluate(operand1, 0, false)
	if e != nil {
		offset := len(opcode) + 1 + diff
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(operand1, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand1)},
		}))
		return 0, false
	} else if src.Type != EvaluationTypeRegister {
		offset := len(opcode) + 1 + diff
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidRegister(operand1, TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(operand1)},
		}))
		return 0, false
	}

	// parse operand 2
	op2, ok := a.EvaluateAndReportErrors(parts[1], 20, false, lineNum, diff+len(parts[0])+1)
	if !ok {
		return 0, false
	} else if op2.Type == EvaluationTypeRegister {
		offset := len(parts[0]) + 1 + diff
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidIntegerLiteral(parts[2], TextRange{
			Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
		}))
		return 0, false
	}

	// if the immediate is a label, must add a link request
	if op2.Type == EvaluationTypeLabel {
		a.labelLinkRequests = append(a.labelLinkRequests, labelLinkRequest{
			address:   a.currentAddress,
			labelName: op2.MatchedValue,
			isBranch:  false,
		})
	}

	op2.Value <<= 12

	// checking to make sure the immediate is in range (32 bits, but only the upper 20 bits are used)
	if op2.Type == EvaluationTypeUnsignedIntegerLiteral {
		// maximum of 32 bits
		if op2.Value > 0xFFFFFFFF {
			offset := diff + len(parts[0]) + 1
			a.Diagnostics = append(a.Diagnostics, Errors.UnsignedImmediateOverflow(parts[1], 20, TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
			}))
			return 0, false
		}

		// yes I could have gotten rid of this, but I'll keep it for now
		if op2.Value&0xFFF != 0 {
			offset := diff + len(parts[0]) + 1
			a.Diagnostics = append(a.Diagnostics, Warnings.ImmediateBitsWillBeDiscarded(parts[1], TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
			}))
		}
	} else if op2.Type == EvaluationTypeIntegerLiteral || op2.Type == EvaluationTypeLabel {
		// maximum of 32 bits
		if op2.Value > 0x7FFFFFFF || op2.Value < -0x80000000 {
			offset := diff + len(parts[0]) + 1
			a.Diagnostics = append(a.Diagnostics, Errors.ImmediateOverflow(parts[1], 20, TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
			}))
			return 0, false
		}

		if (op2.Value > 0 && op2.Value&0xFFF != 0) || (op2.Value < 0 && uint64(op2.Value)&0xFFF != 0) {
			offset := diff + len(parts[0]) + 1
			a.Diagnostics = append(a.Diagnostics, Warnings.ImmediateBitsWillBeDiscarded(parts[1], TextRange{
				Start: TextPosition{Line: lineNum, Char: offset}, End: TextPosition{Line: lineNum, Char: offset + len(parts[1])},
			}))
		}
	}

	deop := uint32(0)
	switch opcode {
	case "lui":
		deop = 0b0110111
	case "auipc":
		deop = 0b0010111
	}

	a.AddressToLine[a.currentAddress] = lineNum
	a.currentAddress += 4 // preparing for the next instruction
	return makeUTypeInstruction(deop, uint32(src.Value), uint32(op2.Value>>12)), true
}

func (a *AssembledResult) parseITypeInstructionWithoutArguments(line string, diff, lineNum int, opcode string) (uint32, bool) {
	// format is just <opcode>
	parts := strings.Split(line, " ")
	if len(parts) != 1 {
		a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstructionFormat("<opcode>", opcode, TextRange{
			Start: TextPosition{Line: lineNum, Char: diff}, End: TextPosition{Line: lineNum, Char: diff + len(line)},
		}))
		return 0, false
	}

	deop := uint32(0)
	immValue := uint32(0)
	switch opcode {
	case "ecall":
		deop = 0b1110011
	case "ebreak":
		deop = 0b1110011
		immValue = 1
	}

	a.AddressToLine[a.currentAddress] = lineNum
	a.currentAddress += 4 // preparing for the next instruction
	return makeITypeInstruction(deop, 0, 0, immValue, 0), true
}

func (a *AssembledResult) resolveLabelLinkRequests() {
	for _, request := range a.labelLinkRequests {
		labelAddr := a.Labels[request.labelName]
		currAddr := request.address

		address := request.address
		instruction := a.ProgramText[address/4]
		opcode := GetOpCode(instruction)
		if opcode == OPCODE_ITYPE || opcode == OPCODE_MEMITYPE || opcode == OPCODE_JALR {
			// I type
			opcode, rd, rs1, _, func3 := DecodeITypeInstruction(instruction)
			imm := uint32(0)
			if request.isBranch || a.LabelTypes[request.labelName] == "text" {
				// if is branch, the label should be relative to the instruction address
				immInt := int32((labelAddr - currAddr))
				if immInt > 4095 || immInt < -4096 {
					lineNum := a.AddressToLine[address]
					charPos := strings.Index(a.fileContents[lineNum], request.labelName)
					a.Diagnostics = append(a.Diagnostics, Errors.LabelTooFar(request.labelName, TextRange{
						Start: TextPosition{Line: lineNum, Char: a.lineLengthDeltas[lineNum] + charPos}, End: TextPosition{Line: lineNum, Char: charPos + a.lineLengthDeltas[lineNum] + len(request.labelName)},
					}))
					return
				}
				imm = uint32(immInt)
			} else {
				imm = labelAddr
			}

			a.ProgramText[address/4] = makeITypeInstruction(opcode, rd, rs1, imm, func3)
		} else if opcode == OPCODE_JAL {
			// J type
			opcode, rd, _ := DecodeJTypeInstruction(instruction)
			// the label should be treated as relative to the instruction address
			immInt := int32((labelAddr - currAddr))
			if immInt > 0xFFFFF || immInt < -0x100000 { // +/- 1M
				lineNum := a.AddressToLine[address]
				charPos := strings.Index(a.fileContents[lineNum], request.labelName)
				a.Diagnostics = append(a.Diagnostics, Errors.LabelTooFar(request.labelName, TextRange{
					Start: TextPosition{Line: lineNum, Char: a.lineLengthDeltas[lineNum] + charPos}, End: TextPosition{Line: lineNum, Char: charPos + a.lineLengthDeltas[lineNum] + len(request.labelName)},
				}))
				return
			}

			a.ProgramText[address/4] = makeJTypeInstruction(opcode, rd, uint32(immInt))
		} else if opcode == OPCODE_STYPE {
			// S type
			opcode, rs1, rs2, _, func3 := DecodeSTypeInstruction(instruction)
			// the label should be treated as relative to the start of the data section, but this will have already been computed as the label's address
			imm := labelAddr
			a.ProgramText[address/4] = makeSTypeInstruction(opcode, rs1, rs2, imm, func3)
		} else if opcode == OPCODE_BTYPE {
			// B type
			opcode, rs1, rs2, _, func3 := DecodeBTypeInstruction(instruction)
			// the label should be treated as relative to the instruction address
			immInt := int32((labelAddr - currAddr))
			if immInt > 4095 || immInt < -4096 {
				lineNum := a.AddressToLine[address]
				charPos := strings.Index(a.fileContents[lineNum], request.labelName)
				a.Diagnostics = append(a.Diagnostics, Errors.LabelTooFar(request.labelName, TextRange{
					Start: TextPosition{Line: lineNum, Char: a.lineLengthDeltas[lineNum] + charPos}, End: TextPosition{Line: lineNum, Char: charPos + a.lineLengthDeltas[lineNum] + len(request.labelName)},
				}))
				return
			}

			a.ProgramText[address/4] = makeBTypeInstruction(opcode, rs1, rs2, uint32(immInt), func3)
		} else if opcode == OPCODE_LUI || opcode == OPCODE_AUIPC {
			// U type
			opcode, rd, _ := DecodeUTypeInstruction(instruction)
			// the label will be treated as absolute
			a.ProgramText[address/4] = makeUTypeInstruction(opcode, rd, labelAddr)
		}
	}
}

func (a *AssembledResult) parseLines() {
	textSection := false
	for i, line := range a.fileContents {
		line, diff := trimAndGetFrontDiffCount(line, " \t\r")
		oldDiff, ok := a.lineLengthDeltas[i]
		if ok {
			diff += oldDiff
		}

		line = strings.ReplaceAll(line, "\t", " ") // replacing tabs with single space because it was originally just one character

		// if the entire line is a macro (say, nop), then we can just replace it with the macro's contents
		eval, ok := MacroMap[strings.ToLower(line)]
		if ok {
			line = eval
		}

		// removing comment
		// perhaps in the future we can add support to parse comments to add advanced code labeling features
		line = strings.Split(line, "#")[0]

		// remove trailing whitespace
		line = strings.TrimRight(line, " \t\r")

		directiveLine := strings.TrimLeft(line, " \t\r")
		if strings.HasPrefix(strings.TrimLeft(directiveLine, " \t\r"), ".text") || strings.HasPrefix(strings.TrimLeft(directiveLine, " \t\r"), ".data") {
			// directive
			textSection = strings.HasPrefix(strings.TrimLeft(directiveLine, " \t\r"), ".text")
		} else if textSection {
			// instruction

			// checking if a label was on this line, if so setting its address
			for label, lineNum := range a.LabelToLineNumber {
				if lineNum == i {
					a.Labels[label] = uint32(a.currentAddress)
					a.LabelTypes[label] = "text"
				}
			}

			if len(line) == 0 {
				continue
			}

			// format is one of
			//<opcode> <operand1 reg>, <operand2 reg>, <operand3 reg|imm>
			//<opcode> <operand1 reg>, <operand2 reg|imm>
			//<opcode> <operand1>
			//<opcode> <operand1 reg>, <operand2 imm>(<operand3 reg>)

			// getting opcode
			opcode := strings.ToLower(strings.Split(line, " ")[0])
			if opcode == "add" ||
				opcode == "sub" ||
				opcode == "xor" ||
				opcode == "or" ||
				opcode == "and" ||
				opcode == "sll" ||
				opcode == "srl" ||
				opcode == "sra" ||
				opcode == "slt" ||
				opcode == "sltu" ||
				opcode == "mul" ||
				opcode == "mulhsu" ||
				opcode == "mulh" ||
				opcode == "mulu" ||
				opcode == "mulhu" ||
				opcode == "div" ||
				opcode == "divu" ||
				opcode == "rem" ||
				opcode == "remu" {
				// R-type instruction
				code, ok := a.parseRTypeInstruction(line, diff, i, opcode)
				if ok {
					a.ProgramText = append(a.ProgramText, code)
				}
			} else if opcode == "addi" ||
				opcode == "slti" ||
				opcode == "sltiu" ||
				opcode == "xori" ||
				opcode == "ori" ||
				opcode == "andi" ||
				opcode == "slli" ||
				opcode == "srli" ||
				opcode == "srai" ||
				opcode == "jalr" {
				// I-type instruction
				code, ok := a.parseITypeInstruction(line, diff, i, opcode)
				if ok {
					a.ProgramText = append(a.ProgramText, code)
				}
			} else if opcode == "lb" ||
				opcode == "lh" ||
				opcode == "lw" ||
				opcode == "lbu" ||
				opcode == "lhu" {
				// I-type instruction, but with memory notation
				code, ok := a.parseITypeMemInstruction(line, diff, i, opcode)
				if ok {
					a.ProgramText = append(a.ProgramText, code)
				}
			} else if opcode == "sb" ||
				opcode == "sh" ||
				opcode == "sw" {
				// S-type instruction
				code, ok := a.parseSTypeInstruction(line, diff, i, opcode)
				if ok {
					a.ProgramText = append(a.ProgramText, code)
				}
			} else if opcode == "beq" ||
				opcode == "bne" ||
				opcode == "blt" ||
				opcode == "bge" ||
				opcode == "bltu" ||
				opcode == "bgeu" {
				// B-type instruction
				code, ok := a.parseBTypeInstruction(line, diff, i, opcode)
				if ok {
					a.ProgramText = append(a.ProgramText, code)
				}
			} else if opcode == "jal" {
				// J-type instruction
				code, ok := a.parseJTypeInstruction(line, diff, i, opcode)
				if ok {
					a.ProgramText = append(a.ProgramText, code)
				}
			} else if opcode == "lui" ||
				opcode == "auipc" {
				// U-type instruction
				code, ok := a.parseUTypeInstruction(line, diff, i, opcode)
				if ok {
					a.ProgramText = append(a.ProgramText, code)
				}
			} else if opcode == "ecall" ||
				opcode == "ebreak" {
				// I-type instruction, but with no operands
				code, ok := a.parseITypeInstructionWithoutArguments(line, diff, i, opcode)
				if ok {
					a.ProgramText = append(a.ProgramText, code)
				}
			} else {
				// invalid instruction
				a.Diagnostics = append(a.Diagnostics, Errors.InvalidInstruction(opcode, TextRange{
					Start: TextPosition{Line: i, Char: diff},
					End:   TextPosition{Line: i, Char: diff + len(opcode)},
				}))
			}
		} else {
			// data section
			// label will have already been removed, need to find it
			for label, lineNum := range a.LabelToLineNumber {
				if lineNum == i {
					a.Labels[label] = uint32(len(a.ProgramData) * 4)
					a.LabelTypes[label] = "data"
				}
			}

			if len(line) == 0 {
				continue
			}

			// now to find how much to allocate and what to allocate
			// format is one of
			//.word <value1>, <value2>, <value3>, ...
			//.ascii <string>
			//.space <size>
			//.alloc <size>

			dType := strings.ToLower(strings.Split(line, " ")[0])
			values := strings.Split(line[strings.Index(line, " ")+1:], ",")
			if dType == ".word" {
				charOffset := diff + len(dType) + 1
				for _, value := range values {
					evalRes, _ := a.EvaluateAndReportErrors(value, 64, false, i, charOffset)
					charOffset += len(value) + 1
					if evalRes.Type == EvaluationTypeLabel || evalRes.Type == EvaluationTypeRegister {
						// error
						a.Diagnostics = append(a.Diagnostics, Errors.InvalidDataSectionValue(value, TextRange{
							Start: TextPosition{Line: i, Char: charOffset},
							End:   TextPosition{Line: i, Char: charOffset + len(value)},
						}))
					}
					a.ProgramData = append(a.ProgramData, uint32(evalRes.Value))
				}
			} else if dType == ".ascii" {
				charOffset := diff + len(dType) + 1

				// remove quotes
				value := line[strings.Index(line, " ")+1:]
				if value[0] != '"' || value[len(value)-1] != '"' {
					// error
					a.Diagnostics = append(a.Diagnostics, Errors.InvalidDataSectionValue(value, TextRange{
						Start: TextPosition{Line: i, Char: charOffset},
						End:   TextPosition{Line: i, Char: charOffset + len(value)},
					}))
					continue
				}

				value = value[1 : len(value)-1]
				valueB := []byte(strings.ReplaceAll(value, "\\n", "\n"))
				valueB = append(valueB, 0)
				for i, char := range valueB {
					// pack 4 chars into a word
					if i%4 == 0 {
						a.ProgramData = append(a.ProgramData, uint32(char))
					} else {
						a.ProgramData[len(a.ProgramData)-1] |= uint32(char) << uint32((i%4)*8)
					}
				}
			} else if dType == ".space" {
				offset := diff + len(dType) + 1
				evalRes, ok := a.EvaluateAndReportErrors(values[0], 64, false, i, offset)
				if ok {
					if evalRes.Type == EvaluationTypeLabel || evalRes.Type == EvaluationTypeRegister {
						// error
						a.Diagnostics = append(a.Diagnostics, Errors.InvalidDataSectionValue(values[0], TextRange{
							Start: TextPosition{Line: i, Char: offset},
							End:   TextPosition{Line: i, Char: offset + len(values[0])},
						}))
					}
					for i := 0; i < int(math.Ceil(float64(evalRes.Value)/4)); i++ {
						a.ProgramData = append(a.ProgramData, 0)
					}
				}
			} else if dType == ".alloc" {
				offset := diff + len(dType) + 1
				evalRes, ok := a.EvaluateAndReportErrors(values[0], 64, false, i, offset)
				if ok {
					if evalRes.Type == EvaluationTypeLabel || evalRes.Type == EvaluationTypeRegister {
						// error
						a.Diagnostics = append(a.Diagnostics, Errors.InvalidDataSectionValue(values[0], TextRange{
							Start: TextPosition{Line: i, Char: offset},
							End:   TextPosition{Line: i, Char: offset + len(values[0])},
						}))
					}
					for i := 0; i < int(evalRes.Value); i++ {
						a.ProgramData = append(a.ProgramData, 0)
					}
				}
			} else {
				// error
				a.Diagnostics = append(a.Diagnostics, Errors.InvalidDataSection(dType, TextRange{
					Start: TextPosition{Line: i, Char: diff},
					End:   TextPosition{Line: i, Char: diff + len(dType)},
				}))
			}
		}
	}
}

func Assemble(input string) (res *AssembledResult) {
	res = new(AssembledResult)
	res.Labels = make(map[string]uint32)
	res.LabelTypes = make(map[string]string)
	res.lineLengthDeltas = make(map[int]int)
	res.AddressToLine = make(map[uint32]int)
	res.LabelToLineNumber = make(map[string]int)
	res.fileContents = strings.Split(input, "\n")

	// extract labels so the line parser can determine which symbols are labels
	res.extractLabels()

	res.parseLines()

	res.resolveLabelLinkRequests()
	return
}
