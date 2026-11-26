# CS2 Major MCP

Inspect and simulate a 16-team CS2 Major Swiss stage locally. The CLI and the
stdio MCP server use the same Go engine.

## Run

Use the executable for your platform. To build it from source with Go 1.25 or
later:

```sh
go build -o build/cs2-major ./cmd/cs2-major
build/cs2-major status examples/singapore-2026-stage-1.json
build/cs2-major score examples/singapore-2026-stage-1.json examples/pickem-card.json
build/cs2-major simulate examples/singapore-2026-stage-1.json --card examples/pickem-card.json --trials 1000 --seed 42
```

For an MCP client that accepts a command and arguments, use the absolute path to
the executable:

```json
{
  "mcpServers": {
    "cs2-major": {
      "command": "/absolute/path/to/cs2-major",
      "args": ["mcp"]
    }
  }
}
```

The tools are `stage_status(stage)`, `score_pickem(stage, card)`, and
`simulate_pickem(stage, card?, trials?, seed?)`. Pass the stage and card as
JSON objects.

## Inputs and outputs

- List 16 unique teams from highest to lowest initial seed. Supply results in
  round order. Each result names the winner and loser of a finished series.
  You can supply some results for the last round and see its remaining matches.
- Set `all_bo3` for a stage that uses best-of-three throughout. Otherwise,
  advancement and elimination matches are best-of-three and other matches are
  best-of-one. A result outside the generated schedule stops the call and reports
  the conflicting round and pairings.
- A card has two `three_zero` picks, six `advance` picks for teams that finish
  3–1 or 3–2, and two `zero_three` picks. `upper_bound` counts correct and
  pending picks. Pending picks can depend on mutually exclusive outcomes.
- Simulation defaults to 1,000 trials and seed 0. It accepts 1–20,000 trials
  and a signed 64-bit seed. Each release produces the same result for the same
  input and seed. For clients that parse JSON numbers as JavaScript numbers,
  keep the seed within ±9,007,199,254,740,991.
- Supply a numeric rating for every team to use rating-based map odds. Omit
  ratings for equal map odds. A suggested card maximizes the expected number of
  correct picks. The probability of getting at least five applies to the card
  supplied to the simulator.

## Code and tests

- `cmd/cs2-major` contains argument handling, JSON file access and the MCP
  server. Its integration test calls all three tools through the built binary.
- `internal/major` contains Swiss pairings, Buchholz, scoring and simulation.
- `examples` contains JSON inputs. Engine replay fixtures are in
  `internal/major/testdata`.

Run `go test ./...` from the project directory.

The engine uses the [Valve Swiss rules pinned at 745a407](https://github.com/ValveSoftware/counter-strike_rules_and_regs/blob/745a40758ed7877483d5544605cda6c6e1a1cd65/major-supplemental-rulebook.md#swiss-bracket).
It checks [Singapore Stage 1 results through Round 3 and PGL's published Round 4 bracket](https://esports.gg/news/counter-strike-2/cs2-pgl-singapore-major-2026-stage-1-overview-results/).
The Austin fixture uses [BLAST's Stage 1 match results](https://blast.tv/cs/tournaments/austin-major-2025-finals/match) and the [May 5 Valve standings](https://github.com/ValveSoftware/counter-strike_regional_standings/blob/main/live/2025/standings_global_2025_05_05.md).
