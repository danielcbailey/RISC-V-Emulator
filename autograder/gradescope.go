package autograder

import (
	"encoding/json"
	"os"
)

type GradescopeTest struct {
	Name       string `json:"name"`
	MaxScore   int    `json:"max_score"`
	Score      int    `json:"score"`
	Output     string `json:"output"`
	Visibility string `json:"visibility"`
	Status     string `json:"status,omitempty"`
}

type GradescopeLeaderBoardEntry struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
	Order string `json:"order,omitempty"`
}

type GradescopeOutput struct {
	Tests           []GradescopeTest             `json:"tests"`
	LeaderboardData []GradescopeLeaderBoardEntry `json:"leaderboard"`
}

func CreateGradescopeOutput() *GradescopeOutput {
	return &GradescopeOutput{
		Tests:           []GradescopeTest{},
		LeaderboardData: []GradescopeLeaderBoardEntry{},
	}
}

func (gso *GradescopeOutput) AddTest(test GradescopeTest, score int) {
	test.Score = score
	gso.Tests = append(gso.Tests, test)
}

func (gso *GradescopeOutput) AddLeaderBoardEntry(entry GradescopeLeaderBoardEntry) {
	gso.LeaderboardData = append(gso.LeaderboardData, entry)
}

func (gso *GradescopeOutput) Save() {
	// Saves the json to a file called results/results.json

	b, e := json.Marshal(gso)
	if e != nil {
		panic(e)
	}

	e = os.WriteFile("results/results.json", b, 0644)
	if e != nil {
		panic(e)
	}
}

func CreateTestCase(name string, maxScore int, visibility string) GradescopeTest {
	return GradescopeTest{
		Name:       name,
		MaxScore:   maxScore,
		Visibility: visibility,
	}
}

func (gt *GradescopeTest) SetStatus(success bool) {
	if success {
		gt.Status = "passed"
	} else {
		gt.Status = "failed"
	}
}

func (gt *GradescopeTest) OutputPrintLn(str string) {
	gt.Output += str + "\n"
}
