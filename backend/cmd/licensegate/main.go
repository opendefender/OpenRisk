// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

// Command licensegate is the dependency licence gate run by CI (#457). It
// exits 1 when a dependency carries a licence security/license-policy.yaml
// does not accept, when a licence cannot be resolved, or when the policy file
// is malformed or holds an exception that matches nothing; 2 when its inputs
// cannot be read.
//
//	licensegate -policy security/license-policy.yaml \
//	    -npm-lock frontend/package-lock.json -npm-lock package-lock.json \
//	    -go-modules go-modules.txt -trivy backend-licenses.json
//
// Every tree is required: a run over part of them would report the other
// trees' exceptions as unused. A package installed in two npm trees at the
// same version is checked once.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/opendefender/openrisk/pkg/licensegate"
)

type fileList []string

func (l *fileList) String() string     { return strings.Join(*l, ",") }
func (l *fileList) Set(v string) error { *l = append(*l, v); return nil }

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("licensegate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var trivy, npmLocks fileList
	policyPath := fs.String("policy", "security/license-policy.yaml", "licence policy file")
	fs.Var(&npmLocks, "npm-lock", "package-lock.json of an installed npm tree (repeatable)")
	goMods := fs.String("go-modules", "", "`go list -deps -test` module list, one \"path version\" per line")
	fs.Var(&trivy, "trivy", "Trivy licence report (JSON) for the Go tree (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(npmLocks) == 0 || *goMods == "" || len(trivy) == 0 {
		fmt.Fprintln(stderr, "licensegate: at least one -npm-lock, -go-modules and at least one -trivy are required")
		return 2
	}

	pf, err := os.Open(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "licensegate: %v\n", err)
		return 2
	}
	policy, err := licensegate.ParsePolicy(pf)
	pf.Close()
	if err != nil {
		fmt.Fprintf(stderr, "licensegate: %v\n", err)
		return 2
	}

	var npm []licensegate.Dependency
	skipped := 0
	seen := map[string]bool{}
	for _, p := range npmLocks {
		tree, n, err := licensegate.NPMTree(p)
		if err != nil {
			fmt.Fprintf(stderr, "licensegate: %v\n", err)
			return 2
		}
		skipped += n
		for _, d := range tree {
			if k := d.Name + "@" + d.Version; !seen[k] {
				seen[k] = true
				npm = append(npm, d)
			}
		}
	}

	mf, err := os.Open(*goMods)
	if err != nil {
		fmt.Fprintf(stderr, "licensegate: %v\n", err)
		return 2
	}
	mods, err := licensegate.ParseGoModules(mf)
	mf.Close()
	if err != nil {
		fmt.Fprintf(stderr, "licensegate: %s: %v\n", *goMods, err)
		return 2
	}

	found := map[string][]string{}
	for _, p := range trivy {
		f, err := os.Open(p)
		if err != nil {
			fmt.Fprintf(stderr, "licensegate: %v\n", err)
			return 2
		}
		got, err := licensegate.ParseTrivyLicenses(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(stderr, "licensegate: %s: %v\n", p, err)
			return 2
		}
		for pkg, lics := range got {
		next:
			for _, l := range lics {
				for _, have := range found[pkg] {
					if have == l {
						continue next
					}
				}
				found[pkg] = append(found[pkg], l)
			}
		}
	}

	deps := append(npm, licensegate.GoTree(mods, found)...)
	res := licensegate.Check(policy, deps)
	if licensegate.Render(stdout, res, policy.Validate(), skipped) {
		return 0
	}
	return 1
}
