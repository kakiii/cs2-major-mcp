package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cs2-major-mcp/internal/major"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBuiltCLIAndMCP(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "cs2-major")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	const stageFile = "../../examples/singapore-2026-stage-1.json"
	const cardFile = "../../examples/pickem-card.json"
	var stage major.Stage
	var card major.Card
	if err := readJSON(stageFile, &stage); err != nil {
		t.Fatal(err)
	}
	if err := readJSON(cardFile, &card); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	command := exec.CommandContext(ctx, binary, "mcp")
	command.Stderr = os.Stderr
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	for _, tc := range []struct {
		args  []string
		tool  string
		input any
	}{
		{[]string{"status", stageFile}, "stage_status", statusInput{Stage: stage}},
		{[]string{"score", stageFile, cardFile}, "score_pickem", scoreInput{Stage: stage, Card: card}},
		{[]string{"simulate", "--trials", "0100", stageFile, "--seed", "042", "--card", cardFile},
			"simulate_pickem", simulateInput{Stage: stage, Card: &card, Trials: 100, Seed: 42}},
	} {
		cli, err := exec.CommandContext(ctx, binary, tc.args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", tc.tool, err, cli)
		}
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.tool, Arguments: tc.input})
		if err != nil || result.IsError {
			t.Fatalf("%s: result %+v, error %v", tc.tool, result, err)
		}
		var value any
		if err := json.Unmarshal(cli, &value); err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(result.StructuredContent)
		if err != nil || string(got) != string(want) {
			t.Fatalf("%s: got %s, want %s (%v)", tc.tool, got, want, err)
		}
	}

	bad := stage
	bad.Rounds = [][]major.Result{{{Winner: stage.Teams[0], Loser: stage.Teams[1]}}}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "stage_status", Arguments: statusInput{Stage: bad}})
	if err != nil || !result.IsError || len(result.Content) != 1 {
		t.Fatalf("expected a tool error for a conflicting bracket: %+v, %v", result, err)
	}
	if content, ok := result.Content[0].(*mcp.TextContent); !ok || !strings.Contains(content.Text, "round 1: got") {
		t.Fatalf("missing pairing diagnostic: %+v", result.Content)
	}

	const largeSeed = 1<<53 + 1
	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "simulate_pickem", Arguments: simulateInput{Stage: stage, Trials: 1, Seed: largeSeed},
	})
	if err != nil || result.IsError || len(result.Content) != 1 {
		t.Fatalf("large seed call: %+v, %v", result, err)
	}
	var simulation major.Simulation
	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected JSON text, got %+v", result.Content[0])
	}
	if err := json.Unmarshal([]byte(content.Text), &simulation); err != nil || simulation.Seed != largeSeed {
		t.Fatalf("seed lost precision: got %d, want %d (%v)", simulation.Seed, largeSeed, err)
	}
	for _, input := range []map[string]any{
		{"stage": stage, "seed": nil},
		{"stage": stage, "trials": nil},
		{"stage": stage, "trials": "100"},
	} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "simulate_pickem", Arguments: input})
		if err != nil || !result.IsError {
			t.Fatalf("expected an invalid argument error: %+v, %v", result, err)
		}
	}
}
