package emulator

import (
	"strconv"

	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/assembler"
	"fmt"
)

func (inst *EmulatorInstance) regRead(reg uint32) uint32 {
	// checking if the register is valid
	if inst.regInit&(1<<reg) == 0 && (inst.pc < inst.profileIgnoreRangeStart || inst.pc >= inst.profileIgnoreRangeEnd) {
		inst.newRegisterAccessedBeforeInitializedException(reg)
		return 0
	}

	if inst.pc < inst.profileIgnoreRangeStart || inst.pc >= inst.profileIgnoreRangeEnd {
		inst.lastUsedRegisters[int(reg)] = int(reg)
	}

	return inst.registers[reg]
}

func (inst *EmulatorInstance) regWrite(reg uint32, value uint32) {
	// setting valid bit
	if inst.pc < inst.profileIgnoreRangeStart || inst.pc >= inst.profileIgnoreRangeEnd {
		if inst.regInit&(1<<reg) == 0 {
			inst.regUsage++
		}
		inst.regInit |= 1 << reg
		inst.lastUsedRegisters[int(reg)] = int(reg)
		if bp, ok := inst.registerBreakpoints[int(reg)]; ok {
			inst.breakCallback(inst, bp.ID, "data breakpoint")
		}
	}

	if reg == 0 {
		// x0 is read-only
		// if code is outside of the profile ignore range, throw an exception
		if inst.pc < inst.profileIgnoreRangeStart || inst.pc >= inst.profileIgnoreRangeEnd {
			inst.newIllegalRegisterWrite()
		}
		return
	}

	// writing value
	inst.registers[reg] = value
}

func (inst *EmulatorInstance) resetLastUsedRegisters() {
	// keep all the registers that are in the a0-a7 range
	// a0 is 10, a7 is 17
	for i := 0; i < 32; i++ {
		if i < 10 || i > 17 {
			delete(inst.lastUsedRegisters, i)
		}
	}
}

func (inst *EmulatorInstance) Emulate(startAddr uint32) {
	// using the state of the registers already in the instance
	// this makes it possible to start, pause, and restart the emulator

	// setting the program counter
	inst.pc = startAddr - 4
	// setting i and d cache to first valid block
	for _, block := range inst.memory.Blocks {
		if block != nil {
			inst.iCache = block
			break
		}
	}

	// setting d cache to first valid block
	for _, block := range inst.memory.Blocks {
		if block != nil {
			inst.dCache = block
			break
		}
	}

	for inst.di < inst.runtimeLimit && !inst.terminated {
		inst.pc += 4
		if inst.pc == 0x20352035 || inst.pc == 0x20352034 {
			// end the emulator when magic number is reached (0x20352035 is the return address
			// of the main program)
			break
		} else if inst.pc == 0x20352037 {
			// magic number to resume from an interrupt
			inst.pc = inst.interrupt.pc
			for i := 0; i < 32; i++ {
				inst.registers[i] = inst.interrupt.registers[i]
			}
			inst.callStack = make([]uint32, len(inst.interrupt.callStack))
			copy(inst.callStack, inst.interrupt.callStack)
			inst.interrupt = nil
		}

		// checking for interrupts
		if inst.interrupt != nil {
			// need to interrupt, save the current state
			inst.interrupt.pc = inst.pc

			// saving registers
			for i := 0; i < 32; i++ {
				inst.interrupt.registers[i] = inst.registers[i]
			}

			// saving call stack
			inst.interrupt.callStack = make([]uint32, len(inst.callStack))
			copy(inst.interrupt.callStack, inst.callStack)

			inst.userGlobalPointer = inst.registers[3]
			inst.registers[3] = inst.osGlobalPointer
			inst.isInOSCode = true
			inst.registers[1] = 0x20352037 // 0x20352037 is the magic number for the RISC-V emulator to know when to resume from an interrupt
		}

		if inst.pc < inst.profileIgnoreRangeStart || inst.pc >= inst.profileIgnoreRangeEnd {
			inst.di++

			// checking if should break - this is only done when profiling
			inst.checkShouldBreak()
		}

		if inst.isInOSCode {
			// checking if code is outside the profile ignore range, thus indictating it is no longer in os code
			if inst.pc < inst.profileIgnoreRangeStart || inst.pc >= inst.profileIgnoreRangeEnd {
				inst.isInOSCode = false
				inst.registers[3] = inst.userGlobalPointer

				// restoring registers
				for i := 1; i < 32; i++ {
					inst.registers[i] = inst.registerPreservation[i]
				}

				if inst.wasEcall {
					// seeing if the registers were modified, they are on the stack
					for i := 0; i < 8; i++ {
						val := inst.memReadWord(inst.registers[2]-uint32(i*4), false)
						if val != inst.registers[i+10] {
							inst.registers[i+10] = val
							// setting the valid bit
							inst.regInit |= 1 << uint32(i+10)
							inst.lastUsedRegisters[i+10] = i + 10
						}
					}
				}
			}
		}

		// fetching next instruction
		instruction := inst.memReadWord(inst.pc, true)

		// decoding instruction
		opcode := assembler.GetOpCode(instruction)
		// executing instruction
		switch opcode {
		case assembler.OPCODE_LUI:
			inst.executeLUI(instruction)
		case assembler.OPCODE_AUIPC:
			inst.executeAUIPC(instruction)
		case assembler.OPCODE_JAL:
			inst.executeJAL(instruction)
		case assembler.OPCODE_JALR:
			inst.executeJALR(instruction)
		case assembler.OPCODE_BTYPE:
			inst.executeBType(instruction)
		case assembler.OPCODE_MEMITYPE:
			inst.executeMemIType(instruction)
		case assembler.OPCODE_ITYPE:
			inst.executeIType(instruction)
		case assembler.OPCODE_RTYPE:
			inst.executeRType(instruction)
		case assembler.OPCODE_STYPE:
			inst.executeSType(instruction)
		case assembler.OPCODE_ENV:
			inst.executeEnv(instruction)
		default:
			inst.newException("Unsupported opcode exception: %d", opcode)
		}

		inst.executedInstructions++
	}
	if (inst.di >= inst.runtimeLimit){
		sendOutput(fmt.Sprintf("***Infinite Loop? DI: %d***", inst.di), true)
	}
}

func (inst *EmulatorInstance) checkShouldBreak() {
	if inst.breakAddr == inst.pc || inst.breakNext {
		inst.breakNext = false
		inst.breakAddr = 0xFFFFFFFF
		if inst.breakCallback != nil {
			inst.breakCallback(inst, inst.breakpoints[inst.pc].ID, "breakpoint")
		}
	}

	bp, ok := inst.breakpoints[inst.pc]
	if !ok {
		return // no breakpoint at this address, so we don't care.
	}

	if bp.condition != "" {
		res, err := EvaluateExpression(bp.condition)
		if err != nil {
			inst.newException("Error evaluating breakpoint condition: %s", err.Error())
			return
		}

		if n, _ := strconv.Atoi(res.String); n != 0 || res.String == "true" {
			inst.breakNext = false
			inst.breakAddr = 0xFFFFFFFF
			if inst.breakCallback != nil {
				inst.breakCallback(inst, inst.breakpoints[inst.pc].ID, "breakpoint")
			}
		}

		return
	}

	inst.breakNext = false
	inst.breakAddr = 0xFFFFFFFF
	if inst.breakCallback != nil {
		inst.breakCallback(inst, inst.breakpoints[inst.pc].ID, "breakpoint")
	}
}

func (inst *EmulatorInstance) executeLUI(instruction uint32) {
	// decode the instruction
	_, rd, imm := assembler.DecodeUTypeInstruction(instruction)
	inst.regWrite(rd, imm<<12)
}

func (inst *EmulatorInstance) executeAUIPC(instruction uint32) {
	// decode the instruction
	_, rd, imm := assembler.DecodeUTypeInstruction(instruction)
	inst.regWrite(rd, (imm<<12)+inst.pc)
}

func (inst *EmulatorInstance) executeJAL(instruction uint32) {
	// decode the instruction
	_, rd, imm := assembler.DecodeJTypeInstruction(instruction)

	// setting the return address
	if rd != 0 {
		inst.regWrite(rd, inst.pc+4)
		if rd == 1 {
			inst.callStack = append(inst.callStack, inst.pc)
		}
	}

	// jumping to the new address
	inst.pc = uint32(int32(inst.pc)+int32(imm<<11)>>11) - 4 // the -4 is because the pc is incremented by 4 before the instruction is fetched
}

func (inst *EmulatorInstance) executeJALR(instruction uint32) {
	// decode the instruction
	_, rd, rs1, imm, _ := assembler.DecodeITypeInstruction(instruction)

	pcVal := inst.pc
	if rs1 == 1 {
		if len(inst.callStack) > 0 {
			inst.callStack = inst.callStack[:len(inst.callStack)-1]
			if inst.breakAddr != 0xFFFFFFFF {
				// all step operations should break if popping a stack frame
				inst.breakNext = true
			}
		}
	} else if rd == 1 {
		inst.callStack = append(inst.callStack, inst.pc)
	}

	// jumping to the new address
	inst.pc = (uint32(int32(inst.regRead(rs1))+int32(imm<<20)>>20) & 0xFFFFFFFE) - 4 // the -4 is because the pc is incremented by 4 before the instruction is fetched

	// setting the return address
	if rd != 0 {
		inst.regWrite(rd, pcVal+4)
	}
}

func (inst *EmulatorInstance) executeBType(instruction uint32) {
	opcode, rs1, rs2, imm, func3 := assembler.DecodeBTypeInstruction(instruction)
	immInt := int32(imm<<19) >> 19
	if opcode == 0b1100011 {
		switch func3 {
		case 0b000:
			// BEQ
			if inst.regRead(rs1) == inst.regRead(rs2) {
				inst.pc = uint32(int32(inst.pc)+immInt) - 4 // the -4 is because the pc is incremented by 4 before the instruction is fetched
			}
		case 0b001:
			// BNE
			if inst.regRead(rs1) != inst.regRead(rs2) {
				inst.pc = uint32(int32(inst.pc)+immInt) - 4 // the -4 is because the pc is incremented by 4 before the instruction is fetched
			}
		case 0b100:
			// BLT
			if int32(inst.regRead(rs1)) < int32(inst.regRead(rs2)) {
				inst.pc = uint32(int32(inst.pc)+immInt) - 4 // the -4 is because the pc is incremented by 4 before the instruction is fetched
			}
		case 0b101:
			// BGE
			if int32(inst.regRead(rs1)) >= int32(inst.regRead(rs2)) {
				inst.pc = uint32(int32(inst.pc)+immInt) - 4 // the -4 is because the pc is incremented by 4 before the instruction is fetched
			}
		case 0b110:
			// BLTU
			if inst.regRead(rs1) < inst.regRead(rs2) {
				inst.pc = uint32(int32(inst.pc)+immInt) - 4 // the -4 is because the pc is incremented by 4 before the instruction is fetched
			}
		case 0b111:
			// BGEU
			if inst.regRead(rs1) >= inst.regRead(rs2) {
				inst.pc = uint32(int32(inst.pc)+immInt) - 4 // the -4 is because the pc is incremented by 4 before the instruction is fetched
			}
		default:
			inst.newException("Unsupported B-Type instruction exception: op=%d func3=%d", opcode, func3)
		}
	} else {
		inst.newException("Unsupported B-Type instruction exception: op=%d func3=%d", opcode, func3)
	}
}

func (inst *EmulatorInstance) executeMemIType(instruction uint32) {
	_, rd, rs1, imm, func3 := assembler.DecodeITypeInstruction(instruction)

	immInt := int32(imm<<20) >> 20
	// since this is the mem I-type, the opcode should be the same for all, thus only func3 needs to be checked
	switch func3 {
	case 0b000:
		// LB
		inst.regWrite(rd, uint32(int8(inst.memReadByte(uint32(int32(inst.regRead(rs1))+immInt)))))
	case 0b001:
		// LH
		inst.regWrite(rd, uint32(int16(inst.memReadHalf(uint32(int32(inst.regRead(rs1))+immInt)))))
	case 0b010:
		// LW
		inst.regWrite(rd, inst.memReadWord(uint32(int32(inst.regRead(rs1))+immInt), false))
	case 0b100:
		// LBU
		inst.regWrite(rd, uint32(inst.memReadByte(uint32(int32(inst.regRead(rs1))+immInt))))
	case 0b101:
		// LHU
		inst.regWrite(rd, uint32(inst.memReadHalf(uint32(int32(inst.regRead(rs1))+immInt))))
	default:
		inst.newException("Unsupported Mem I-Type instruction exception: func3=%d", func3)
	}
}

func (inst *EmulatorInstance) executeIType(instruction uint32) {
	opcode, rd, rs1, imm, func3 := assembler.DecodeITypeInstruction(instruction)

	if opcode == 0b0010011 {
		switch func3 {
		case 0b000:
			// ADDI
			inst.regWrite(rd, uint32(int32(inst.regRead(rs1))+int32(imm<<20)>>20))
		case 0b010:
			// SLTI
			if int32(inst.regRead(rs1)) < (int32(imm<<20) >> 20) {
				inst.regWrite(rd, 1)
			} else {
				inst.regWrite(rd, 0)
			}
		case 0b011:
			// SLTIU
			if inst.regRead(rs1) < imm {
				inst.regWrite(rd, 1)
			} else {
				inst.regWrite(rd, 0)
			}
		case 0b100:
			// XORI
			inst.regWrite(rd, inst.regRead(rs1)^uint32((int32(imm<<20)>>20)))
		case 0b110:
			// ORI
			inst.regWrite(rd, inst.regRead(rs1)|uint32((int32(imm<<20)>>20)))
		case 0b111:
			// ANDI
			inst.regWrite(rd, inst.regRead(rs1)&uint32((int32(imm<<20)>>20)))
		case 0b001:
			// SLLI
			inst.regWrite(rd, inst.regRead(rs1)<<(imm&0b11111))
		case 0b101:
			// SRLI/SRAI
			if imm>>5 == 0b0000000 {
				// SRLI
				inst.regWrite(rd, inst.regRead(rs1)>>(imm&0b11111))
			} else if imm>>5 == 0b0100000 {
				// SRAI
				inst.regWrite(rd, uint32(int32(inst.regRead(rs1))>>(imm&0b11111)))
			} else {
				inst.newException("Unsupported I-Type instruction exception: op=%d func3=%d imm=%d", opcode, func3, imm)
			}
		}
	} else {
		inst.newException("Unsupported I-Type instruction exception: op=%d func3=%d", opcode, func3)
	}
}

func (inst *EmulatorInstance) executeRType(instruction uint32) {
	opcode, rd, rs1, rs2, func7, func3 := assembler.DecodeRTypeInstruction(instruction)
	if opcode == 0b0110011 {
		if func7 == 0b0000000 || func7 == 0b0100000 {
			switch func3 {
			case 0b000:
				// ADD/SUB
				if func7 == 0b0000000 {
					// ADD
					inst.regWrite(rd, uint32(int32(inst.regRead(rs1))+int32(inst.regRead(rs2))))
				} else if func7 == 0b0100000 {
					// SUB
					inst.regWrite(rd, uint32(int32(inst.regRead(rs1))-int32(inst.regRead(rs2))))
				}
			case 0b001:
				// SLL
				inst.regWrite(rd, inst.regRead(rs1)<<(inst.regRead(rs2)&0b11111))
			case 0b010:
				// SLT
				if int32(inst.regRead(rs1)) < int32(inst.regRead(rs2)) {
					inst.regWrite(rd, 1)
				} else {
					inst.regWrite(rd, 0)
				}
			case 0b011:
				// SLTU
				if inst.regRead(rs1) < inst.regRead(rs2) {
					inst.regWrite(rd, 1)
				} else {
					inst.regWrite(rd, 0)
				}
			case 0b100:
				// XOR
				inst.regWrite(rd, inst.regRead(rs1)^inst.regRead(rs2))
			case 0b101:
				// SRL/SRA
				if func7 == 0b0000000 {
					// SRL
					inst.regWrite(rd, inst.regRead(rs1)>>(inst.regRead(rs2)&0b11111))
				} else if func7 == 0b0100000 {
					// SRA
					inst.regWrite(rd, uint32(int32(inst.regRead(rs1))>>(inst.regRead(rs2)&0b11111)))
				}
			case 0b110:
				// OR
				inst.regWrite(rd, inst.regRead(rs1)|inst.regRead(rs2))
			case 0b111:
				// AND
				inst.regWrite(rd, inst.regRead(rs1)&inst.regRead(rs2))
			}
		} else if func7 == 0b0000001 {
			switch func3 {
			case 0b000:
				// MUL
				inst.regWrite(rd, inst.regRead(rs1)*inst.regRead(rs2))
			case 0b001:
				// MULH
				inst.regWrite(rd, uint32(int64(int64(int32(inst.regRead(rs1)))*int64(int32(inst.regRead(rs2)))>>32)))
			case 0b010:
				// MULHSU
				inst.regWrite(rd, uint32(int64(int64(int32(inst.regRead(rs1)))*int64(inst.regRead(rs2))>>32)))
			case 0b011:
				// MULHU
				inst.regWrite(rd, uint32(int64(int64(inst.regRead(rs1))*int64(inst.regRead(rs2))>>32)))
			case 0b100:
				// DIV
				// testing for divide by zero
				if inst.regRead(rs2) == 0 {
					inst.newException("divide by zero")
					return
				}

				inst.regWrite(rd, uint32(int32(inst.regRead(rs1))/int32(inst.regRead(rs2))))
			case 0b101:
				// DIVU
				// testing for divide by zero
				if inst.regRead(rs2) == 0 {
					inst.newException("divide by zero")
					return
				}

				inst.regWrite(rd, inst.regRead(rs1)/inst.regRead(rs2))
			case 0b110:
				// REM
				inst.regWrite(rd, uint32(int32(inst.regRead(rs1))%int32(inst.regRead(rs2))))
			case 0b111:
				// REMU
				inst.regWrite(rd, inst.regRead(rs1)%inst.regRead(rs2))
			}
		} else {
			inst.newException("Unsupported R-Type instruction exception: op=%d func3=%d func7=%d", opcode, func3, func7)
		}
	} else {
		inst.newException("Unsupported R-Type instruction exception: op=%d func3=%d func7=%d", opcode, func3, func7)
	}
}

func (inst *EmulatorInstance) executeSType(instruction uint32) {
	opcode, rs1, rs2, imm, func3 := assembler.DecodeSTypeInstruction(instruction)

	immInt := int32(imm<<20) >> 20

	if opcode == 0b0100011 {
		switch func3 {
		case 0b000:
			// SB
			inst.memWriteByte(uint32(int32(inst.regRead(rs1))+immInt), inst.regRead(rs2))
		case 0b001:
			// SH
			inst.memWriteHalf(uint32(int32(inst.regRead(rs1))+immInt), inst.regRead(rs2))
		case 0b010:
			// SW
			inst.memWriteWord(uint32(int32(inst.regRead(rs1))+immInt), inst.regRead(rs2))
		default:
			inst.newException("Unsupported S-Type instruction exception: op=%d func3=%d", opcode, func3)
		}
	} else {
		inst.newException("Unsupported S-Type instruction exception: op=%d func3=%d", opcode, func3)
	}
}

func (inst *EmulatorInstance) executeEnv(instruction uint32) {
	opcode, _, _, imm, func3 := assembler.DecodeITypeInstruction(instruction)
	switch opcode {
	case 0b1110011:
		switch func3 {
		case 0b000:
			// ECALL/EBREAK
			if imm == 0b000000000000 {
				if inst.osEntry == 0 {
					// no os entry point, so throw an exception
					inst.newException("No ECALL handler registered. Perhaps the assignment file wasn't specified, or the editor is in the wrong folder?")
					return
				}

				// ECALL - should set PC to the os entry point and store the return address in x1
				// great resource: https://marcin.juszkiewicz.com.pl/download/tables/syscalls.html
				if inst.registers[17] == 93 {
					// syscall 93 is a special case that is used to exit the emulator
					inst.exitCode = int(inst.registers[10])
					inst.pc = 0x20352031
					return
				} else if inst.registers[17] == 214 {
					// sbrk
					// increment amount is in x16
					// return value is in x10
					inst.registers[10] = inst.heapPointer
					inst.heapPointer = uint32(int32(inst.heapPointer) + int32(inst.registers[16]))
					return
				} else if inst.registers[17] == 64 {
					// write
					// file descriptor is in x12
					// buffer address is in x11
					// buffer length is in x10
					for i := uint32(0); i < inst.registers[10]; i++ {
						if inst.stdOutCallback != nil {
							b := byte(inst.memReadByte(inst.registers[11] + i))
							if b == 0 {
								break
							}
							inst.stdOutCallback(b)
						} else {
							break
						}
					}
					return
				}

				inst.userGlobalPointer = inst.registers[3]
				inst.isInOSCode = true

				// preserving registers
				for i := 1; i < 31; i++ {
					inst.registerPreservation[i] = inst.registers[i]
				}
				inst.wasEcall = true

				// pushing registers a0 - a7 to the stack
				for i := 0; i < 8; i++ {
					inst.memWriteWord(inst.registers[2], inst.registers[10+i])
					inst.registers[2] -= 4
				}
				inst.registers[10] = inst.registers[2] + 4                // in the event that the compiled code is expecting it in the argument register
				inst.memWriteWord(inst.registers[2], inst.registers[2]+4) // in c it is Registers* so this is the pointer, but it is really stack allocated
				inst.registers[2] -= 40

				// setting the frame pointer to the top of the stack
				inst.registers[8] = inst.registers[2]

				// setting the ra register to the return address
				inst.registers[1] = inst.pc + 4

				inst.registers[3] = inst.osGlobalPointer
				inst.pc = inst.osEntry
			} else if imm == 0b000000000001 {
				// EBREAK
				inst.newException("EBREAK instruction exception")
			}
		default:
			inst.newException("Unsupported Env-Type instruction exception: op=%d func3=%d", opcode, func3)
		}
	default:
		inst.newException("Unsupported Env-Type instruction exception: op=%d func3=%d", opcode, func3)
	}
}

func (inst *EmulatorInstance) reportException(exception RuntimeException) {
	inst.errors = append(inst.errors, exception)
	if inst.runtimeErrorCallback != nil && !inst.terminated {
		inst.runtimeErrorCallback(exception)
	}
}

func (inst *EmulatorInstance) Interrupt(interrupt *Interrupt) {
	if inst.interrupt != nil {
		return // still processing previous interrupt
	}
	inst.interrupt = interrupt
}
