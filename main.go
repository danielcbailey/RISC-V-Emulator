package main

import (
	"flag"
	"log"
	"os"
	"strconv"
	"strings"

	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/assembler"
	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/autograder"
	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/emulator"
	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/languageServer"
	"github.gatech.edu/ECEInnovation/RISC-V-Emulator/util"
)

func main() {
	specialRegisters := flag.String("specialregisters", "", "A comma-separated list of special registers to throw a warning if modified")

	flag.Parse()

	args := flag.Args()

	assembler.SetConfig(assembler.AssemblerConfig{
		SpecialRegisters: strings.Split((*specialRegisters), ","),
	})

	if autograder.GetConfig() != nil {
		conf := autograder.GetConfig()
		if conf.Mode == "c" {
			autograder.AutogradeCCode(conf.AssignmentCodeDir, conf.StudentCodePath, conf.TestCases)
		} else if conf.Mode == "asm" {
			// TODO
		} else {
			log.Fatalln("Invalid autograding mode:", conf.Mode)
		}
	} else if len(args) >= 1 && args[0] == "languageServer" {
		if len(args) >= 2 && args[1] == "debug" {
			util.LoggingEnabled = true
		}

		languageServer.ListenAndServe()
		return
	} else if len(args) >= 1 && args[0] == "debug" {
		// listen for emulation requests over the stdin/out pipe
		emulator.RunDebugServer()
	} else if len(args) == 3 && args[0] == "assemble" {
		filePath := args[1]
		// assemble the file - just for debugging!
		b, e := os.ReadFile(filePath)
		if e != nil {
			log.Fatalf("Could not read file %s: %v", filePath, e)
		}
		_ = assembler.Assemble(string(b))
	} else if len(args) >= 2 && args[0] == "runELF" {
		filePath := args[1]
		assemblyPath := ""
		if len(args) >= 3 {
			assemblyPath = os.Args[3]
		}
		// run the elf file
		emulator.RunStandaloneWebserver(filePath, assemblyPath)
	} else if len(args) == 0 {
		// run as language server but in tcp mode so it can be remotely debugged
		languageServer.ListenAndServeTCP()
	} else if len(args) == 4 && args[0] == "runBatch" {
		asmFilePath := args[1]
		elfFilePath := args[2]
		seeds := strings.Split(args[3], ",")
		seedInts := []uint32{}
		for _, s := range seeds {
			v, _ := strconv.ParseUint(s, 10, 32)
			seedInts = append(seedInts, uint32(v))
		}

		emulator.BatchRun(elfFilePath, asmFilePath, seedInts, true)
	} else {
		log.Fatalln("Invalid arguments:", os.Args)
	}
}
