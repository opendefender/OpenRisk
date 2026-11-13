// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

// Command catalogsync regenerates the regulatory catalogues that are imported
// from an official machine-readable source rather than typed by hand (#809):
//
//		go run ./cmd/catalogsync -catalog all -out pkg/compliance
//
//	  - nist-800-53-r5 from NIST's OSCAL catalogue (moderate baseline,
//	    resolved profile), pinned to one commit of usnistgov/oscal-content;
//	  - nis2-2022-2555 and dora-2022-2554 from the French XHTML of the Official
//	    Journal, fetched from the Publications Office's Cellar by CELEX number
//	    (EUR-Lex itself answers automated clients with a browser challenge).
//
// Every description it writes is the source text, verbatim. What a person
// chose — which paragraph is a row, and the short name of an EU row — lives in
// spec_*.go next to this file, and the generator fails if a chosen paragraph
// does not exist in the source. No row is written from memory (D-002).
//
// -src reads the sources from a directory instead of the network (file names
// as written by -save), so a regeneration can be reproduced offline.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opendefender/openrisk/pkg/netguard"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("catalogsync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	which := fs.String("catalog", "all", "nist | nis2 | dora | all")
	out := fs.String("out", "pkg/compliance", "directory the catalog_*.go files are written to")
	src := fs.String("src", "", "read sources from this directory instead of downloading them")
	save := fs.String("save", "", "also save the downloaded sources to this directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	jobs := map[string]func(fetcher) (string, []byte, error){
		"nist": genNIST,
		"nis2": genNIS2,
		"dora": genDORA,
	}
	order := []string{"nist", "nis2", "dora"}
	if *which != "all" {
		if _, ok := jobs[*which]; !ok {
			fmt.Fprintf(stderr, "unknown catalog %q\n", *which)
			return 2
		}
		order = []string{*which}
	}

	// The guarded client: public addresses only, redirects followed by hand.
	f := fetcher{dir: *src, save: *save, client: netguard.Client(netguard.Options{Timeout: 2 * time.Minute, NoRedirects: true})}
	for _, k := range order {
		name, code, err := jobs[k](f)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", k, err)
			return 1
		}
		path := filepath.Join(*out, name)
		if err := os.WriteFile(path, code, 0o644); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", k, err)
			return 1
		}
		fmt.Fprintf(stdout, "wrote %s\n", path)
	}
	return 0
}

// fetcher reads a source either from disk (-src) or over HTTP.
type fetcher struct {
	dir, save string
	client    *http.Client
}

// source is one downloaded document and the hash recorded in the generated file.
type source struct {
	URL    string
	SHA256 string
	Body   []byte
}

func (f fetcher) get(file, url string, header map[string]string) (source, error) {
	var body []byte
	if f.dir != "" {
		b, err := os.ReadFile(filepath.Join(f.dir, file))
		if err != nil {
			return source{}, err
		}
		body = b
	} else {
		b, err := f.download(url, header)
		if err != nil {
			return source{}, err
		}
		body = b
		if f.save != "" {
			if err := os.WriteFile(filepath.Join(f.save, file), body, 0o644); err != nil {
				return source{}, err
			}
		}
	}
	sum := sha256.Sum256(body)
	return source{URL: url, SHA256: hex.EncodeToString(sum[:]), Body: body}, nil
}

// download GETs url, following up to five redirects on the same host. The
// Publications Office answers an https request with a redirect to http on its
// own host; that hop is upgraded back to https rather than followed in clear.
func (f fetcher) download(url string, header map[string]string) ([]byte, error) {
	for hop := 0; hop < 5; hop++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range header {
			req.Header.Set(k, v)
		}
		resp, err := f.client.Do(req)
		if err != nil {
			return nil, err
		}
		// The largest source (the NIST catalogue) is about 4 MB.
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		switch {
		case resp.StatusCode == http.StatusOK:
			return body, nil
		case resp.StatusCode >= 300 && resp.StatusCode < 400:
			loc, err := resp.Location()
			if err != nil {
				return nil, fmt.Errorf("GET %s: %s without a usable Location", url, resp.Status)
			}
			if !strings.EqualFold(loc.Hostname(), req.URL.Hostname()) {
				return nil, fmt.Errorf("GET %s: redirect to another host %q refused", url, loc.Host)
			}
			loc.Scheme = "https"
			url = loc.String()
		default:
			return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
		}
	}
	return nil, fmt.Errorf("GET %s: too many redirects", url)
}
