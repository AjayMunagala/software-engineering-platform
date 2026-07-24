package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/AjayMunagala/software-engineering-platform/experiments/phase-3.2-postgresql-spike/spike"
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: spike fixture|benchmark [options]")
	}
	switch os.Args[1] {
	case "fixture":
		runFixture(os.Args[2:])
	case "benchmark":
		runBenchmark(os.Args[2:])
	default:
		fatal("unknown command %q", os.Args[1])
	}
}

func runFixture(args []string) {
	flags := flag.NewFlagSet("fixture", flag.ExitOnError)
	root := flags.String("root", "", "repository root")
	output := flags.String("output", "", "fixture output directory")
	label := flags.String("label", "", "stable fixture label")
	commit := flags.String("commit", "", "pinned repository commit")
	fullRIE := flags.Bool("rie", false, "emit the RIE report artifact")
	semanticOnly := flags.Bool("semantic-only", false, "emit only the semantic artifact")
	workers := flags.Int("workers", 8, "maximum semantic workers")
	flags.Parse(args)
	manifest, err := spike.GenerateFixtures(context.Background(), spike.FixtureConfig{
		RepositoryRoot: *root, OutputDirectory: *output, Label: *label,
		Commit: *commit, IncludeRIEReport: *fullRIE, SemanticOnly: *semanticOnly,
		MaxWorkers: *workers,
	})
	if err != nil {
		fatal("fixture generation failed: %v", err)
	}
	printJSON(manifest)
}

func runBenchmark(args []string) {
	flags := flag.NewFlagSet("benchmark", flag.ExitOnError)
	connection := flags.String("connection", "host=/var/run/postgresql user=postgres dbname=platform_bench_phase32 sslmode=disable", "disposable PostgreSQL connection")
	fixtures := flags.String("fixtures", "", "fixture directory")
	output := flags.String("output", "", "benchmark report path")
	iterations := flags.Int("iterations", 10, "measured iterations per fixture/representation")
	hostStorage := flags.String("host-storage", "", "host storage identity")
	flags.Parse(args)
	report, err := spike.RunBenchmark(context.Background(), spike.Config{
		ConnectionString: *connection, FixtureDirectory: *fixtures,
		OutputPath: *output, Iterations: *iterations,
		HostStorage: strings.TrimSpace(*hostStorage),
	})
	if err != nil {
		fatal("benchmark failed: %v", err)
	}
	printJSON(report)
}

func printJSON(value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatal("encode output: %v", err)
	}
	fmt.Println(string(encoded))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
