package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-evolution-transaction-coordinator/internal/planner"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "plan":
		runPlan(os.Args[2:])
	case "conformance":
		runConformance(os.Args[2:])
	default:
		usage()
	}
}

func runPlan(args []string) {
	flags := flag.NewFlagSet("plan", flag.ExitOnError)
	metaPath := flags.String("source", ".gooo/semantic-work-wave-planner.gooo", "authoritative semantic planner source")
	contractPath := flags.String("contract", "contracts/semantic-work-wave-planner-v1.json", "fixed denominator contract")
	inputPath := flags.String("input", "", "immutable planner input JSON")
	outputRoot := flags.String("out", "", "empty caller-owned output directory")
	sourceRoot := flags.String("source-root", ".", "repository root to inventory")
	_ = flags.Parse(args)
	if *inputPath == "" || *outputRoot == "" {
		fmt.Fprintln(os.Stderr, "--input and --out are required caller-owned paths")
		os.Exit(2)
	}
	evidence, err := planner.Run(planner.RunInput{MetaPath: *metaPath, ContractPath: *contractPath, InputPath: *inputPath, OutputRoot: *outputRoot, SourceRoot: *sourceRoot})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("state=%s waves=%d boundaries=%d blocked=%d denied=%d replay_equal=%t\n", evidence.Plan.State, len(evidence.Plan.Waves), len(evidence.Plan.SerializedBoundaries), len(evidence.Plan.BlockedCausalFrontier), len(evidence.Plan.DeniedOperations), evidence.ReplayEqual)
}

func runConformance(args []string) {
	flags := flag.NewFlagSet("conformance", flag.ExitOnError)
	metaPath := flags.String("source", ".gooo/semantic-work-wave-planner.gooo", "authoritative semantic planner source")
	contractPath := flags.String("contract", "contracts/semantic-work-wave-planner-v1.json", "fixed denominator contract")
	corpusRoot := flags.String("corpus", "fixtures/semantic-work-wave-planner", "fixed corpus directory")
	outputRoot := flags.String("out", "", "empty caller-owned output directory")
	sourceRoot := flags.String("source-root", ".", "repository root to inventory")
	_ = flags.Parse(args)
	if *outputRoot == "" {
		fmt.Fprintln(os.Stderr, "--out is required caller-owned path")
		os.Exit(2)
	}
	summary, err := planner.RunConformance(planner.ConformanceInput{MetaPath: *metaPath, ContractPath: *contractPath, CorpusRoot: *corpusRoot, OutputRoot: *outputRoot, SourceRoot: *sourceRoot})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("fixed_cases=%d closed=%d unknown=%d refuted=%d replay_equal=%t\n", summary.FixedCases, summary.Closed, summary.Unknown, summary.Refuted, summary.ReplayEqual)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: gooo-semantic-work-wave-planner plan --input PATH --out PATH [--source PATH --contract PATH --source-root PATH]")
	fmt.Fprintln(os.Stderr, "   or: gooo-semantic-work-wave-planner conformance --out PATH [--source PATH --contract PATH --corpus PATH --source-root PATH]")
	os.Exit(2)
}
