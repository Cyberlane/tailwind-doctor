package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Cyberlane/tailwind-doctor/internal/audit"
	"github.com/Cyberlane/tailwind-doctor/internal/lsp"
)

// Exit codes are part of the public contract; see the README.
const (
	exitSuccess          = 0
	exitBelowThreshold   = 1
	exitOperationalError = 2
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "lsp" {
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "tw-doctor: lsp does not accept arguments")
			os.Exit(exitOperationalError)
		}
		if err := lsp.Serve(os.Stdin, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, "tw-doctor lsp:", err)
			os.Exit(exitOperationalError)
		}
		return
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tw-doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "write a machine-readable report")
	sarifOutput := flags.Bool("sarif", false, "write a SARIF 2.1.0 report for code scanning and editors")
	failUnder := flags.Int("fail-under", 0, "exit 1 when the score is below this value (0-100)")
	requireCoverage := flags.Int("require-coverage", 0,
		"exit 1 when resolved class-list coverage is below this percentage (0-100)")
	version := flags.Bool("version", false, "print the version")
	writeBaseline := flags.Bool("write-baseline", false,
		"record every current finding in "+audit.BaselineFileName+" so later runs gate on new debt only")
	fix := flags.Bool("fix", false, "replace arbitrary values that exactly match a named design token")

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
	if *fix && *writeBaseline {
		fmt.Fprintln(stderr, "tw-doctor: --fix and --write-baseline cannot be combined; choose one write operation")
		return exitOperationalError
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderr, "tw-doctor: expected at most one path")
		return exitOperationalError
	}

	// Validate before scanning: an unusable threshold should be reported
	// immediately rather than after a full audit.
	if *failUnder < 0 || *failUnder > audit.MaximumScore {
		fmt.Fprintf(stderr, "tw-doctor: --fail-under must be between 0 and %d, got %d\n", audit.MaximumScore, *failUnder)
		return exitOperationalError
	}
	if *requireCoverage < 0 || *requireCoverage > 100 {
		fmt.Fprintf(stderr, "tw-doctor: --require-coverage must be between 0 and 100, got %d\n", *requireCoverage)
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
	if *fix {
		fixed, err := audit.Fix(root)
		if err != nil {
			if fixed.Files > 0 {
				fmt.Fprintf(stderr, "tw-doctor: fixed %d arbitrary value(s) in %d file(s) before the failure\n",
					fixed.Replacements, fixed.Files)
			}
			fmt.Fprintln(stderr, "tw-doctor:", err)
			return exitOperationalError
		}
		fmt.Fprintf(stderr, "tw-doctor: fixed %d arbitrary value(s) in %d file(s)\n", fixed.Replacements, fixed.Files)
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
		audit.WriteHumanWith(stdout, report, audit.HumanOptions{FailUnder: *failUnder})
	}

	// The threshold is always applied. A threshold of 0 is a valid, always-passing
	// gate, which keeps a templated `--fail-under $THRESHOLD` honest instead of
	// silently disabling the check.
	if report.Score < *failUnder {
		return exitBelowThreshold
	}
	if report.Coverage.ResolutionPercent < *requireCoverage {
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
