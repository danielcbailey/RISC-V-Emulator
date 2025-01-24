package emulator

import (
	"debug/elf"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/assembler"
)

// Evaluation is for measuring accuracy of a single run or multiple runs. To be used in tandem with the
// extension or an autograder

type EvaluationRunResult struct {
	Seed      uint32 `json:"seed"`
	Passed    bool   `json:"passed"`
	DI        int    `json:"di"`   // stats
	SI        int    `json:"si"`   // stats
	Regs      int    `json:"regs"` // stats
	Mem       int    `json:"mem"`  // stats
	NumErrors int    `json:"numErrors"`
}

type streamingMessage struct {
	Type string      `json:"type"`
	Body interface{} `json:"body"`
}

type memoryImageContext struct {
	image                     *MemoryImage
	osCodeStart               uint32
	osCodeEnd                 uint32
	assemblyEntry             uint32
	asmGlobalPointer          uint32
	osGlobalPointer           uint32
	osEntry                   uint32
	asmStaticInstructionCount int
	asmStaticMemoryCount      int
}

func BatchRun(elfFilePath, asmFilePath string, seeds []uint32, streamToStdout bool) ([]EvaluationRunResult, error) {
	memImg, e := buildMemoryImage(elfFilePath, asmFilePath)
	if e != nil {
		if streamToStdout {
			msg := streamingMessage{
				Type: "error",
				Body: e.Error(),
			}
			mb, _ := json.Marshal(msg)
			fmt.Println(string(mb))
		}
		return nil, e
	}

	// detecting number of processors
	numCPUs := runtime.NumCPU()

	// only using half the available, if more than 1
	if numCPUs > 1 {
		numCPUs /= 2
	}

	stdOutMutex := sync.Mutex{}

	inputQueue := make(chan uint32, len(seeds))
	results := make(chan EvaluationRunResult, len(seeds))

	// start workers
	for i := 0; i < numCPUs; i++ {
		go evalWorker(memImg, inputQueue, &stdOutMutex, streamToStdout, results)
	}

	// queue up seeds
	for _, seed := range seeds {
		inputQueue <- seed
	}

	// close the input queue
	close(inputQueue)

	// collect results
	finalResults := make([]EvaluationRunResult, 0, len(seeds))
	for i := 0; i < len(seeds); i++ {
		result := <-results
		finalResults = append(finalResults, result)
	}

	return finalResults, nil
}

func evalWorker(memImg memoryImageContext, seedQueue chan uint32, stdOutMutex *sync.Mutex, streamToStdout bool, results chan EvaluationRunResult) {
	for seed := range seedQueue {
		// configure emulator
		numErrors := 0
		config := EmulatorConfig{
			StackStartAddress:       0x7FFFFFF0,
			GlobalDataAddress:       memImg.asmGlobalPointer,
			OSGlobalPointer:         memImg.osGlobalPointer,
			HeapStartAddress:        0x10000000,
			Memory:                  memImg.image.Clone(),
			ProfileIgnoreRangeStart: memImg.osCodeStart,
			ProfileIgnoreRangeEnd:   memImg.osCodeEnd,
			RandomSeed:              seed,
			RuntimeErrorCallback: func(e RuntimeException) {
				numErrors++
			},
			StdOutCallback: func(b byte) {
				// nothing..
			},
			RuntimeLimit: 100000, // 100,000 instructions, which doesn't include the CPP code
		}

		emulator := NewEmulator(config)

		emulator.Emulate(memImg.osEntry) // running assignment setup
		config.GlobalDataAddress = memImg.asmGlobalPointer
		emulator.ResetRegisters(config)
		emulator.Emulate(memImg.assemblyEntry) // running assembly

		// check if the emulator passed
		passed := emulator.solutionValidity == 2

		// emit results
		result := EvaluationRunResult{
			Seed:      seed,
			Passed:    passed,
			DI:        int(emulator.di),
			SI:        memImg.asmStaticInstructionCount,
			Regs:      int(emulator.regUsage),
			Mem:       int(emulator.memUsage) + memImg.asmStaticMemoryCount,
			NumErrors: numErrors,
		}

		if streamToStdout {
			stdOutMutex.Lock()
			msg := streamingMessage{
				Type: "result",
				Body: result,
			}
			mb, _ := json.Marshal(msg)
			fmt.Println(string(mb))
			stdOutMutex.Unlock()
		}

		results <- result
	}
}

func buildMemoryImage(elfFilePath, asmFilePath string) (memoryImageContext, error) {
	// loading the elf file
	f, e := elf.Open(elfFilePath)
	if e != nil {
		err := fmt.Errorf("error opening file: %v", e)
		return memoryImageContext{}, err
	}

	memoryImage := NewMemoryImage()
	cEnd := uint32(0)
	startAddr := uint32(0)
	globalPointer := uint32(0)
	osCodeStart := uint32(0)
	osCodeEnd := uint32(0)

	sections := f.Sections
	for _, section := range sections {
		if section.Addr == 0 || section.Size == 0 {
			continue // if it doesn't have an address, it's not a section we care about
		}

		if section.Type == elf.SHT_NOBITS {
			// this is a bss section, so we need to allocate memory for it
			for i := uint64(0); i < section.Size; i++ {
				memoryImage.WriteByte(uint32(section.Addr)+uint32(i), 0)
				cEnd = uint32(section.Addr) + uint32(i)
			}
			continue
		}

		// read the section data and write it to memory
		b, e := section.Data()
		if e != nil {
			err := fmt.Errorf("error reading section data of elf file: %v", e)
			return memoryImageContext{}, err
		}
		for i, v := range b {
			memoryImage.WriteByte(uint32(section.Addr)+uint32(i), v)
			cEnd = uint32(section.Addr) + uint32(i)
		}
	}

	symbols, e := f.Symbols()
	if e != nil {
		err := fmt.Errorf("error reading symbols of elf file: %v", e)
		return memoryImageContext{}, err
	}
	for _, symbol := range symbols {
		if symbol.Name == "_start" {
			startAddr = uint32(symbol.Value)
		} else if symbol.Name == "__global_pointer$" {
			globalPointer = uint32(symbol.Value)
		}
	}

	osCodeStart = uint32(f.Section(".text").Addr)
	osCodeEnd = uint32(f.Section(".text").Addr) + uint32(f.Section(".text").Size)

	if cEnd != 0 {
		cEnd += 1
		cEnd = (cEnd + 3) & ^uint32(3) // align to 4 bytes
	}

	// assembling the file
	b, e := os.ReadFile(asmFilePath)
	if e != nil {
		err := fmt.Errorf("error reading file: %v", e)
		return memoryImageContext{}, err
	}

	assemblyGlobalPointer := uint32(0)

	assembleRes := assembler.Assemble(string(b))
	numErrors := 0
	for _, diag := range assembleRes.Diagnostics {
		if diag.Severity == assembler.Error {
			numErrors++
		}
	}

	if numErrors > 0 {
		builder := strings.Builder{}
		builder.WriteByte('\n')
		for _, diag := range assembleRes.Diagnostics {
			builder.WriteString(fmt.Sprintf("\t%s:%d:%d: %s\n", filepath.Base(asmFilePath), diag.Range.Start.Line+1, diag.Range.Start.Char, diag.Message))
		}

		err := fmt.Errorf("errors assembling file: %s\n%s", asmFilePath, builder.String())
		return memoryImageContext{}, err
	}

	// load assembled code into memory
	for i, v := range assembleRes.ProgramText {
		memoryImage.WriteWord(cEnd+uint32(i)*4, v)
	}
	assemblyEntryPoint := cEnd
	assemblyGlobalPointer = cEnd + uint32(len(assembleRes.ProgramText)*4)
	for i, v := range assembleRes.ProgramData {
		memoryImage.WriteWord(assemblyGlobalPointer+uint32(i*4), v)
	}

	return memoryImageContext{
		image:                     memoryImage,
		osCodeStart:               osCodeStart,
		osCodeEnd:                 osCodeEnd,
		assemblyEntry:             assemblyEntryPoint,
		osEntry:                   startAddr,
		osGlobalPointer:           globalPointer,
		asmGlobalPointer:          assemblyGlobalPointer,
		asmStaticInstructionCount: len(assembleRes.ProgramText),
		asmStaticMemoryCount:      len(assembleRes.ProgramData),
	}, nil
}
