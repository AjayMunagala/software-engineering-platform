package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	frameworkengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/framework"
	ignoreengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	languageengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

func main() {
	repositoryPath := flag.String("path", ".", "absolute or relative path of the repository to inspect")
	pretty := flag.Bool("pretty", true, "format JSON output for people")
	flag.Parse()

	run := rie.NewRunContext(*repositoryPath, rie.DefaultConfig())
	pipeline := rie.New()
	if err := pipeline.Register(discovery.New()); err != nil {
		fmt.Fprintln(os.Stderr, "RIE engine registration error:", err)
		os.Exit(1)
	}
	if err := pipeline.Register(ignoreengine.New()); err != nil {
		fmt.Fprintln(os.Stderr, "RIE engine registration error:", err)
		os.Exit(1)
	}
	if err := pipeline.Register(languageengine.New()); err != nil {
		fmt.Fprintln(os.Stderr, "RIE engine registration error:", err)
		os.Exit(1)
	}
	if err := pipeline.Register(frameworkengine.New()); err != nil {
		fmt.Fprintln(os.Stderr, "RIE engine registration error:", err)
		os.Exit(1)
	}
	if err := pipeline.Run(context.Background(), run); err != nil {
		fmt.Fprintln(os.Stderr, "RIE v0.4 scan error:", err)
		os.Exit(1)
	}

	var (
		output []byte
		err    error
	)
	if *pretty {
		output, err = json.MarshalIndent(run.Report, "", "  ")
	} else {
		output, err = json.Marshal(run.Report)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "RIE v0.4 export error:", err)
		os.Exit(1)
	}

	fmt.Println(string(output))
}
