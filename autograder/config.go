package autograder

import (
	"encoding/json"
	"log"
	"os"
)

type TestCase struct {
	Number     int    `json:"number"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
	Points     int    `json:"points"`
}

type Config struct {
	AssignmentName    string     `json:"assignmentName"`
	AssignmentCodeDir string     `json:"assignmentCodeDir"`
	StudentCodePath   string     `json:"studentCodePath"`
	TestCases         []TestCase `json:"testCases"`
	CompilationPoints int        `json:"compilationPoints"`
	MemleakPoints     int        `json:"memleakPoints"`
	Mode              string     `json:"mode"` // either 'c' or 'asm'
}

var conf *Config

func GetConfig() *Config {
	if conf == nil {
		// attempt to load autograderConfig.json
		b, e := os.ReadFile("source/autograderConfig.json")
		if e != nil {
			return nil
		}

		// unmarshal the json into a Config struct
		conf = new(Config)
		e = json.Unmarshal(b, conf)
		if e != nil {
			log.Fatalln("Error unmarshalling autograderConfig.json:", e)
		}
	}

	return conf
}
