package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cyberlane/tailwind-doctor/internal/audit"
)

func main() {
	jsonOutput := flag.Bool("json", false, "write a machine-readable report")
	failUnder := flag.Int("fail-under", 0, "exit non-zero when the score is below this value")
	version := flag.Bool("version", false, "print the version")
	flag.Parse()

	if *version {
		fmt.Println("tw-doctor dev")
		return
	}

	path := "."
	if flag.NArg() > 0 {
		path = flag.Arg(0)
	}

	report, err := audit.Run(filepath.Clean(path))
	if err != nil {
		fmt.Fprintln(os.Stderr, "tw-doctor:", err)
		os.Exit(2)
	}

	if *jsonOutput {
		if err := audit.WriteJSON(os.Stdout, report); err != nil {
			fmt.Fprintln(os.Stderr, "tw-doctor:", err)
			os.Exit(2)
		}
	} else {
		audit.WriteHuman(os.Stdout, report)
	}

	if *failUnder > 0 && report.Score < *failUnder {
		os.Exit(1)
	}
}
