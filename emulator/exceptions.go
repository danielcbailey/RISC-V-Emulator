package emulator

import "fmt"

func (inst *EmulatorInstance) newException(format string, args ...interface{}) RuntimeException {
	// auto-reports

	// deep-copy registers
	regs := [32]uint32{}
	for i := 0; i < 32; i++ {
		regs[i] = inst.registers[i]
	}

	// deep-copy call stack
	callStack := make([]uint32, len(inst.callStack))
	copy(callStack, inst.callStack)
	callStack = append(callStack, inst.pc)

	exception := RuntimeException{
		regs:      regs,
		pc:        inst.pc,
		callStack: callStack,
		message:   fmt.Sprintf(format, args...),
	}

	inst.reportException(exception)
	return exception
}

func (inst *EmulatorInstance) newMemoryAccessedBeforeInitializedException(addr uint32) RuntimeException {
	return inst.newException("Memory accessed before initialized at 0x%08X", addr)
}

func (inst *EmulatorInstance) newMemoryAccessNotAlignedException(addr uint32, accessType string) RuntimeException {
	return inst.newException("Memory access not aligned at 0x%08X for type %s", addr, accessType)
}

func (inst *EmulatorInstance) newRegisterAccessedBeforeInitializedException(register uint32) RuntimeException {
	return inst.newException("Register accessed before initialized: x%d", register)
}

func (inst *EmulatorInstance) newSegmentationFaultException(addr uint32) RuntimeException {
	addr += 0x80000000 // because of some other line of code but this can be changed later. All references are in the reserved memory section, so look there
	return inst.newException("Segmentation fault accessing 0x%08X", addr)
}

func (inst *EmulatorInstance) newIllegalRegisterWrite() RuntimeException {
	return inst.newException("Illegal register write to read-only register x0")
}
