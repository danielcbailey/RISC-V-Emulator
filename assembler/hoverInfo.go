package assembler

type hoverInfoFormatsType struct {
	labelDefinition string
	labelReference  string
	integerLiteral  string

	// registers
	zeroRegister         string
	raRegister           string
	spRegister           string
	gpRegister           string
	tpRegister           string
	namedGenericRegister string
	genericRegister      string

	// instructions
	add  string
	sub  string
	xor  string
	or   string
	and  string
	sll  string
	srl  string
	sra  string
	slt  string
	sltu string

	addi  string
	xori  string
	ori   string
	andi  string
	slli  string
	srli  string
	srai  string
	slti  string
	sltiu string

	lb  string
	lh  string
	lw  string
	lbu string
	lhu string

	sb string
	sh string
	sw string

	beq  string
	bne  string
	blt  string
	bge  string
	bltu string
	bgeu string

	jal  string
	jalr string

	auipc string
	lui   string

	ecall  string
	ebreak string

	mul    string
	mulh   string
	mulhsu string
	mulhu  string
	div    string
	divu   string
	rem    string
	remu   string
}

var hoverInfoFormats = hoverInfoFormatsType{
	labelDefinition: "Definition of label `%s`.\n\n %s of 0x%X",
	labelReference:  "Reference to label `%s`\n\nEvaluates to `%d`",
	integerLiteral:  "Integer Literal `%d` (`%s`)",

	zeroRegister:         "Zero Register `zero` (`x0`)\n\nAlways evaluates to `0`",
	raRegister:           "Return Address Register `ra` (`x1`)\n\nContains the return address of the current function",
	spRegister:           "Stack Pointer Register `sp` (`x2`)\n\nContains the address of the top of the stack",
	gpRegister:           "Global Pointer Register `gp` (`x3`)\n\nContains the address of the start of the global data segment",
	tpRegister:           "Thread Pointer Register `tp` (`x4`)\n\nContains the address of the thread-local storage segment",
	genericRegister:      "Register `x%d`. 32-Bit General Purpose Register",
	namedGenericRegister: "Register `%s` (`x%d`). 32-Bit General Purpose Register",

	add:  "Addition Instruction.\n\nFormat: `add <dst reg>, <src reg>, <src reg>`\n\nExample: `add x10, x11, x12` is the same as `x10 = x11 + x12`",
	sub:  "Subtraction Instruction.\n\nFormat: `sub <dst reg>, <src reg>, <src reg>`\n\nExample: `sub x10, x11, x12` is the same as `x10 = x11 - x12`",
	xor:  "XOR Instruction.\n\nFormat: `xor <dst reg>, <src reg>, <src reg>`\n\nExample: `xor x10, x11, x12` is the same as `x10 = x11 ^ x12`",
	or:   "OR Instruction.\n\nFormat: `or <dst reg>, <src reg>, <src reg>`\n\nExample: `or x10, x11, x12` is the same as `x10 = x11 | x12`",
	and:  "AND Instruction.\n\nFormat: `and <dst reg>, <src reg>, <src reg>`\n\nExample: `and x10, x11, x12` is the same as `x10 = x11 & x12`",
	sll:  "Shift Left Logical Instruction.\n\nFormat: `sll <dst reg>, <src reg>, <amt reg>`\n\nExample: `sll x10, x11, x12` is the same as `x10 = x11 << x12`",
	srl:  "Shift Right Logical Instruction.\n\nFormat: `srl <dst reg>, <src reg>, <amt reg>`\n\nExample: `srl x10, x11, x12` is the same as `x10 = x11 >> x12`",
	sra:  "Shift Right Arithmetic Instruction.\n\nFormat: `sra <dst reg>, <src reg>, <amt reg>`\n\nExample: `sra x10, x11, x12` is the same as `x10 = x11 >> x12`\n\nNote, however, that this looks the same as `srl`, but the most-significant bit will be copied for each bit shifted.",
	slt:  "Set Less Than Instruction.\n\nFormat: `slt <dst reg>, <src reg>, <src reg>`\n\nExample: `slt x10, x11, x12` is the same as `x10 = x11 < x12`\n\nIf `x11 < x12`, then `x10` will be set to `1`, otherwise it will be set to `0`",
	sltu: "Set Less Than Unsigned Instruction.\n\nFormat: `sltu <dst reg>, <src reg>, <src reg>`\n\nExample: `sltu x10, x11, x12` is the same as `x10 = x11 < x12`\n\nIf `x11 < x12`, then `x10` will be set to `1`, otherwise it will be set to `0`\n\nNote that this is an unsigned comparison.",

	addi:  "Addition Immediate Instruction.\n\nFormat: `addi <dst reg>, <src reg>, <imm>`\n\nExample: `addi x10, x11, 2035` is the same as `x10 = x11 + 2035`\n\nNote that the immediate is a signed 12-bit value, so it must be between -2048 and 2047.",
	xori:  "XOR Immediate Instruction.\n\nFormat: `xori <dst reg>, <src reg>, <imm>`\n\nExample: `xori x10, x11, 2035` is the same as `x10 = x11 ^ 2035`\n\nNote that the immediate is an unsigned 12-bit value, so it must be between 0 and 4095.",
	ori:   "OR Immediate Instruction.\n\nFormat: `ori <dst reg>, <src reg>, <imm>`\n\nExample: `ori x10, x11, 2035` is the same as `x10 = x11 | 2035`\n\nNote that the immediate is an unsigned 12-bit value, so it must be between 0 and 4095.",
	andi:  "AND Immediate Instruction.\n\nFormat: `andi <dst reg>, <src reg>, <imm>`\n\nExample: `andi x10, x11, 2035` is the same as `x10 = x11 & 2035`\n\nNote that the immediate is an unsigned 12-bit value, so it must be between 0 and 4095.",
	slli:  "Shift Left Logical Immediate Instruction.\n\nFormat: `slli <dst reg>, <src reg>, <amt>`\n\nExample: `slli x10, x11, 5` is the same as `x10 = x11 << 5`\n\nNote that the immediate is an unsigned 5-bit value, so it must be between 0 and 31.",
	srli:  "Shift Right Logical Immediate Instruction.\n\nFormat: `srli <dst reg>, <src reg>, <amt>`\n\nExample: `srli x10, x11, 5` is the same as `x10 = x11 >> 5`\n\nNote that the immediate is an unsigned 5-bit value, so it must be between 0 and 31.",
	srai:  "Shift Right Arithmetic Immediate Instruction.\n\nFormat: `srai <dst reg>, <src reg>, <amt>`\n\nExample: `srai x10, x11, 5` is the same as `x10 = x11 >> 5`\n\nNote that the immediate is a signed 5-bit value, so it must be between 0 and 31.\n\nNote, however, that unlike `slri`, the most-significant bit will be copied for each bit shifted so that the sign is preserved.",
	slti:  "Set Less Than Immediate Instruction.\n\nFormat: `slti <dst reg>, <src reg>, <imm>`\n\nExample: `slti x10, x11, 2035` is the same as `x10 = x11 < 2035`\n\nIf `x11 < 2035`, then `x10` will be set to `1`, otherwise it will be set to `0`\n\nNote that the immediate is a signed 12-bit value, so it must be between -2048 and 2047.",
	sltiu: "Set Less Than Unsigned Immediate Instruction.\n\nFormat: `sltiu <dst reg>, <src reg>, <imm>`\n\nExample: `sltiu x10, x11, 2035` is the same as `x10 = x11 < 2035`\n\nIf `x11 < 2035`, then `x10` will be set to `1`, otherwise it will be set to `0`\n\nNote that the immediate is an unsigned 12-bit value, so it must be between 0 and 4095.\n\nNote that this is an unsigned comparison.",

	lb:  "Load Byte Instruction.\n\nFormat: `lb <dst reg>, <imm>(<src reg>)`\n\nExample: `lb x10, 2035(x11)` is the same as `x10 = mem[x11 + 2035]`\n\nNote that the immediate is a signed 12-bit value, so it must be between -2048 and 2047. This is a signed operation, so the loaded value **will** be sign extended",
	lh:  "Load Halfword Instruction.\n\nFormat: `lh <dst reg>, <imm>(<src reg>)`\n\nExample: `lh x10, 2035(x11)` is the same as `x10 = mem[x11 + 2035]`\n\nNote that the immediate is a signed 12-bit value, so it must be between -2048 and 2047. This is a signed operation, so the loaded value **will** be sign extended",
	lw:  "Load Word Instruction.\n\nFormat: `lw <dst reg>, <imm>(<src reg>)`\n\nExample: `lw x10, 2035(x11)` is the same as `x10 = mem[x11 + 2035]`\n\nNote that the immediate is a signed 12-bit value, so it must be between -2048 and 2047. This is a signed operation, so the loaded value **will** be sign extended",
	lbu: "Load Byte Unsigned Instruction.\n\nFormat: `lbu <dst reg>, <imm>(<src reg>)`\n\nExample: `lbu x10, 2035(x11)` is the same as `x10 = mem[x11 + 2035]`\n\nNote that the immediate is a signed 12-bit value, so it must be between -2048 and 2047. This is an unsigned operation, so the loaded value **will not** be sign extended",
	lhu: "Load Halfword Unsigned Instruction.\n\nFormat: `lhu <dst reg>, <imm>(<src reg>)`\n\nExample: `lhu x10, 2035(x11)` is the same as `x10 = mem[x11 + 2035]`\n\nNote that the immediate is a signed 12-bit value, so it must be between -2048 and 2047. This is an unsigned operation, so the loaded value **will not** be sign extended",

	sb: "Store Byte Instruction.\n\nFormat: `sb <src reg>, <imm>(<dst reg>)`\n\nExample: `sb x10, 2035(x11)` is the same as `mem[x11 + 2035] = x10`\n\nNote that the immediate is a signed 12-bit value, so it must be between -2048 and 2047.",
	sh: "Store Halfword Instruction.\n\nFormat: `sh <src reg>, <imm>(<dst reg>)`\n\nExample: `sh x10, 2035(x11)` is the same as `mem[x11 + 2035] = x10`\n\nNote that the immediate is a signed 12-bit value, so it must be between -2048 and 2047.",
	sw: "Store Word Instruction.\n\nFormat: `sw <src reg>, <imm>(<dst reg>)`\n\nExample: `sw x10, 2035(x11)` is the same as `mem[x11 + 2035] = x10`\n\nNote that the immediate is a signed 12-bit value, so it must be between -2048 and 2047.",

	beq:  "Branch Equal Instruction.\n\nFormat: `beq <src reg 1>, <src reg 2>, <imm>`\n\nExample: `beq x10, x11, 40` is the same as `if x10 == x11 { pc += 40 }`\n\nThe `<imm>` specifies the number of bytes away to branch. It is encoded in 12 bits as `<imm>/2` (a signed offset in multiples of 2 bytes), allowing `<imm>` to range from -4096 to 4095 bytes.\n\nAn instruction label may be used as `<imm>`.",
	bne:  "Branch Not Equal Instruction.\n\nFormat: `bne <src reg 1>, <src reg 2>, <imm>`\n\nExample: `bne x10, x11, 40` is the same as `if x10 != x11 { pc += 40 }`\n\nThe `<imm>` specifies the number of bytes away to branch. It is encoded in 12 bits as `<imm>/2`(a signed offset in multiples of 2 bytes), allowing `<imm>` to range from -4096 to 4095 bytes.\n\nAn instruction label may be used as `<imm>`.",
	blt:  "Branch Less Than Instruction.\n\nFormat: `blt <src reg 1>, <src reg 2>, <imm>`\n\nExample: `blt x10, x11, 40` is the same as `if x10 < x11 { pc += 40 }`\n\nThe `<imm>` specifies the number of bytes away to branch. It is encoded in 12 bits as `<imm>/2`(a signed offset in multiples of 2 bytes), allowing `<imm>` to range from -4096 to 4095 bytes.\n\nAn instruction label may be used as `<imm>`.",
	bge:  "Branch Greater Than or Equal Instruction.\n\nFormat: `bge <src reg 1>, <src reg 2>, <imm>`\n\nExample: `bge x10, x11, 40` is the same as `if x10 >= x11 { pc += 40 }`\n\nThe `<imm>` specifies the number of bytes away to branch. It is encoded in 12 bits as `<imm>/2`(a signed offset in multiples of 2 bytes), allowing `<imm>` to range from -4096 to 4095 bytes.\n\nAn instruction label may be used as `<imm>`.",
	bltu: "Branch Less Than Unsigned Instruction.\n\nFormat: `bltu <src reg 1>, <src reg 2>, <imm>`\n\nExample: `bltu x10, x11, 40` is the same as `if x10 < x11 { pc += 40 }`\n\nThe `<imm>` specifies the number of bytes away to branch. It is encoded in 12 bits as `<imm>/2`(a signed offset in multiples of 2 bytes), allowing `<imm>` to range from -4096 to 4095 bytes.\n\nAn instruction label may be used as `<imm>`.",
	bgeu: "Branch Greater Than or Equal Unsigned Instruction.\n\nFormat: `bgeu <src reg 1>, <src reg 2>, <imm>`\n\nExample: `bgeu x10, x11, 40` is the same as `if x10 >= x11 { pc += 40 }`\n\nThe `<imm>` specifies the number of bytes away to branch. It is encoded in 12 bits as `<imm>/2`(a signed offset in multiples of 2 bytes), allowing `<imm>` to range from -4096 to 4095 bytes.\n\nAn instruction label may be used as `<imm>`.",

	jal:  "Jump and Link Instruction.\n\nFormat: `jal <dst reg> <imm>`\n\nExample: `jal x1, 40` is the same as `x1 = pc; pc += 40`\n\nThe immediate is encoded in 20-bits as `<imm>/2` (a signed offset in multiples of 2 bytes). So the jump target offset `<imm>` range is +/- 1M.\n\nIf `<imm>` is an instruction label, pc = address of labeled instruction.",
	jalr: "Jump and Link Register Instruction.\n\nFormat: `jalr <dst reg>, <src reg>, <imm>`\n\nExample: `jalr x1, x10, 40` is the same as `x1 = pc; pc = x10 + 40`\n\nNote that the immediate is a signed 12-bit value, so it must be between -2048 and 2047.",

	lui:   "Load Upper Immediate Instruction.\n\nFormat: `lui <dst reg>, <imm>`\n\nExample: `lui x10, 0x12345` is the same as `x10 = 0x12345000`\n\nNote that the immediate is a 20-bit value.",
	auipc: "Add Upper Immediate to PC Instruction.\n\nFormat: `auipc <dst reg>, <imm>`\n\nExample: `auipc x10, 0x12345` is the same as `x10 = pc + 0x12345000`\n\nNote that the immediate is a 20-bit value.",

	ecall:  "Environment Call Instruction.\n\nFormat: `ecall`\n\nExample: `ecall` is the same as `syscall(17, x10, x11, x12, x13, x14, x15)`\n\nNote that the syscall number is always 17, and the arguments are in x10-x15.",
	ebreak: "Environment Break Instruction.\n\nFormat: `ebreak`\n\nExample: `ebreak` will trigger a breakpoint exception. This is not recommended because the development environment allows for hardware breakpoints to be set.",

	// RV32M
	mul:    "Multiply Instruction.\n\nFormat: `mul <dst reg>, <src reg 1>, <src reg 2>`\n\nExample: `mul x10, x11, x12` is the same as `x10 = uint32_t(x11 * x12)`",
	mulh:   "Multiply High Instruction.\n\nFormat: `mulh <dst reg>, <src reg 1>, <src reg 2>`\n\nExample: `mulh x10, x11, x12` is the same as `x10 = int64_t(int32_t(x11)) * int64_t(int32_t(x12)) >> 32`",
	mulhsu: "Multiply High Signed Unsigned Instruction.\n\nFormat: `mulhsu <dst reg>, <src reg 1>, <src reg 2>`\n\nExample: `mulhsu x10, x11, x12` is the same as `x10 = int64_t(int32_t(x11)) * int64_t(uint32_t(x12)) >> 32`.\n\nNote that the second argument is treated as unsigned.",
	mulhu:  "Multiply High Unsigned Instruction.\n\nFormat: `mulhu <dst reg>, <src reg 1>, <src reg 2>`\n\nExample: `mulhu x10, x11, x12` is the same as `x10 = int64_t(uint32_t(x11)) * int64_t(uint32_t(x12)) >> 32`.\n\nNote that both arguments are treated as unsigned.",

	div:  "Divide Instruction.\n\nFormat: `div <dst reg>, <src reg 1>, <src reg 2>`\n\nExample: `div x10, x11, x12` is the same as `x10 = int32_t(x11) / int32_t(x12)`",
	divu: "Divide Unsigned Instruction.\n\nFormat: `divu <dst reg>, <src reg 1>, <src reg 2>`\n\nExample: `divu x10, x11, x12` is the same as `x10 = x11 / x12`.\n\nNote that both arguments are treated as unsigned.",
	rem:  "Remainder Instruction.\n\nFormat: `rem <dst reg>, <src reg 1>, <src reg 2>`\n\nExample: `rem x10, x11, x12` is the same as `x10 = int32_t(x11) % int32_t(x12)`",
	remu: "Remainder Unsigned Instruction.\n\nFormat: `remu <dst reg>, <src reg 1>, <src reg 2>`\n\nExample: `remu x10, x11, x12` is the same as `x10 = x11 % x12`.\n\nNote that both arguments are treated as unsigned.",
}
