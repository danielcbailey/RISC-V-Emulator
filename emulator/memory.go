package emulator

import(
	"fmt"
)

func (inst *EmulatorInstance) memReadByte(addr uint32) uint32 {
	// preparing bit mask
	bitmask := uint32(0xFF)
	bitmask <<= (addr & 0x3) << 3

	return inst.memReadRaw(addr, bitmask, false)
}

func (inst *EmulatorInstance) memReadHalf(addr uint32) uint32 {
	// preparing bit mask
	bitmask := uint32(0xFFFF)
	bitmask <<= (addr & 0x2) << 3

	// checking for alignment
	if addr&0x1 != 0 {
		inst.newMemoryAccessNotAlignedException(addr, "halfword")
		return 0
	}

	return inst.memReadRaw(addr, bitmask, false)
}

func (inst *EmulatorInstance) memReadWord(addr uint32, isInstruction bool) uint32 {
	// checking for alignment
	if addr&0x3 != 0 {
		inst.newMemoryAccessNotAlignedException(addr, "word")
		return 0
	}

	return inst.memReadRaw(addr, 0xFFFFFFFF, isInstruction)
}

func (inst *EmulatorInstance) memReadRaw(addr uint32, bitmask uint32, isInstruction bool) uint32 {
	// accessing the memory, but first check if it is in the cache
	// if not, then load it into the cache
	blockAddr := addr & 0xFFFFF000
	if blockAddr >= 0x80000000 {
		// reserved memory
		return (inst.memReadReserved(addr&0xFFFFFFFF) & bitmask) >> ((addr & 0x3) * 8)
	}
	if isInstruction && inst.iCache.StartAddr != blockAddr {
		newBlock, ok := inst.memory.Blocks[blockAddr>>12]
		if !ok {
			inst.newMemoryAccessedBeforeInitializedException(addr)
			return 0
		}
		inst.iCache = newBlock
	} else if !isInstruction && inst.dCache.StartAddr != blockAddr {
		newBlock, ok := inst.memory.Blocks[blockAddr>>12]
		if !ok {
			inst.newMemoryAccessedBeforeInitializedException(addr)
			return 0
		}
		inst.dCache = newBlock
	}

	// now that the cache is loaded, we can read from it
	offset := (addr & 0xFFF) >> 2
	value := uint32(0)
	if isInstruction {
		if !inst.iCache.Initialized[offset] {
			inst.newMemoryAccessedBeforeInitializedException(addr)
			return 0
		}
		value = inst.iCache.Block[offset]
	} else {
		if !inst.dCache.Initialized[offset] {
			inst.newMemoryAccessedBeforeInitializedException(addr)
			return 0
		}
		value = inst.dCache.Block[offset]
	}

	return (value & bitmask) >> ((addr & 0x3) * 8)
}

func (inst *EmulatorInstance) memWriteRaw(addr, bitmask, value uint32) {
	// accessing the memory, but first check if it is in the cache
	// if not, then load it into the cache
	blockAddr := addr & 0xFFFFF000
	if blockAddr >= 0x80000000 {
		// reserved memory
		inst.memWriteReserved(addr&0xFFFFFFFF, bitmask, value)
		return
	}

	if inst.dCache.StartAddr != blockAddr {
		newBlock, ok := inst.memory.Blocks[blockAddr>>12]
		if !ok {
			newBlock = &MemoryPage{
				StartAddr:   blockAddr,
				Initialized: [1024]bool{},
				Block:       [1024]uint32{},
			}
			inst.memory.Blocks[blockAddr>>12] = newBlock
		}
		inst.dCache = newBlock
	}

	// now that the cache is loaded, we can write to it
	offset := (addr & 0xFFF) >> 2
	inst.dCache.Block[offset] = (inst.dCache.Block[offset] & ^bitmask) | (value << ((addr & 0x3) * 8))
	if inst.pc < inst.profileIgnoreRangeStart || inst.pc >= inst.profileIgnoreRangeEnd {
		if !inst.dCache.Initialized[offset] {
			sendOutput(fmt.Sprintf("inc memUsage when first write to offset %d\n", offset), true)
			// new memory access
			inst.memUsage++
		}

		if bp, ok := inst.memoryBreakpoints[addr]; ok {
			inst.breakCallback(inst, bp.ID, "data breakpoint")
		}
	}

	inst.dCache.Initialized[offset] = true
}

func (inst *EmulatorInstance) memWriteByte(addr, value uint32) {
	// preparing bit mask
	bitmask := uint32(0xFF)
	bitmask <<= (addr & 0x3) << 3

	inst.memWriteRaw(addr, bitmask, value)
}

func (inst *EmulatorInstance) memWriteHalf(addr, value uint32) {
	// preparing bit mask
	bitmask := uint32(0xFFFF)
	bitmask <<= (addr & 0x2) << 3

	// checking for alignment
	if addr&0x1 != 0 {
		inst.newMemoryAccessNotAlignedException(addr, "halfword")
		return
	}

	inst.memWriteRaw(addr, bitmask, value)
}

func (inst *EmulatorInstance) memWriteWord(addr, value uint32) {
	// checking for alignment
	if addr&0x3 != 0 {
		inst.newMemoryAccessNotAlignedException(addr, "word")
		return
	}
	//sendOutput(fmt.Sprintf("writing to addr %d\n", addr), true)
	inst.memWriteRaw(addr, 0xFFFFFFFF, value)
}

func (inst *EmulatorInstance) memReadReserved(addr uint32) uint32 {
	/*
	 * Reserved Memory Map
	 * 0x80000000 - 0x80002FEB: Future Reserved
	 *
	 * 0x80002FEC - 0x80002FEF: Virtual Display Shape Draw Filled Rectangle Color (executes the draw on write)
	 * 0x80002FF0 - 0x80002FFF: Virtual Display Shape Draw Parameters
	 * 0x80003000 - 0x80003003: OS ECALL Handler Entry Point
	 * 0x80003004 - 0x80003007: StdOut Pipe WRITEONLY
	 * 0x80003008 - 0x8000300B: Virtual Display Width
	 * 0x8000300C - 0x8000300F: Virtual Display Height
	 * 0x80003010 - 0x80003013: OS Interrupt Handler Entry Point
	 * 0x80003014 - 0x80003017: Random number seed READONLY
	 * 0x80003018 - 0x8000301B: Solution Correctness WRITEONLY (only from OS code)
	 * 0x8000301C - 0x8000301F: Interrupt ID
	 * 0x80003020 - 0x8000FFFF: Interrupt context
	 *
	 * 0x80010000 - 0x807FFFFF: Virtual Display Pixel Data [RGBA][RGBA]...
	 * 0x80800000 - 0xFFFFFFFF: Virutal FAT Storage Filesystem READONLY (for now)
	 */

	addr &= 0x7FFFFFFF

	if addr < 0x3000 {
		// future reserved - create a new memory access exception
		inst.newSegmentationFaultException(addr)
		return 0
	} else if addr < 0x3020 {
		switch addr {
		case 0x3000:
			// OS ECALL Handler Entry Point
			return inst.osEntry
		case 0x3004:
			// StdOut Pipe WRITEONLY
			inst.newSegmentationFaultException(addr)
			return 0
		case 0x3008:
			// Virtual Display Width
			return uint32(inst.display.width)
		case 0x300C:
			// Virtual Display Height
			return uint32(inst.display.height)
		case 0x3010:
			// OS Interrupt Handler Entry Point
			return inst.osInterruptHandlerEntry
		case 0x3014:
			return inst.randomSeed
		case 0x3018:
			// Solution Correctness WRITEONLY (only from OS code)
			inst.newSegmentationFaultException(addr)
			return 0
		case 0x301C:
			// Interrupt ID
			if inst.interrupt == nil {
				return 0
			}
			return uint32(inst.interrupt.ID)

		}
	} else if addr < 0x10000 {
		// interrupt context data
		offset := addr - 0x3020
		if inst.interrupt == nil || ((offset >> 2) > uint32(len(inst.interrupt.Data))) {
			inst.newSegmentationFaultException(addr)
			return 0
		}

		return inst.interrupt.Data[offset>>2]
	} else if addr < 0x00800000 {
		// Virtual Display Pixel Data
		offset := addr - 0x10000
		return inst.display.data[offset>>2]
	} else if addr < 0x80000000 {
		offset := addr - 0x100000
		if inst.fs == nil || ((offset >> 2) > uint32(len(inst.fs.data))) {
			inst.newSegmentationFaultException(addr)
			return 0
		}
		return inst.fs.data[offset>>2]
	}

	return 0 // not possible
}

func (inst *EmulatorInstance) memWriteReserved(addr, bitmask, value uint32) {
	addr &= 0x7FFFFFFF

	// memory map is defined in memReadReserved
	if addr < 0x2FEC {
		// future reserved - create a new memory access exception
		inst.newSegmentationFaultException(addr)
		return
	} else if addr < 0x3020 {
		switch addr & 0xFFFFFFFC {
		case 0x2FEC:
			// Virtual Display Shape Draw Filled Rectangle Color (executes the draw on write)
			inst.display.drawFilledRectangle(value)
		case 0x2FF0:
			// Virtual Display Shape Draw Parameters 0
			inst.display.shapeDrawParams[0] = value
		case 0x2FF4:
			// Virtual Display Shape Draw Parameters 1
			inst.display.shapeDrawParams[1] = value
		case 0x2FF8:
			// Virtual Display Shape Draw Parameters 2
			inst.display.shapeDrawParams[2] = value
		case 0x2FFC:
			// Virtual Display Shape Draw Parameters 3
			inst.display.shapeDrawParams[3] = value
		case 0x3000:
			// OS ECALL Handler Entry Point
			inst.osEntry = value
		case 0x3004:
			// StdOut Pipe WRITEONLY
			if inst.stdOutCallback != nil {
				inst.stdOutCallback(byte(value))
			}
		case 0x3008:
			// Virtual Display Width
			inst.display.width = int(value)
		case 0x300C:
			// Virtual Display Height
			inst.display.height = int(value)
		case 0x3010:
			// OS Interrupt Handler Entry Point
			inst.osInterruptHandlerEntry = value
		case 0x3014:
			// Random number seed READONLY
			inst.newSegmentationFaultException(addr)
			return
		case 0x3018:
			// Solution Correctness WRITEONLY (only from OS code)
			if inst.pc >= inst.profileIgnoreRangeStart && inst.pc < inst.profileIgnoreRangeEnd {
				inst.solutionValidity = value
			} else {
				inst.newSegmentationFaultException(addr)
			}
		case 0x301C:
			// Interrupt ID
			inst.newSegmentationFaultException(addr)
			return
		}
	} else if addr < 0x10000 {
		// interrupt data READONLY - create a new memory access exception
		inst.newSegmentationFaultException(addr)
		return
	} else if addr < 0x00800000 {
		offset := addr - 0x10000
		updateRegion := inst.display.getUpdateOffset(offset >> 2)

		// Virtual Display Pixel Data
		inst.display.dataMutex.Lock()
		inst.display.displayWrites++
		inst.display.data[offset>>2] = (inst.display.data[offset>>2] & ^bitmask) | (value << ((addr & 0x3) * 8))
		inst.display.updateRegions[updateRegion] = true
		inst.display.dataMutex.Unlock()
	} else if addr < 0x80000000 {
		// Virtual FAT Storage Filesystem READONLY (for now)
		inst.newSegmentationFaultException(addr)
		return
	}
}
