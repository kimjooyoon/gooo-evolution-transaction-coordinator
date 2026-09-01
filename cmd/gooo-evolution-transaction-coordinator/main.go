package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-evolution-transaction-coordinator/internal/coordinator"
)

type stringList []string

func (items *stringList) String() string {
	return fmt.Sprint([]string(*items))
}

func (items *stringList) Set(value string) error {
	*items = append(*items, value)
	return nil
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "generate" {
		fmt.Fprintln(os.Stderr, "usage: gooo-evolution-transaction-coordinator generate --source PATH --contract PATH --candidates-root PATH --fixture-root PATH --out PATH --source-root PATH [--candidate PATH ...]")
		os.Exit(2)
	}
	flags := flag.NewFlagSet("generate", flag.ExitOnError)
	metaPath := flags.String("source", ".gooo/evolution-transaction-coordinator.gooo", "authoritative Gooo meta source")
	contractPath := flags.String("contract", "contracts/denominator-v1.json", "fixed denominator contract")
	candidatesRoot := flags.String("candidates-root", "examples/candidates", "typed candidate directory")
	fixtureRoot := flags.String("fixture-root", "", "empty caller-owned compiler fixture directory")
	outputRoot := flags.String("out", "", "empty caller-owned output directory")
	sourceRoot := flags.String("source-root", ".", "input repository root to inventory")
	var candidatePaths stringList
	flags.Var(&candidatePaths, "candidate", "typed candidate path; repeat to supply an explicit candidate set")
	if err := flags.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if *fixtureRoot == "" || *outputRoot == "" {
		fmt.Fprintln(os.Stderr, "--fixture-root and --out are required caller-owned directories")
		os.Exit(2)
	}
	evidence, err := coordinator.Run(coordinator.RunInput{
		MetaPath: *metaPath, ContractPath: *contractPath, CandidatesRoot: *candidatesRoot,
		CandidatePaths: candidatePaths, FixtureRoot: *fixtureRoot, OutputRoot: *outputRoot, SourceRoot: *sourceRoot,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("state=%s closed=%d unknown=%d refuted=%d artifacts=%d\n", overallState(evidence.Summary), evidence.Summary.Closed, evidence.Summary.Unknown, evidence.Summary.Refuted, evidence.ArtifactCount)
}

func overallState(summary coordinator.Summary) string {
	if summary.Refuted > 0 {
		return coordinator.StateRefuted
	}
	if summary.Unknown > 0 {
		return coordinator.StateUnknown
	}
	return coordinator.StateClosed
}
