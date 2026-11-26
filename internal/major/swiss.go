package major

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
)

const teamCount = 16

type Stage struct {
	Teams   []string            `json:"teams"`
	Rounds  [][]Result          `json:"rounds,omitempty"`
	AllBO3  bool                `json:"all_bo3,omitempty"`
	Ratings map[string]*float64 `json:"ratings,omitempty"`
}

type Result struct {
	Winner string `json:"winner"`
	Loser  string `json:"loser"`
}

type Standing struct {
	Team     string `json:"team"`
	Seed     int    `json:"seed"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
	Buchholz int    `json:"buchholz"`
}

type Match struct {
	TeamA  string `json:"team_a"`
	TeamB  string `json:"team_b"`
	BestOf int    `json:"best_of"`
	Record [2]int `json:"record"`
}

type Status struct {
	Standings []Standing `json:"standings"`
	Round     *int       `json:"round"`
	Matches   []Match    `json:"matches"`
}

type record struct{ wins, losses int }
type pair struct{ a, b int }
type match struct {
	pair
	bestOf int
	before record
}

type swiss struct {
	teams     [teamCount]string
	ratings   [teamCount]float64
	records   [teamCount]record
	opponents [teamCount][teamCount]uint8
	allBO3    bool
	round     int
	pending   []match
}

func StageStatus(stage Stage) (Status, error) {
	s, err := replay(stage)
	if err != nil {
		return Status{}, err
	}
	out := Status{Standings: s.standings(), Matches: make([]Match, 0, len(s.pending))}
	if len(s.pending) > 0 {
		out.Round = &s.round
	}
	for _, m := range s.pending {
		out.Matches = append(out.Matches, Match{
			s.teams[m.a], s.teams[m.b], m.bestOf, [2]int{m.before.wins, m.before.losses},
		})
	}
	return out, nil
}

func replay(stage Stage) (swiss, error) {
	s := swiss{allBO3: stage.AllBO3, round: 1}
	if len(stage.Teams) != teamCount || len(stage.Rounds) > 5 {
		return s, fmt.Errorf("provide exactly 16 teams and at most five rounds")
	}
	seeds := make(map[string]int, teamCount)
	for i, name := range stage.Teams {
		name = strings.TrimSpace(name)
		if _, exists := seeds[name]; exists || name == "" {
			return s, fmt.Errorf("team names must be nonempty and unique: %q", name)
		}
		seeds[name], s.teams[i] = i, name
	}
	rated := make(map[int]bool)
	for name, rating := range stage.Ratings {
		i, known := seeds[strings.TrimSpace(name)]
		if !known || rated[i] || rating == nil || math.IsNaN(*rating) || math.IsInf(*rating, 0) {
			return s, fmt.Errorf("invalid, duplicate or unknown rating for %q", name)
		}
		rated[i], s.ratings[i] = true, *rating
	}
	if len(rated) != 0 && len(rated) != teamCount {
		return s, fmt.Errorf("provide ratings for every team or omit ratings")
	}

	s.schedule()
	for i, results := range stage.Rounds {
		if len(results) == 0 {
			return s, fmt.Errorf("round %d has no results", s.round)
		}
		for _, result := range results {
			winner, loser := strings.TrimSpace(result.Winner), strings.TrimSpace(result.Loser)
			index := slices.IndexFunc(s.pending, func(m match) bool {
				a, b := s.teams[m.a], s.teams[m.b]
				return a == winner && b == loser || a == loser && b == winner
			})
			if index < 0 {
				return s, fmt.Errorf("round %d: got %q vs %q, expected one of the pending matches: %s",
					s.round, winner, loser, s.pendingNames())
			}
			s.recordWin(seeds[winner], seeds[loser])
			s.pending = slices.Delete(s.pending, index, index+1)
		}
		if len(s.pending) == 0 {
			s.round++
			s.schedule()
		} else if i < len(stage.Rounds)-1 {
			return s, fmt.Errorf("round %d is incomplete, expected results for: %s", s.round, s.pendingNames())
		}
	}
	return s, nil
}

func (s *swiss) pendingNames() string {
	names := make([]string, 0, len(s.pending))
	for _, m := range s.pending {
		names = append(names, fmt.Sprintf("%q vs %q", s.teams[m.a], s.teams[m.b]))
	}
	return strings.Join(names, ", ")
}

func (s *swiss) recordWin(winner, loser int) {
	s.records[winner].wins++
	s.records[loser].losses++
	s.opponents[winner][loser]++
	s.opponents[loser][winner]++
}

func (s *swiss) standings() []Standing {
	rows := make([]Standing, teamCount)
	for i, name := range s.teams {
		buchholz := 0
		for j, count := range s.opponents[i] {
			buchholz += int(count) * (s.records[j].wins - s.records[j].losses)
		}
		rows[i] = Standing{name, i + 1, s.records[i].wins, s.records[i].losses, buchholz}
	}
	slices.SortStableFunc(rows, func(a, b Standing) int {
		if a.Wins != b.Wins {
			return cmp.Compare(b.Wins, a.Wins)
		}
		if a.Losses != b.Losses {
			return cmp.Compare(a.Losses, b.Losses)
		}
		return cmp.Compare(b.Buchholz, a.Buchholz)
	})
	return rows
}

// Each number encodes two one-based positions in a six-team record group.
var priority = [][3]int{
	{16, 25, 34}, {16, 24, 35}, {15, 26, 34}, {15, 24, 36}, {14, 26, 35},
	{14, 25, 36}, {16, 23, 45}, {15, 23, 46}, {13, 26, 45}, {13, 25, 46},
	{14, 23, 56}, {13, 24, 56}, {12, 36, 45}, {12, 35, 46}, {12, 34, 56},
}

func (s *swiss) schedule() {
	s.pending = nil
	if s.round == 1 {
		for i := range teamCount / 2 {
			s.addMatch(pair{i, i + teamCount/2}, record{})
		}
		return
	}
	rows := s.standings()
	for len(rows) > 0 {
		r := record{rows[0].Wins, rows[0].Losses}
		n := 1
		for n < len(rows) && rows[n].Wins == r.wins && rows[n].Losses == r.losses {
			n++
		}
		if max(r.wins, r.losses) < 3 {
			teams := make([]int, n)
			for i, row := range rows[:n] {
				teams[i] = row.Seed - 1
			}
			for _, p := range s.pairGroup(teams) {
				s.addMatch(p, r)
			}
		}
		rows = rows[n:]
	}
}

func (s *swiss) addMatch(p pair, r record) {
	bestOf := 1
	if s.allBO3 || max(r.wins, r.losses) == 2 {
		bestOf = 3
	}
	s.pending = append(s.pending, match{p, bestOf, r})
}

func (s *swiss) pairGroup(teams []int) []pair {
	if s.round < 4 {
		pairs, _ := s.highLow(teams)
		return pairs
	}
	var best []pair
	fewest := len(teams)
	for _, row := range priority {
		var candidate [3]pair
		rematches := 0
		for i, positions := range row {
			a, b := teams[positions/10-1], teams[positions%10-1]
			candidate[i] = pair{a, b}
			if s.opponents[a][b] > 0 {
				rematches++
			}
		}
		if rematches < fewest {
			best, fewest = candidate[:], rematches
		}
		if rematches == 0 {
			break
		}
	}
	return best
}

func (s *swiss) highLow(teams []int) ([]pair, int) {
	if len(teams) == 0 {
		return nil, 0
	}
	var best []pair
	fewest := len(teams)
	for i := len(teams) - 1; i > 0; i-- {
		p := pair{teams[0], teams[i]}
		rest := append(slices.Clone(teams[1:i]), teams[i+1:]...)
		tail, rematches := s.highLow(rest)
		if s.opponents[p.a][p.b] > 0 {
			rematches++
		}
		if rematches < fewest {
			best, fewest = append([]pair{p}, tail...), rematches
		}
		if rematches == 0 {
			break
		}
	}
	return best, fewest
}
