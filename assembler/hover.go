package assembler

import (
	"fmt"
	"strconv"
	"strings"
)

func getImmediateValue(instruction uint32) int64 {
	opcode := GetOpCode(instruction)
	if opcode == OPCODE_ITYPE || opcode == OPCODE_MEMITYPE || opcode == OPCODE_JALR {
		// I type
		_, _, _, imm, _ := DecodeITypeInstruction(instruction)
		return int64(int16(imm<<4) >> 4)
	} else if opcode == OPCODE_JAL {
		// J type
		_, _, imm := DecodeJTypeInstruction(instruction)
		return int64(int32(imm<<11) >> 11)
	} else if opcode == OPCODE_STYPE {
		// S type
		_, _, _, imm, _ := DecodeSTypeInstruction(instruction)
		return int64(int16(imm<<4) >> 4)
	} else if opcode == OPCODE_BTYPE {
		// B type
		_, _, _, imm, _ := DecodeBTypeInstruction(instruction)
		return int64(int16(imm<<3) >> 3)
	} else if opcode == OPCODE_LUI || opcode == OPCODE_AUIPC {
		// U type
		_, _, imm := DecodeUTypeInstruction(instruction)
		return int64(imm)
	}

	return 0
}

func (a *AssembledResult) EvaluateHover(position TextPosition) (string, bool) {
	// returns markdown
	// returns true if there is a hover
	// returns false if there is no hover

	line := a.fileContents[position.Line]

	// removing comments
	commentIndex := strings.Index(line, "#")
	if commentIndex != -1 {
		line = line[:commentIndex]
	}

	if position.Char < a.lineLengthDeltas[position.Line] {
		// the hover is over a label definition
		labelAtLine := ""
		for k, v := range a.LabelToLineNumber {
			if v == position.Line {
				labelAtLine = k
				break
			}
		}

		labelValueType := "Offset"
		if a.LabelTypes[labelAtLine] == "text" {
			labelValueType = "Address"
		}

		return fmt.Sprintf(hoverInfoFormats.labelDefinition, labelAtLine, labelValueType, a.Labels[labelAtLine]), true
	}

	isInstruction := false
	address := uint32(0)
	for a, v := range a.AddressToLine {
		if v == position.Line {
			isInstruction = true
			address = a
			break
		}
	}

	if isInstruction {
		// the hover is over an instruction, but must figure out whether its a label, literal, register, or instruction opcode
		opcodeStart := 0

		diff := a.lineLengthDeltas[position.Line]
		lineTrimmed := strings.TrimLeft(line, " \t")
		diff += len(line) - len(lineTrimmed)
		line = lineTrimmed

		if position.Char < diff {
			return "", false
		}

		// opcode is delimited by a space
		opcodeEnd := strings.Index(line, " ")
		if opcodeEnd == -1 {
			opcodeEnd = len(line)
		}

		if opcodeEnd == -1+opcodeStart {
			if position.Char-diff > opcodeStart {
				// the hover is over an instruction opcode
				opcode := line[opcodeStart:opcodeEnd]
				return getHoverInfoForInstruction(opcode), true
			}
			return "", false
		} else if position.Char-diff < opcodeEnd && position.Char-diff >= opcodeStart {
			// the hover is over an instruction opcode
			opcode := line[opcodeStart:opcodeEnd]
			return getHoverInfoForInstruction(opcode), true
		}

		// now to figure out which operand it is
		line = strings.ReplaceAll(line, "(", ",")
		line = strings.ReplaceAll(line, ")", "")
		operands := strings.Split(line[opcodeEnd:], ",")
		pos := opcodeEnd + 1
		for _, v := range operands {
			if position.Char-diff < pos+len(v) {
				// the hover is over this operand
				evRes, err := a.Evaluate(v, 12, true) 
				if err != nil {
					evRes, err = a.Evaluate(v, 32, false)
					if err != nil {
						return "", false
					}
				}

				if evRes.Type == EvaluationTypeLabel {
					return fmt.Sprintf(hoverInfoFormats.labelReference, evRes.MatchedValue, getImmediateValue(a.ProgramText[address/4])), true
				} else if evRes.Type == EvaluationTypeIntegerLiteral || evRes.Type == EvaluationTypeUnsignedIntegerLiteral {
					if evRes.Value < 0 {
						return fmt.Sprintf(hoverInfoFormats.integerLiteral, evRes.Value, "0x"+strconv.FormatUint(uint64(evRes.Value)&0xFFFFFFFF, 16)), true
					}
					return fmt.Sprintf(hoverInfoFormats.integerLiteral, evRes.Value, "0x"+strconv.FormatInt(evRes.Value, 16)), true
				} else if evRes.Type == EvaluationTypeRegister {
					return getHoverInfoForRegister(int(evRes.Value), evRes.MatchedValue), true
				}
			}
			pos += len(v) + 1
		}
	}

	return "", false
}

func getHoverInfoForInstruction(opcode string) string {
	opcode = strings.TrimSpace(strings.ToLower(opcode))
	switch opcode {
	case "add":
		return hoverInfoFormats.add
	case "sub":
		return hoverInfoFormats.sub
	case "xor":
		return hoverInfoFormats.xor
	case "or":
		return hoverInfoFormats.or
	case "and":
		return hoverInfoFormats.and
	case "sll":
		return hoverInfoFormats.sll
	case "srl":
		return hoverInfoFormats.srl
	case "sra":
		return hoverInfoFormats.sra
	case "slt":
		return hoverInfoFormats.slt
	case "sltu":
		return hoverInfoFormats.sltu
	case "addi":
		return hoverInfoFormats.addi
	case "xori":
		return hoverInfoFormats.xori
	case "ori":
		return hoverInfoFormats.ori
	case "andi":
		return hoverInfoFormats.andi
	case "slli":
		return hoverInfoFormats.slli
	case "srli":
		return hoverInfoFormats.srli
	case "srai":
		return hoverInfoFormats.srai
	case "slti":
		return hoverInfoFormats.slti
	case "sltiu":
		return hoverInfoFormats.sltiu
	case "lb":
		return hoverInfoFormats.lb
	case "lh":
		return hoverInfoFormats.lh
	case "lw":
		return hoverInfoFormats.lw
	case "lbu":
		return hoverInfoFormats.lbu
	case "lhu":
		return hoverInfoFormats.lhu
	case "sb":
		return hoverInfoFormats.sb
	case "sh":
		return hoverInfoFormats.sh
	case "sw":
		return hoverInfoFormats.sw
	case "beq":
		return hoverInfoFormats.beq
	case "bne":
		return hoverInfoFormats.bne
	case "blt":
		return hoverInfoFormats.blt
	case "bge":
		return hoverInfoFormats.bge
	case "bltu":
		return hoverInfoFormats.bltu
	case "bgeu":
		return hoverInfoFormats.bgeu
	case "jal":
		return hoverInfoFormats.jal
	case "jalr":
		return hoverInfoFormats.jalr
	case "lui":
		return hoverInfoFormats.lui
	case "auipc":
		return hoverInfoFormats.auipc
	case "ecall":
		return hoverInfoFormats.ecall
	case "ebreak":
		return hoverInfoFormats.ebreak
	case "mul":
		return hoverInfoFormats.mul
	case "mulh":
		return hoverInfoFormats.mulh
	case "mulhsu":
		return hoverInfoFormats.mulhsu
	case "mulhu":
		return hoverInfoFormats.mulhu
	case "mulu":
		return hoverInfoFormats.mulhu
	case "div":
		return hoverInfoFormats.div
	case "divu":
		return hoverInfoFormats.divu
	case "rem":
		return hoverInfoFormats.rem
	case "remu":
		return hoverInfoFormats.remu
	}
	return ""
}

func getHoverInfoForRegister(register int, name string) string {
	if register == 0 {
		return hoverInfoFormats.zeroRegister
	} else if register == 1 {
		return hoverInfoFormats.raRegister
	} else if register == 2 {
		return hoverInfoFormats.spRegister
	} else if register == 3 {
		return hoverInfoFormats.gpRegister
	} else if register == 4 {
		return hoverInfoFormats.tpRegister
	} else {
		name = strings.TrimSpace(strings.ToLower(name))
		if !strings.HasPrefix(name, "x") {
			return fmt.Sprintf(hoverInfoFormats.namedGenericRegister, name, register)
		}
		return fmt.Sprintf(hoverInfoFormats.genericRegister, register)
	}
}
