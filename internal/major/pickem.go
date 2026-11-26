package major

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
)

const (
	DefaultTrials = 1000
	MaxTrials     = 20_000
)

type Card struct {
	ThreeZero []string `json:"three_zero"`
	Advance   []string `json:"advance"`
	ZeroThree []string `json:"zero_three"`
}

type Score struct {
	Picks      map[string]string `json:"picks"`
	Correct    int               `json:"correct"`
	UpperBound int               `json:"upper_bound"`
}

type Simulation struct {
	Trials        int64                         `json:"trials"`
	Seed          int64                         `json:"seed"`
	Teams         map[string]map[string]float64 `json:"teams"`
	SuggestedCard Card                          `json:"suggested_card"`
	Card          *CardOdds                     `json:"card,omitempty"`
}

type CardOdds struct {
	ScoreProbability       [11]float64 `json:"score_probability"`
	ProbabilityAtLeastFive float64     `json:"probability_at_least_five"`
}

const (
	threeZero = iota
	advance
	zeroThree
	eliminated
)

var categories = [...]string{"three_zero", "advance", "zero_three", "eliminated"}
var slots = [...]int{2, 6, 2}

func (s *swiss) cardPicks(c Card) (map[int]int, error) {
	picks := make(map[int]int, 10)
	for category, group := range [3][]string{c.ThreeZero, c.Advance, c.ZeroThree} {
		if len(group) != slots[category] {
			return nil, fmt.Errorf("%s requires %d picks", categories[category], slots[category])
		}
		for _, name := range group {
			i := slices.Index(s.teams[:], strings.TrimSpace(name))
			if _, duplicate := picks[i]; i < 0 || duplicate {
				return nil, fmt.Errorf("unknown or duplicate Pick'em pick: %q", name)
			}
			picks[i] = category
		}
	}
	return picks, nil
}

func (r record) category() int {
	switch {
	case r.wins == 3 && r.losses == 0:
		return threeZero
	case r.wins == 3:
		return advance
	case r.wins == 0:
		return zeroThree
	default:
		return eliminated
	}
}

func ScoreCard(stage Stage, card Card) (Score, error) {
	s, err := replay(stage)
	if err != nil {
		return Score{}, err
	}
	picks, err := s.cardPicks(card)
	if err != nil {
		return Score{}, err
	}
	out := Score{Picks: make(map[string]string, len(picks))}
	for i, pick := range picks {
		r := s.records[i]
		status := "pending"
		switch {
		case max(r.wins, r.losses) == 3 && r.category() == pick:
			status = "correct"
			out.Correct++
		case max(r.wins, r.losses) == 3, pick == threeZero && r.losses > 0, pick == zeroThree && r.wins > 0:
			status = "missed"
		}
		if status != "missed" {
			out.UpperBound++
		}
		out.Picks[s.teams[i]] = status
	}
	return out, nil
}

func Simulate(stage Stage, trials, seed int64, card *Card) (Simulation, error) {
	if trials < 1 || trials > MaxTrials {
		return Simulation{}, fmt.Errorf("trials must be between 1 and %d", MaxTrials)
	}
	start, err := replay(stage)
	if err != nil {
		return Simulation{}, err
	}
	var picks map[int]int
	if card != nil {
		picks, err = start.cardPicks(*card)
		if err != nil {
			return Simulation{}, err
		}
	}

	rng := rand.New(rand.NewPCG(uint64(seed), 0))
	var counts [teamCount][4]int
	var scores [11]int
	for range trials {
		s := start
		for len(s.pending) > 0 {
			for _, m := range s.pending {
				gap := s.ratings[m.a] - s.ratings[m.b]
				p := (1 + math.Tanh(math.Ln10*(gap/800))) / 2
				if m.bestOf == 3 {
					p = p * p * (3 - 2*p)
				}
				if rng.Float64() < p {
					s.recordWin(m.a, m.b)
				} else {
					s.recordWin(m.b, m.a)
				}
			}
			s.round++
			s.schedule()
		}
		correct := 0
		for i, r := range s.records {
			category := r.category()
			counts[i][category]++
			if pick, present := picks[i]; present && pick == category {
				correct++
			}
		}
		scores[correct]++
	}

	out := Simulation{Trials: trials, Seed: seed, Teams: make(map[string]map[string]float64, teamCount)}
	for i, team := range start.teams {
		odds := make(map[string]float64, len(categories))
		for category, name := range categories {
			odds[name] = float64(counts[i][category]) / float64(trials)
		}
		out.Teams[team] = odds
	}
	out.SuggestedCard = suggestCard(start.teams, counts)
	if card != nil {
		out.Card = &CardOdds{}
		atLeastFive := 0
		for correct, count := range scores {
			out.Card.ScoreProbability[correct] = float64(count) / float64(trials)
			if correct >= 5 {
				atLeastFive += count
			}
		}
		out.Card.ProbabilityAtLeastFive = float64(atLeastFive) / float64(trials)
	}
	return out, nil
}

func suggestCard(teams [teamCount]string, counts [teamCount][4]int) Card {
	type assignment struct {
		valid bool
		total int
		picks [3][6]int
	}
	// Each state keeps the best assignment for that number of filled slots.
	var states [3][7][3]assignment
	states[0][0][0].valid = true
	for team := range teams {
		next := states
		for a := range states {
			for b := range states[a] {
				for c, state := range states[a][b] {
					if !state.valid {
						continue
					}
					filled := [3]int{a, b, c}
					for category, limit := range slots {
						index := filled[category]
						if index == limit {
							continue
						}
						used := filled
						used[category]++
						dst := &next[used[0]][used[1]][used[2]]
						if points := state.total + counts[team][category]; !dst.valid || points > dst.total {
							*dst = state
							dst.total = points
							dst.picks[category][index] = team
						}
					}
				}
			}
		}
		states = next
	}
	var picks [3][]string
	for category, limit := range slots {
		for _, i := range states[2][6][2].picks[category][:limit] {
			picks[category] = append(picks[category], teams[i])
		}
	}
	return Card{picks[0], picks[1], picks[2]}
}
