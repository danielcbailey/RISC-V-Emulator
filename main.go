package main

import (
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
	if autograder.GetConfig() != nil {
		conf := autograder.GetConfig()
		if conf.Mode == "c" {
			autograder.AutogradeCCode(conf.AssignmentCodeDir, conf.StudentCodePath, conf.TestCases)
		} else if conf.Mode == "asm" {
			// TODO
		} else {
			log.Fatalln("Invalid autograding mode:", conf.Mode)
		}
	} else if len(os.Args) >= 2 && os.Args[1] == "languageServer" {
		if len(os.Args) >= 3 && os.Args[2] == "debug" {
			util.LoggingEnabled = true
		}
		languageServer.ListenAndServe()
		return
	} else if len(os.Args) >= 2 && os.Args[1] == "debug" {
		// listen for emulation requests over the stdin/out pipe
		emulator.RunDebugServer()
	} else if len(os.Args) == 3 && os.Args[1] == "assemble" {
		filePath := os.Args[2]
		// assemble the file - just for debugging!
		b, e := os.ReadFile(filePath)
		if e != nil {
			log.Fatalf("Could not read file %s: %v", filePath, e)
		}
		_ = assembler.Assemble(string(b))
	} else if len(os.Args) >= 3 && os.Args[1] == "runELF" {
		filePath := os.Args[2]
		assemblyPath := ""
		if len(os.Args) >= 4 {
			assemblyPath = os.Args[3]
		}
		// run the elf file
		emulator.RunStandaloneWebserver(filePath, assemblyPath)
	} else if len(os.Args) == 1 {
		// run as language server but in tcp mode so it can be remotely debugged
		languageServer.ListenAndServeTCP()
	} else if len(os.Args) == 5 && os.Args[1] == "runBatch" {
		asmFilePath := os.Args[2]
		elfFilePath := os.Args[3]
		seeds := strings.Split(os.Args[4], ",")
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
