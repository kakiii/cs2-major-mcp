package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	"cs2-major-mcp/internal/major"
	"github.com/google/jsonschema-go/jsonschema"
)

const usage = "usage: cs2-major {status|score|simulate|mcp} [arguments]"

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	command, args := args[0], args[1:]
	switch command {
	case "-h", "--help":
		fmt.Println(usage)
		return nil
	case "mcp":
		if len(args) != 0 {
			return errors.New("usage: cs2-major mcp")
		}
		return serveMCP(ctx)
	case "status", "score", "simulate":
	default:
		return fmt.Errorf("unknown command %q\n%s", command, usage)
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var cardPath string
	trials, seed := int64(major.DefaultTrials), int64(0)
	if command == "simulate" {
		flags.StringVar(&cardPath, "card", "", "Pick'em card JSON file")
		flags.Func("trials", "number of trials", decimalFlag(&trials))
		flags.Func("seed", "random seed", decimalFlag(&seed))
	}
	commandUsage := map[string]string{
		"status":   "status STAGE.json",
		"score":    "score STAGE.json CARD.json",
		"simulate": "simulate STAGE.json [--card CARD.json] [--trials N] [--seed N]",
	}
	flags.Usage = func() { fmt.Fprintln(os.Stderr, "usage: cs2-major "+commandUsage[command]) }

	var paths []string
	for len(args) > 0 {
		if err := flags.Parse(args); err != nil {
			return err
		}
		rest := flags.Args()
		if len(rest) == 0 {
			break
		}
		if slices.Contains(args[:len(args)-len(rest)], "--") {
			paths = append(paths, rest...)
			break
		}
		paths, args = append(paths, rest[0]), rest[1:]
	}
	wantPaths := 1
	if command == "score" {
		wantPaths = 2
	}
	if len(paths) != wantPaths {
		return fmt.Errorf("usage: cs2-major %s", commandUsage[command])
	}
	if command == "score" {
		cardPath = paths[1]
	}
	var stage major.Stage
	if err := readJSON(paths[0], &stage); err != nil {
		return err
	}
	var card *major.Card
	if command == "score" || cardPath != "" {
		card = new(major.Card)
		if err := readJSON(cardPath, card); err != nil {
			return err
		}
	}

	var output any
	var err error
	switch command {
	case "status":
		output, err = major.StageStatus(stage)
	case "score":
		output, err = major.ScoreCard(stage, *card)
	case "simulate":
		output, err = major.Simulate(stage, trials, seed, card)
	}
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func decimalFlag(dest *int64) func(string) error {
	return func(arg string) error {
		value, err := strconv.ParseInt(arg, 10, 64)
		*dest = value
		return err
	}
}

func readJSON[T any](path string, dest *T) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return err
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return err
	}
	if err := decodeJSON(data, dest, resolved); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func decodeJSON(data []byte, dest any, schema *jsonschema.Resolved) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if err := schema.Validate(value); err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}
