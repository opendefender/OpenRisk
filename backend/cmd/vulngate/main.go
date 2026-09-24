// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

// Command vulngate is the dependency vulnerability gate run by CI and by the
// release workflow. It exits 1 when a HIGH/CRITICAL (Trivy) or reachable
// (govulncheck) finding has no valid exception, or when the exception file is
// malformed or holds an expired entry; 2 when its inputs cannot be read.
//
//	vulngate -exceptions security/vulnerability-exceptions.yaml \
//	    -trivy backend.json -trivy frontend.json -trivy image.json \
//	    -govulncheck govulncheck.json
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/opendefender/openrisk/pkg/vulngate"
)

type fileList []string

func (l *fileList) String() string     { return strings.Join(*l, ",") }
func (l *fileList) Set(v string) error { *l = append(*l, v); return nil }

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vulngate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var trivy, govuln fileList
	excPath := fs.String("exceptions", "security/vulnerability-exceptions.yaml", "exceptions file")
	todayFlag := fs.String("today", "", "evaluate expiry as of this YYYY-MM-DD date (default: today, UTC)")
	fs.Var(&trivy, "trivy", "Trivy JSON report (repeatable)")
	fs.Var(&govuln, "govulncheck", "govulncheck -format json output (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(trivy)+len(govuln) == 0 {
		fmt.Fprintln(stderr, "vulngate: at least one -trivy or -govulncheck report is required")
		return 2
	}

	today := time.Now().UTC()
	if *todayFlag != "" {
		t, err := time.Parse("2006-01-02", *todayFlag)
		if err != nil {
			fmt.Fprintf(stderr, "vulngate: -today: %v\n", err)
			return 2
		}
		today = t
	}

	excs, err := readExceptions(*excPath)
	if err != nil {
		fmt.Fprintf(stderr, "vulngate: %v\n", err)
		return 2
	}

	var findings []vulngate.Finding
	for _, p := range trivy {
		f, err := parseFile(p, vulngate.ParseTrivy)
		if err != nil {
			fmt.Fprintf(stderr, "vulngate: %s: %v\n", p, err)
			return 2
		}
		findings = append(findings, f...)
	}
	for _, p := range govuln {
		f, err := parseFile(p, vulngate.ParseGovulncheck)
		if err != nil {
			fmt.Fprintf(stderr, "vulngate: %s: %v\n", p, err)
			return 2
		}
		findings = append(findings, f...)
	}

	valid, excErrs := vulngate.ValidateExceptions(excs, today)
	res := vulngate.Apply(findings, valid)
	if vulngate.Render(stdout, res, excErrs) {
		return 0
	}
	return 1
}

func readExceptions(path string) ([]vulngate.Exception, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return vulngate.ParseExceptions(f)
}

func parseFile(path string, parse func(io.Reader) ([]vulngate.Finding, error)) ([]vulngate.Finding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parse(f)
}
