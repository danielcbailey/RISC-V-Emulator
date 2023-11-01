package autograder

import (
	"bytes"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func AutogradeCCode(assignmentCodeDir, studentCodePath string, testCases []TestCase) {
	// Call make -s c_autograder with assignmentCodeDir as working directory
	cmd := exec.Command("make", "-s", "c_autograder")
	cmd.Dir = assignmentCodeDir
	out, e := cmd.Output()
	if e != nil {
		log.Fatalln("Error running make -s c_autograder:", e)
	}

	// looking for .c file in submission dir
	dirFiles, e := os.ReadDir(filepath.Base(studentCodePath))
	if e != nil {
		log.Fatalln("Failed to list submission directory:", e.Error())
	}

	for _, f := range dirFiles {
		if filepath.Ext(f.Name()) == ".c" {
			studentCodePath = filepath.Join(studentCodePath, f.Name())
			break
		}
	}

	sources := strings.Split(strings.ReplaceAll(string(out), "\n", ""), " ")

	for i, source := range sources {
		if strings.Contains(source, "syscall.cpp") {
			sources = append(sources[:i], sources[i+1:]...)
			break
		}
	}

	// randomly generate an 8 digit number
	key := 0
	for i := 0; i < 8; i++ {
		key = key*10 + rand.Intn(10)
	}

	gso := CreateGradescopeOutput()

	// Call g++ with the following arguments:
	// -g -DAVOID_DISPLAY -DC_AUTOGRADER -D'C_AUTOGRADER_KEY={key}'
	// {sources}
	// -o StudentAssignmentBinary
	gppArgs := []string{"-c", "-g", "-DAVOID_DISPLAY", "-DC_AUTOGRADER", "-DC_AUTOGRADER_KEY=" + strconv.Itoa(key)}
	gppArgs = append(gppArgs, sources...)
	gppArgs = append(gppArgs, "-lstdc++")
	cmd = exec.Command("g++", gppArgs...)
	cmd.Dir = assignmentCodeDir
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	e = cmd.Run()
	if e != nil {
		log.Println("g++ " + strings.Join(gppArgs, " "))
		log.Print(string(errb.Bytes()))
		log.Print(string(outb.Bytes()))
		log.Fatalln("Error running g++:", e)
	}

	sourceObjs := make([]string, len(sources))
	for i, s := range sources {
		s = filepath.Base(s)
		sourceObjs[i] = s[:strings.LastIndex(s, ".")] + ".o"
	}

	// The student's code file will get modified to prevent an attack which could leak the assignment source code
	studentAbsFilePath := filepath.Join(assignmentCodeDir, studentCodePath)
	b, e := os.ReadFile(studentAbsFilePath)
	if e != nil {
		log.Fatalln("failed to open student code file:", e)
	}

	newCodeContents := strings.ReplaceAll(string(b), "#include <stdio.h>", "#include <stdio.h>\n#define fopen(a,b) NULL;_Static_assert(0, \"File I/O is not allowed\")\n#define open(a,b,c) NULL;_Static_assert(0, \"File I/O is not allowed\")")
	e = os.WriteFile(studentAbsFilePath, []byte(newCodeContents), 0644)
	if e != nil {
		log.Fatalln("failed to write modifications to student code file:", e)
	}

	compilationTestCase := CreateTestCase("Compilation", GetConfig().CompilationPoints, "visible")
	// Creating student code object file
	// call gcc with the following arguments:
	// -g -Wall -DAVOID_DISPLAY -DC_AUTOGRADER -D'C_AUTOGRADER_KEY={key}'
	// {student code}
	// -o {student code object file}
	gccArgs := []string{"-g", "-Wall", "-DAVOID_DISPLAY", "-DC_AUTOGRADER", "-o", "StudentProgram", studentCodePath}
	gccArgs = append(gccArgs, sourceObjs...)
	gccArgs = append(gccArgs, "-lstdc++")
	cmd = exec.Command("gcc", gccArgs...)
	cmd.Dir = assignmentCodeDir
	outb = bytes.Buffer{}
	errb = bytes.Buffer{}
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	e = cmd.Run()
	if e != nil {
		compilationTestCase.OutputPrintLn("gcc " + strings.Join(gccArgs, " "))
		compilationTestCase.OutputPrintLn(string(errb.Bytes()))
		compilationTestCase.OutputPrintLn(string(outb.Bytes()))
		compilationTestCase.OutputPrintLn("Failed to compile code with gcc: " + e.Error())
		compilationTestCase.SetStatus(false)
		gso.AddTest(compilationTestCase, 0)
		gso.Save()
		return
	}

	gccOutput := string(outb.Bytes()) + string(errb.Bytes())
	if strings.Contains(gccOutput, ": warning:") {
		compilationTestCase.OutputPrintLn("gcc " + strings.Join(gccArgs, " "))
		compilationTestCase.OutputPrintLn(gccOutput)
		compilationTestCase.OutputPrintLn("Compiled with warnings. Please see above for more info.")
		compilationTestCase.SetStatus(false)
		gso.AddTest(compilationTestCase, GetConfig().CompilationPoints/2)
	} else {
		compilationTestCase.OutputPrintLn("gcc " + strings.Join(gccArgs, " "))
		compilationTestCase.OutputPrintLn(gccOutput)
		compilationTestCase.OutputPrintLn("Successfully compiled code with no warnings with gcc.")
		gso.AddTest(compilationTestCase, GetConfig().CompilationPoints)
	}

	type TCTypePair struct {
		correct      int
		total        int
		earnedPoints int
		totalPoints  int
		output       string
	}

	tcRes := make(map[string]TCTypePair) // key is the visbility of the test case

	errorsCase := CreateTestCase("Errors", 0, "visible")
	errorsCase.SetStatus(true) // will be set to false if there are any errors
	for _, testCase := range testCases {
		correct, progOut := cAutogradeTestCase(assignmentCodeDir, strconv.Itoa(testCase.Number), key, &errorsCase)

		earnedPoints := 0
		if correct != 0 {
			earnedPoints = testCase.Points
		}

		passFail := "\n[FAIL] "
		if correct != 0 {
			passFail = "[PASS] "
		}

		outputStr := passFail + "Test Case: " + testCase.Name + " (" + strconv.Itoa(testCase.Number) + ")\n"
		outputStr += progOut

		if _, ok := tcRes[testCase.Visibility]; !ok {
			tcRes[testCase.Visibility] = TCTypePair{
				correct:      correct,
				total:        1,
				earnedPoints: earnedPoints,
				totalPoints:  testCase.Points,
				output:       outputStr,
			}
		} else {
			tcRes[testCase.Visibility] = TCTypePair{
				correct:      tcRes[testCase.Visibility].correct + correct,
				total:        tcRes[testCase.Visibility].total + 1,
				earnedPoints: tcRes[testCase.Visibility].earnedPoints + earnedPoints,
				totalPoints:  tcRes[testCase.Visibility].totalPoints + testCase.Points,
				output:       tcRes[testCase.Visibility].output + outputStr,
			}
		}
	}
	gso.AddTest(errorsCase, 0)

	// collating the results
	for visibility, res := range tcRes {
		// creating the test case
		tcTypeStr := "Smoke Test Cases"
		if visibility != "visible" {
			tcTypeStr = "All Other Test Cases"
		}
		tc := CreateTestCase(tcTypeStr, res.totalPoints, visibility)
		tc.OutputPrintLn("Number Passed: " + strconv.Itoa(res.correct) + "/" + strconv.Itoa(res.total))
		tc.OutputPrintLn(res.output)
		gso.AddTest(tc, res.earnedPoints)
	}

	gso.Save()
}

func cAutogradeTestCase(workingDir, testCaseNumber string, key int, tc *GradescopeTest) (int, string) {
	cmd := exec.Command("./StudentProgram", testCaseNumber)
	cmd.Dir = workingDir
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	e := cmd.Run()
	output := string(outb.Bytes())
	output = output + string(errb.Bytes())
	if e != nil {
		tc.OutputPrintLn(output)
		tc.OutputPrintLn("Error running student assignment program: " + e.Error())
		tc.SetStatus(false)
	}

	outLines := strings.Split(output, "\n")
	// Looking for the line "{key} student correctness: %d"
	lookingPrefix := strconv.Itoa(key) + " student correctness: "
	for i, line := range outLines {
		if strings.HasPrefix(line, lookingPrefix) {
			// Extracting the correctness percentage
			correctness, e := strconv.Atoi(line[len(lookingPrefix):])
			if e != nil {
				log.Fatalln("Error converting correctness percentage to int:", e)
			}
			outLines = append(outLines[:i], outLines[i+1:]...)
			return correctness, strings.Join(outLines, "\n")
		}
	}

	tc.OutputPrintLn("Error: Program never reported answer.")
	tc.SetStatus(false)
	return 0, output
}
