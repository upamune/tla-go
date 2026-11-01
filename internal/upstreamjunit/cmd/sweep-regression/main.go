// Command sweep-regression flags newly observed selected diagnostic
// differences against a pinned partial sweep. It does not run JUnit tests.
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/upamune/tla-go/internal/upstreamjunit"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errout io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(errout, "usage: sweep-regression FULL-SWEEP-LOG")
		return 2
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(errout, err)
		return 2
	}
	baseline, err := upstreamjunit.PinnedRegressionBaseline()
	if err != nil {
		fmt.Fprintln(errout, err)
		return 2
	}
	candidate, err := upstreamjunit.ParseSweepLog(bytes.NewReader(data))
	if err != nil {
		fmt.Fprintln(errout, err)
		return 2
	}
	result, err := upstreamjunit.CompareSweepRegression(baseline, candidate)
	if err != nil {
		fmt.Fprintln(errout, err)
		return 2
	}
	return writeReport(out, baseline, result, sha256.Sum256(data))
}

func writeReport(out io.Writer, baseline upstreamjunit.SweepSnapshot, result upstreamjunit.SweepRegression, digest [32]byte) int {
	fmt.Fprintf(out, "Diagnostic comparison only; this is not a JUnit pass.\nBaseline: %s; upstream: %s; jar SHA-256: %s\n", baseline.TlaGoCommit, baseline.UpstreamCommit, baseline.JarSHA256)
	fmt.Fprintf(out, "Candidate log SHA-256: %x; executable and oracle provenance are not established by the log.\n", digest)
	fmt.Fprintf(out, "Compared: %d; baseline selected mismatch cases: %d; candidate selected mismatch cases: %d; baseline-unresolved: %d; excluded: %d\n", result.Compared, result.ExistingMismatches, result.RemainingMismatches, len(result.Unresolved), len(result.Excluded))
	for _, exclusion := range result.Excluded {
		fmt.Fprintf(out, "excluded %s: %s\n", exclusion.Class, exclusion.Detail)
	}
	for _, unknown := range result.Unresolved {
		fmt.Fprintf(out, "baseline-unresolved %s: %s %s\n", unknown.Class, unknown.Kind, unknown.Detail)
	}
	for _, finding := range result.Findings {
		fmt.Fprintf(out, "%s %s: %s\n", finding.Kind, finding.Class, finding.Detail)
	}
	fmt.Fprintln(out, "New differences require reproduction; unchanged categories can hide changed values. Unavailable and excluded cases remain unresolved.")
	return reportExit(result)
}

func reportExit(result upstreamjunit.SweepRegression) int {
	code := 0
	if len(result.Unresolved) > 0 || len(result.Excluded) > 0 || result.RemainingMismatches > 0 {
		code = 2
	}
	for _, finding := range result.Findings {
		switch finding.Kind {
		case "new_difference":
			code = 1
		case "no_longer_observed":
			// An absent category is only an observation, not a verified fix.
		default:
			if code == 0 {
				code = 2
			}
		}
	}
	return code
}
