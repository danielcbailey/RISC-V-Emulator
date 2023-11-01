package assembler

func makeRTypeInstruction(opcode, rd, rs1, rs2, func7, func3 uint32) uint32 {
	return (func7 << 25) | (rs2 << 20) | (rs1 << 15) | (func3 << 12) | (rd << 7) | opcode
}

func makeITypeInstruction(opcode, rd, rs1, imm, func3 uint32) uint32 {
	imm = imm & 0xFFF
	return (imm << 20) | (rs1 << 15) | (func3 << 12) | (rd << 7) | opcode
}

func makeSTypeInstruction(opcode, rs1, rs2, imm, func3 uint32) uint32 {
	imm = imm & 0xFFF
	return ((imm >> 5) << 25) | (rs2 << 20) | (rs1 << 15) | (func3 << 12) | ((imm & 0x1F) << 7) | opcode
}

func makeBTypeInstruction(opcode, rs1, rs2, imm, func3 uint32) uint32 {
	imm = imm & 0x1FFF
	// expects immediate to be in the amount of bytes to jump, *not* adjusted by 2

	instr := (rs2 << 20) | (rs1 << 15) | (func3 << 12) | opcode
	// immediate is stored in a very convoluted way
	instr |= ((imm >> 12) & 0x1) << 31
	instr |= ((imm >> 11) & 0x1) << 7
	instr |= ((imm >> 5) & 0x3F) << 25
	instr |= ((imm >> 1) & 0xF) << 8

	return instr
}

func makeUTypeInstruction(opcode, rd, imm uint32) uint32 {
	imm = imm & 0xFFFFF
	return (imm << 12) | (rd << 7) | opcode
}

func makeJTypeInstruction(opcode, rd, imm uint32) uint32 {
	imm = imm & 0x1FFFFF
	// expects immediate to be in the amount of bytes to jump, *not* adjusted by 2

	instr := (rd << 7) | opcode
	// immediate is stored in a very convoluted way
	instr |= ((imm >> 20) & 0x1) << 31
	instr |= ((imm >> 1) & 0x3FF) << 21
	instr |= ((imm >> 11) & 0x1) << 20
	instr |= ((imm >> 12) & 0xFF) << 12

	return instr
}

func DecodeRTypeInstruction(instruction uint32) (opcode, rd, rs1, rs2, func7, func3 uint32) {
	opcode = instruction & 0x7F
	rd = (instruction >> 7) & 0x1F
	func3 = (instruction >> 12) & 0x7
	rs1 = (instruction >> 15) & 0x1F
	rs2 = (instruction >> 20) & 0x1F
	func7 = (instruction >> 25) & 0x7F
	return
}

func DecodeITypeInstruction(instruction uint32) (opcode, rd, rs1, imm, func3 uint32) {
	opcode = instruction & 0x7F
	rd = (instruction >> 7) & 0x1F
	func3 = (instruction >> 12) & 0x7
	rs1 = (instruction >> 15) & 0x1F
	imm = (instruction >> 20) & 0xFFF
	return
}

func DecodeSTypeInstruction(instruction uint32) (opcode, rs1, rs2, imm, func3 uint32) {
	opcode = instruction & 0x7F
	func3 = (instruction >> 12) & 0x7
	rs1 = (instruction >> 15) & 0x1F
	rs2 = (instruction >> 20) & 0x1F
	imm = (((instruction >> 25) & 0x7F) << 5) | ((instruction >> 7) & 0x1F)
	return
}

func DecodeBTypeInstruction(instruction uint32) (opcode, rs1, rs2, imm, func3 uint32) {
	opcode = instruction & 0x7F
	func3 = (instruction >> 12) & 0x7
	rs1 = (instruction >> 15) & 0x1F
	rs2 = (instruction >> 20) & 0x1F
	imm = ((instruction >> 31) & 0x1) << 12
	imm |= ((instruction >> 7) & 0x1) << 11
	imm |= ((instruction >> 25) & 0x3F) << 5
	imm |= ((instruction >> 8) & 0xF) << 1
	return
}

func DecodeUTypeInstruction(instruction uint32) (opcode, rd, imm uint32) {
	opcode = instruction & 0x7F
	rd = (instruction >> 7) & 0x1F
	imm = (instruction >> 12) & 0xFFFFF
	return
}

func DecodeJTypeInstruction(instruction uint32) (opcode, rd, imm uint32) {
	opcode = instruction & 0x7F
	rd = (instruction >> 7) & 0x1F
	imm = ((instruction >> 31) & 0x1) << 20
	imm |= ((instruction >> 21) & 0x3FF) << 1
	imm |= ((instruction >> 20) & 0x1) << 11
	imm |= ((instruction >> 12) & 0xFF) << 12
	return
}

func GetOpCode(instruction uint32) uint32 {
	return instruction & 0x7F
}

// opcode conversions
const (
	OPCODE_RTYPE    = 0b0110011
	OPCODE_ITYPE    = 0b0010011
	OPCODE_STYPE    = 0b0100011
	OPCODE_BTYPE    = 0b1100011
	OPCODE_LUI      = 0b0110111
	OPCODE_AUIPC    = 0b0010111
	OPCODE_JAL      = 0b1101111
	OPCODE_JALR     = 0b1100111
	OPCODE_MEMITYPE = 0b0000011
	OPCODE_ENV      = 0b1110011
)
