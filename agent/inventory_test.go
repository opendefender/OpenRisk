// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeOsquery answers from canned tables; a table it does not know errors, as
// osqueryi does for a table absent on the platform.
func fakeOsquery(tables map[string][]map[string]string) osqueryFunc {
	return func(_ context.Context, sql string) ([]map[string]string, error) {
		for name, rows := range tables {
			if strings.Contains(sql, "FROM "+name+";") {
				return rows, nil
			}
		}
		return nil, errors.New("no such table")
	}
}

func TestCollectInventory_WithoutOsquery(t *testing.T) {
	_, pkgs, errs := collectInventory(context.Background(), false, nil)
	if len(pkgs) != 0 || len(errs) != 1 || errs[0] != errOsqueryMissing {
		t.Fatalf("want only %q, got pkgs=%v errs=%v", errOsqueryMissing, pkgs, errs)
	}
}

func TestCollectInventory_Debian(t *testing.T) {
	q := fakeOsquery(map[string][]map[string]string{
		"os_version":   {{"name": "Ubuntu", "version": "22.04.3 LTS (Jammy Jellyfish)", "platform": "ubuntu"}},
		"system_info":  {{"hostname": "srv-core-01", "computer_name": ""}},
		"deb_packages": {{"name": "openssl", "version": "3.0.2-0ubuntu1.10", "arch": "amd64"}},
	})
	host, pkgs, errs := collectInventory(context.Background(), true, q)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if host.Platform != "ubuntu" || host.Hostname != "srv-core-01" {
		t.Fatalf("host: %+v", host)
	}
	if len(pkgs) != 1 || pkgs[0].Source != "deb" || pkgs[0].Version != "3.0.2-0ubuntu1.10" {
		t.Fatalf("packages: %+v", pkgs)
	}
}

func TestCollectInventory_RPMKeepsEpoch(t *testing.T) {
	q := fakeOsquery(map[string][]map[string]string{
		"os_version": {{"name": "Rocky Linux", "version": "9.3 (Blue Onyx)", "platform": "rocky"}},
		"rpm_packages": {
			{"name": "openssl", "version": "3.0.7", "release": "27.el9", "epoch": "1", "arch": "x86_64"},
			{"name": "bash", "version": "5.1.8", "release": "9.el9", "epoch": "0", "arch": "x86_64"},
		},
	})
	_, pkgs, _ := collectInventory(context.Background(), true, q)
	if pkgs[0].Version != "1:3.0.7-27.el9" || pkgs[1].Version != "5.1.8-9.el9" {
		t.Fatalf("versions: %q %q", pkgs[0].Version, pkgs[1].Version)
	}
}

func TestCollectInventory_Windows(t *testing.T) {
	q := fakeOsquery(map[string][]map[string]string{
		"os_version": {{"name": "Microsoft Windows 11 Pro", "version": "10.0.22631", "platform": "windows"}},
		"programs":   {{"name": "7-Zip 23.01 (x64)", "version": "23.01", "publisher": "Igor Pavlov"}},
	})
	_, pkgs, errs := collectInventory(context.Background(), true, q)
	if len(errs) != 0 || len(pkgs) != 1 || pkgs[0].Source != "programs" || pkgs[0].Publisher != "Igor Pavlov" {
		t.Fatalf("pkgs=%+v errs=%v", pkgs, errs)
	}
}

func TestCollectInventory_NoPackageTable(t *testing.T) {
	q := fakeOsquery(map[string][]map[string]string{
		"os_version": {{"name": "Arch", "version": "rolling", "platform": "arch"}},
	})
	_, _, errs := collectInventory(context.Background(), true, q)
	if len(errs) != 1 || errs[0] != "no package table readable" {
		t.Fatalf("errs: %v", errs)
	}
}

// The push is signed with the agent's secret over the exact body sent.
func TestPushInventory_Signed(t *testing.T) {
	var gotSig, gotAuth string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/scanner/agent/inventory" {
			http.NotFound(w, r)
			return
		}
		gotSig, gotAuth = r.Header.Get("X-OpenRisk-Signature"), r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := &Agent{Server: srv.URL, State: State{Token: "tok", PushSecret: "s3cret"}}
	if err := a.pushInventory(inventoryBody{JobID: "j1", Collector: "osquery"}); err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write(gotBody)
	if gotSig != hex.EncodeToString(mac.Sum(nil)) || gotAuth != "Bearer tok" {
		t.Fatalf("sig=%s auth=%s", gotSig, gotAuth)
	}
	var b inventoryBody
	if json.Unmarshal(gotBody, &b) != nil || b.JobID != "j1" {
		t.Fatalf("body: %s", gotBody)
	}
}
