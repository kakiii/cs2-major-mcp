package main

import (
	"context"
	"encoding/json"

	"cs2-major-mcp/internal/major"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type statusInput struct {
	Stage major.Stage `json:"stage"`
}

type scoreInput struct {
	Stage major.Stage `json:"stage"`
	Card  major.Card  `json:"card"`
}

type simulateInput struct {
	Stage  major.Stage `json:"stage"`
	Card   *major.Card `json:"card,omitempty"`
	Trials int64       `json:"trials,omitempty"`
	Seed   int64       `json:"seed,omitempty"`
}

func serveMCP(ctx context.Context) error {
	server := mcp.NewServer(&mcp.Implementation{Name: "CS2 Major", Version: "0.1.0"}, nil)
	if err := addTool(server, &mcp.Tool{
		Name: "stage_status", Description: "Show standings and the remaining matches in the current round.",
	}, statusInput{}, func(input statusInput) (major.Status, error) {
		return major.StageStatus(input.Stage)
	}); err != nil {
		return err
	}
	if err := addTool(server, &mcp.Tool{
		Name: "score_pickem", Description: "Score a Pick'em card against known results.",
	}, scoreInput{}, func(input scoreInput) (major.Score, error) {
		return major.ScoreCard(input.Stage, input.Card)
	}); err != nil {
		return err
	}
	if err := addTool(server, &mcp.Tool{
		Name: "simulate_pickem", Description: "Estimate stage outcomes and optional card-score probabilities.",
	}, simulateInput{Trials: major.DefaultTrials}, func(input simulateInput) (major.Simulation, error) {
		return major.Simulate(input.Stage, input.Trials, input.Seed, input.Card)
	}); err != nil {
		return err
	}
	return server.Run(ctx, &mcp.StdioTransport{})
}

func addTool[In, Out any](server *mcp.Server, tool *mcp.Tool, defaults In, run func(In) (Out, error)) error {
	inputSchema, err := jsonschema.For[In](nil)
	if err != nil {
		return err
	}
	resolved, err := inputSchema.Resolve(nil)
	if err != nil {
		return err
	}
	outputSchema, err := jsonschema.For[Out](nil)
	if err != nil {
		return err
	}
	tool.InputSchema, tool.OutputSchema = inputSchema, outputSchema
	server.AddTool(tool, func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		input := defaults
		var result mcp.CallToolResult
		if err := decodeJSON(request.Params.Arguments, &input, resolved); err != nil {
			result.SetError(err)
			return &result, nil
		}
		out, err := run(input)
		if err != nil {
			result.SetError(err)
			return &result, nil
		}
		data, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		result.Content = []mcp.Content{&mcp.TextContent{Text: string(data)}}
		result.StructuredContent = json.RawMessage(data)
		return &result, nil
	})
	return nil
}
