package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Cyberlane/tailwind-doctor/internal/audit"
)

// Exit codes are part of the public contract; see the README.
const (
	exitSuccess          = 0
	exitBelowThreshold   = 1
	exitOperationalError = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tw-doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "write a machine-readable report")
	sarifOutput := flags.Bool("sarif", false, "write a SARIF 2.1.0 report for code scanning and editors")
	failUnder := flags.Int("fail-under", 0, "exit 1 when the score is below this value (0-100)")
	version := flags.Bool("version", false, "print the version")
	writeBaseline := flags.Bool("write-baseline", false,
		"record every current finding in "+audit.BaselineFileName+" so later runs gate on new debt only")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		return exitOperationalError
	}

	if *version {
		fmt.Fprintln(stdout, "tw-doctor "+audit.Version)
		return exitSuccess
	}

	// One report, one format. Interleaving two on the same stream would produce
	// something neither consumer can parse.
	if *jsonOutput && *sarifOutput {
		fmt.Fprintln(stderr, "tw-doctor: --json and --sarif cannot be combined; choose one")
		return exitOperationalError
	}

	// Validate before scanning: an unusable threshold should be reported
	// immediately rather than after a full audit.
	if *failUnder < 0 || *failUnder > audit.MaximumScore {
		fmt.Fprintf(stderr, "tw-doctor: --fail-under must be between 0 and %d, got %d\n", audit.MaximumScore, *failUnder)
		return exitOperationalError
	}

	path := "."
	if flags.NArg() > 0 {
		path = flags.Arg(0)
	}

	root := filepath.Clean(path)

	if *writeBaseline {
		return writeBaselineFile(root, stdout, stderr)
	}

	report, err := audit.Run(root)
	if err != nil {
		fmt.Fprintln(stderr, "tw-doctor:", err)
		return exitOperationalError
	}

	switch {
	case *jsonOutput:
		if err := audit.WriteJSON(stdout, report); err != nil {
			fmt.Fprintln(stderr, "tw-doctor:", err)
			return exitOperationalError
		}
	case *sarifOutput:
		if err := audit.WriteSARIF(stdout, report); err != nil {
			fmt.Fprintln(stderr, "tw-doctor:", err)
			return exitOperationalError
		}
	default:
		audit.WriteHuman(stdout, report)
	}

	// The threshold is always applied. A threshold of 0 is a valid, always-passing
	// gate, which keeps a templated `--fail-under $THRESHOLD` honest instead of
	// silently disabling the check.
	if report.Score < *failUnder {
		return exitBelowThreshold
	}
	return exitSuccess
}

// writeBaselineFile records the project's current findings. It deliberately runs
// without the existing baseline, so the file it writes describes the codebase
// rather than the difference from the last time it was written.
func writeBaselineFile(root string, stdout, stderr io.Writer) int {
	config, err := audit.LoadConfig(root)
	if err != nil {
		fmt.Fprintln(stderr, "tw-doctor:", err)
		return exitOperationalError
	}

	report, err := audit.RunWithConfig(root, config, nil)
	if err != nil {
		fmt.Fprintln(stderr, "tw-doctor:", err)
		return exitOperationalError
	}

	destination := filepath.Join(root, config.BaselinePath)
	baseline := audit.NewBaseline(report)
	if err := audit.WriteBaseline(destination, baseline); err != nil {
		fmt.Fprintln(stderr, "tw-doctor:", err)
		return exitOperationalError
	}

	fmt.Fprintf(stdout, "Wrote %s with %d suppressed finding(s).\n", destination, len(baseline.Suppressed))
	return exitSuccess
}
