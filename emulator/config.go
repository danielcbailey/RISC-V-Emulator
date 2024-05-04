package emulator

import "time"

func (inst *EmulatorInstance) ResetRegisters(config EmulatorConfig) {
	for i := 0; i < 32; i++ {
		inst.registers[i] = 0
	}

	inst.registers[1] = 0x20352035 // 0x20352035 is the magic number for the RISC-V emulator to know when to end
	inst.registers[2] = config.StackStartAddress
	inst.registers[3] = config.GlobalDataAddress
	inst.registers[8] = config.StackStartAddress // frame pointer
	inst.callStack = []uint32{}
	inst.regInit = 0x10F
	inst.lastUsedRegisters = map[int]int{}
}

func NewEmulator(config EmulatorConfig) *EmulatorInstance {
	regs := [32]uint32{}
	regs[0] = 0
	regs[1] = 0x20352035 // 0x20352035 is the magic number for the RISC-V emulator to know when to end
	regs[2] = config.StackStartAddress
	regs[3] = config.GlobalDataAddress
	regs[8] = config.StackStartAddress // frame pointer

	config.Memory.WriteWord(config.StackStartAddress, 0x20352035) // in case the program tries to read from the stack

	randomSeed := config.RandomSeed
	if randomSeed == 0 {
		randomSeed = uint32(time.Now().Unix())
	}

	return &EmulatorInstance{
		registers:               regs,
		memory:                  config.Memory,
		pc:                      0,
		regInit:                 0x10F,
		iCache:                  nil,
		runtimeLimit:            config.RuntimeLimit,
		dCache:                  nil,
		profileIgnoreRangeStart: config.ProfileIgnoreRangeStart,
		profileIgnoreRangeEnd:   config.ProfileIgnoreRangeEnd,
		di:                      0,
		memUsage:                0,
		randomSeed:              randomSeed,
		heapPointer:             config.HeapStartAddress,
		errors:                  []RuntimeException{},
		breakpoints:             map[uint32]Breakpoint{},
		registerBreakpoints:     map[int]Breakpoint{},
		memoryBreakpoints:       map[uint32]Breakpoint{},
		osGlobalPointer:         config.OSGlobalPointer,
		userGlobalPointer:       config.GlobalDataAddress,
		breakAddr:               0xFFFFFFFF,
		interrupt:               nil,
		display:                 &VirtualDisplay{},
		breakNext:               false,
		stdOutCallback:          config.StdOutCallback,
		runtimeErrorCallback:    config.RuntimeErrorCallback,
		lastUsedRegisters:       map[int]int{},
	}
}

func NewMemoryImage() *MemoryImage {
	return &MemoryImage{Blocks: map[uint32]*MemoryPage{}}
}

func (m *MemoryImage) getOrCreatePage(addr uint32) *MemoryPage {
	page, ok := m.Blocks[addr>>12]
	if !ok {
		page = &MemoryPage{Block: [1024]uint32{}, StartAddr: addr & 0xFFFFF000}
		m.Blocks[addr>>12] = page
	}
	return page
}

func (m *MemoryImage) WriteWord(addr uint32, value uint32) {
	page := m.getOrCreatePage(addr)
	page.Block[(addr&0xFFF)>>2] = value
	page.Initialized[(addr&0xFFF)>>2] = true
}

func (m *MemoryImage) WriteByte(addr uint32, value byte) {
	page := m.getOrCreatePage(addr)
	page.Block[(addr&0xFFF)>>2] = (page.Block[(addr&0xFFF)>>2] & ^(0xFF << ((addr & 0x3) * 8))) | (uint32(value) << ((addr & 0x3) * 8))
	page.Initialized[(addr&0xFFF)>>2] = true
}

func (m *MemoryImage) ReadWord(addr uint32) (uint32, bool) {
	page, ok := m.Blocks[addr>>12]
	if !ok {
		return 0, false
	}
	return page.Block[(addr&0xFFF)>>2], page.Initialized[(addr&0xFFF)>>2]
}

func (m *MemoryImage) ReadByte(addr uint32) (byte, bool) {
	page, ok := m.Blocks[addr>>12]
	if !ok {
		return 0, false
	}
	return byte((page.Block[(addr&0xFFF)>>2] >> ((addr & 0x3) * 8)) & 0xFF), page.Initialized[(addr&0xFFF)>>2]
}

func (m *MemoryImage) ReadHalfWord(addr uint32) (uint16, bool) {
	page, ok := m.Blocks[addr>>12]
	if !ok {
		return 0, false
	}
	return uint16((page.Block[(addr&0xFFF)>>2] >> ((addr & 0x3) * 8)) & 0xFFFF), page.Initialized[(addr&0xFFF)>>2]
}

func (m *MemoryImage) Clone() *MemoryImage {
	newMem := NewMemoryImage()
	for k, v := range m.Blocks {
		newPage := &MemoryPage{Block: [1024]uint32{}, Initialized: [1024]bool{}, StartAddr: v.StartAddr}
		copy(newPage.Block[:], v.Block[:])
		copy(newPage.Initialized[:], v.Initialized[:])
		newMem.Blocks[k] = newPage
	}
	return newMem
}

func (inst *EmulatorInstance) GetExitCode() int {
	return inst.exitCode
}

func (inst *EmulatorInstance) GetMemoryUsage() uint32 {
	return inst.memUsage
}

func (inst *EmulatorInstance) GetDynamicInstructionCount() uint32 {
	return inst.di
}

func (inst *EmulatorInstance) GetDisplay() *VirtualDisplay {
	return inst.display
}

func (inst *EmulatorInstance) GetErrors() []RuntimeException {
	return inst.errors
}

func (inst *EmulatorInstance) GetTotalInstructionsExecuted() uint64 {
	return inst.executedInstructions
}

func (inst *EmulatorInstance) Terminate() {
	inst.terminated = true
}

func (inst *EmulatorInstance) AddBreakpoint(addr uint32, breakpoint Breakpoint) {
	inst.breakpoints[addr] = breakpoint
}

func (inst *EmulatorInstance) RemoveBreakpoint(addr uint32) {
	delete(inst.breakpoints, addr)
}

func (inst *EmulatorInstance) RemoveAllBreakpoints() {
	inst.breakpoints = map[uint32]Breakpoint{}
}

func (inst *EmulatorInstance) AddRegisterBreakpoint(reg int, breakpoint Breakpoint) {
	inst.registerBreakpoints[reg] = breakpoint
}

func (inst *EmulatorInstance) RemoveAllRegisterBreakpoints() {
	inst.registerBreakpoints = map[int]Breakpoint{}
}

func (inst *EmulatorInstance) AddMemoryBreakpoint(addr uint32, breakpoint Breakpoint) {
	inst.memoryBreakpoints[addr] = breakpoint
}

func (inst *EmulatorInstance) RemoveAllMemoryBreakpoints() {
	inst.memoryBreakpoints = map[uint32]Breakpoint{}
}
