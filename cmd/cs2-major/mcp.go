package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"

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
	Trials jsonInteger `json:"trials,omitempty"`
	Seed   jsonInteger `json:"seed,omitempty"`
}

type jsonInteger int64

func (n *jsonInteger) UnmarshalJSON(data []byte) error {
	value, err := strconv.ParseInt(string(data), 10, 64)
	*n = jsonInteger(value)
	return err
}

func serveMCP(ctx context.Context) error {
	server := mcp.NewServer(&mcp.Implementation{Name: "CS2 Major", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name: "stage_status", Description: "Show standings and the remaining matches in the current round.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input statusInput) (*mcp.CallToolResult, major.Status, error) {
		out, err := major.StageStatus(input.Stage)
		return nil, out, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "score_pickem", Description: "Score a Pick'em card against known results.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input scoreInput) (*mcp.CallToolResult, major.Score, error) {
		out, err := major.ScoreCard(input.Stage, input.Card)
		return nil, out, err
	})
	inputSchema, err := jsonschema.For[simulateInput](nil)
	if err != nil {
		return err
	}
	outputSchema, err := jsonschema.For[major.Simulation](nil)
	if err != nil {
		return err
	}
	server.AddTool(&mcp.Tool{
		Name: "simulate_pickem", Description: "Estimate stage outcomes and optional card-score probabilities.",
		InputSchema: inputSchema, OutputSchema: outputSchema,
	}, simulateTool)
	return server.Run(ctx, &mcp.StdioTransport{})
}

func simulateTool(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Decode the raw integer seed before any conversion to a floating-point value.
	input := simulateInput{Trials: major.DefaultTrials}
	var result mcp.CallToolResult
	if err := decodeJSON(bytes.NewReader(request.Params.Arguments), &input); err != nil {
		result.SetError(err)
		return &result, nil
	}
	out, err := major.Simulate(input.Stage, int64(input.Trials), int64(input.Seed), input.Card)
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
}
