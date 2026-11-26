package major

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func load[T any](t *testing.T, path string) T {
	t.Helper()
	var value T
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func status(t *testing.T, stage Stage) Status {
	t.Helper()
	out, err := StageStatus(stage)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

const singapore = "../../examples/singapore-2026-stage-1.json"

func TestSingaporeRoundFour(t *testing.T) {
	stage := load[Stage](t, singapore)
	want := []Match{
		{"M80", "Eternal Fire", 3, [2]int{2, 1}},
		{"Imperial", "Liquid", 3, [2]int{2, 1}},
		{"Lynn Vision", "PVISION", 3, [2]int{2, 1}},
		{"HEROIC", "FaZe", 3, [2]int{1, 2}},
		{"100 Thieves", "9z", 3, [2]int{1, 2}},
		{"Wildcard", "Chinggis Warriors", 3, [2]int{1, 2}},
	}
	out := status(t, stage)
	if out.Round == nil || *out.Round != 4 || !slices.Equal(out.Matches, want) {
		t.Fatalf("round 4 pairings: got %+v, want %+v", out.Matches, want)
	}
	if out.Standings[0].Team != "Ninjas in Pyjamas" || out.Standings[1].Team != "FlyQuest" ||
		out.Standings[0].Buchholz != 3 || out.Standings[1].Buchholz != 3 {
		t.Fatalf("unexpected ordering in Buchholz tie: %+v", out.Standings[:2])
	}
	stage.Rounds = append(stage.Rounds, []Result{{"FaZe", "HEROIC"}})
	out = status(t, stage)
	want = slices.Delete(want, 3, 4)
	if out.Round == nil || *out.Round != 4 || !slices.Equal(out.Matches, want) {
		t.Fatalf("partial round changed the remaining pairings: %+v", out.Matches)
	}
	stage.Rounds[3] = append(stage.Rounds[3], Result{"FaZe", "HEROIC"})
	if _, err := StageStatus(stage); err == nil || !strings.Contains(err.Error(), "round 4: got \"FaZe\" vs \"HEROIC\"") || !strings.Contains(err.Error(), "\"M80\" vs \"Eternal Fire\"") {
		t.Fatalf("expected round, actual pair and expected pairs in error, got %v", err)
	}
}

func TestAustinRoundFiveAndFinal(t *testing.T) {
	stage := load[Stage](t, "testdata/austin-2025-stage-1.json")
	full := stage.Rounds
	stage.Rounds = full[:4]
	want := []Match{
		{"FlyQuest", "TYLOO", 3, [2]int{2, 2}},
		{"NRG", "Lynn Vision", 3, [2]int{2, 2}},
		{"Wildcard", "Legacy", 3, [2]int{2, 2}},
	}
	out := status(t, stage)
	if out.Round == nil || *out.Round != 5 || !slices.Equal(out.Matches, want) {
		t.Fatalf("round 5 pairings: got %+v, want %+v", out.Matches, want)
	}
	stage.Rounds = full
	out = status(t, stage)
	final := []Standing{
		{"B8", 3, 3, 0, -1}, {"HEROIC", 2, 3, 0, -4},
		{"BetBoom", 4, 3, 1, 1}, {"OG", 9, 3, 1, 1}, {"Nemiga", 12, 3, 1, 0},
		{"Lynn Vision", 6, 3, 2, -1}, {"Legacy", 14, 3, 2, -2}, {"TYLOO", 5, 3, 2, -5},
		{"FlyQuest", 8, 2, 3, 5}, {"NRG", 13, 2, 3, 5}, {"Wildcard", 7, 2, 3, 4},
		{"Imperial", 11, 1, 3, 3}, {"Chinggis Warriors", 10, 1, 3, 2}, {"Complexity", 1, 1, 3, -1},
		{"Metizport", 15, 0, 3, -2}, {"Fluxo", 16, 0, 3, -5},
	}
	if out.Round != nil || len(out.Matches) != 0 || !slices.Equal(out.Standings, final) {
		t.Fatalf("expected a completed stage, got %+v", out)
	}
}

func TestRoundThreeRematchAndOpeningFormat(t *testing.T) {
	teams := strings.Fields("A B C D E F G H I J K L M N O P")
	stage := Stage{Teams: teams, AllBO3: true}
	opened := status(t, stage)
	for i, m := range opened.Matches {
		if m != (Match{teams[i], teams[i+8], 3, [2]int{}}) {
			t.Fatalf("opening pair %d: %+v", i, m)
		}
		if opened.Standings[i].Team != teams[i] || opened.Standings[i].Buchholz != 0 {
			t.Fatalf("opening seed tie: %+v", opened.Standings[i])
		}
	}
	stage.AllBO3 = false
	stage.Rounds = [][]Result{
		{{"I", "A"}, {"B", "J"}, {"C", "K"}, {"D", "L"}, {"M", "E"}, {"N", "F"}, {"G", "O"}, {"P", "H"}},
		{{"B", "P"}, {"N", "C"}, {"M", "D"}, {"I", "G"}, {"O", "A"}, {"L", "E"}, {"K", "F"}, {"H", "J"}},
	}
	var got []pair
	for _, m := range status(t, stage).Matches {
		if m.Record == [2]int{1, 1} {
			got = append(got, pair{slices.Index(teams, m.TeamA), slices.Index(teams, m.TeamB)})
			if m.BestOf != 1 {
				t.Fatalf("expected a BO1 match, got %+v", m)
			}
		}
	}
	if want := []pair{{2, 14}, {3, 10}, {6, 7}, {15, 11}}; !slices.Equal(got, want) {
		t.Fatalf("1-1 group: got %v, want %v", got, want)
	}
}

func TestPickem(t *testing.T) {
	stage := load[Stage](t, singapore)
	card := load[Card](t, "../../examples/pickem-card.json")
	score, err := ScoreCard(stage, card)
	if err != nil || score.Correct != 3 || score.UpperBound != 9 ||
		score.Picks["PVISION"] != "missed" || score.Picks["FaZe"] != "pending" {
		t.Fatalf("unexpected score: %+v, %v", score, err)
	}
	a, err := Simulate(stage, 200, 42, &card)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Simulate(stage, 200, 42, &card)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("simulation changed between calls: %v", err)
	}
	if a.Teams["Ninjas in Pyjamas"]["three_zero"] != 1 ||
		a.Teams["DENDELE"]["zero_three"] != 1 ||
		math.Abs(a.Card.ProbabilityAtLeastFive-sumOdds(a.Card.ScoreProbability[5:])) > 1e-12 {
		t.Fatalf("simulation disagrees with confirmed results or card odds: %+v", a)
	}
	if _, err := ScoreCard(stage, a.SuggestedCard); err != nil {
		t.Fatal(err)
	}
	card.ZeroThree[0] = card.ThreeZero[0]
	if _, err := ScoreCard(stage, card); err == nil {
		t.Fatal("expected duplicate picks to fail")
	}
}

func sumOdds(odds []float64) float64 {
	total := 0.0
	for _, p := range odds {
		total += p
	}
	return total
}
