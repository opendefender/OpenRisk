// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// --- software inventory (#910) ---------------------------------------------
//
// An "inventory.job" asks the agent for the packages installed on the host it
// runs on. osquery is required (owner decision D-071): without osqueryi the
// agent reports the error code "osquery_missing" and no packages, so the
// console can say what to install. Only this host is read; remote hosts are
// out of reach.

const inventoryTimeout = 2 * time.Minute

// errOsqueryMissing is the code the backend and console understand.
const errOsqueryMissing = "osquery_missing"

type inventoryHost struct {
	Hostname  string `json:"hostname"`
	Platform  string `json:"platform"`
	OS        string `json:"os"`
	OSVersion string `json:"os_version"`
}

type inventoryPackage struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Source    string `json:"source"`
	Publisher string `json:"publisher,omitempty"`
	Arch      string `json:"arch,omitempty"`
}

type inventoryBody struct {
	JobID     string             `json:"job_id"`
	Host      inventoryHost      `json:"host"`
	Packages  []inventoryPackage `json:"packages"`
	Collector string             `json:"collector"`
	Errors    []string           `json:"errors"`
}

// osqueryFunc runs one SQL statement through osqueryi and returns its rows.
// A seam for tests; production uses runOsqueryi.
type osqueryFunc func(ctx context.Context, sql string) ([]map[string]string, error)

func runOsqueryi(ctx context.Context, sql string) ([]map[string]string, error) {
	out, err := exec.CommandContext(ctx, "osqueryi", "--json", sql).Output()
	if err != nil {
		return nil, err
	}
	var rows []map[string]string
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// collectInventory reads the host and its packages. hasOsquery reports whether
// osqueryi is installed; it is a parameter so tests need no binary.
func collectInventory(ctx context.Context, hasOsquery bool, q osqueryFunc) (inventoryHost, []inventoryPackage, []string) {
	if !hasOsquery {
		return inventoryHost{}, nil, []string{errOsqueryMissing}
	}
	var errs []string
	host := inventoryHost{}
	if rows, err := q(ctx, "SELECT name, version, platform FROM os_version;"); err == nil && len(rows) > 0 {
		host.OS, host.OSVersion, host.Platform = rows[0]["name"], rows[0]["version"], rows[0]["platform"]
	} else {
		errs = append(errs, "os_version unreadable")
	}
	if rows, err := q(ctx, "SELECT hostname, computer_name FROM system_info;"); err == nil && len(rows) > 0 {
		host.Hostname = firstNonEmptyStr(rows[0]["hostname"], rows[0]["computer_name"])
	}

	var pkgs []inventoryPackage
	if strings.EqualFold(host.Platform, "windows") {
		rows, err := q(ctx, "SELECT name, version, publisher FROM programs;")
		if err != nil {
			errs = append(errs, "programs unreadable")
		}
		for _, r := range rows {
			pkgs = append(pkgs, inventoryPackage{Name: r["name"], Version: r["version"], Publisher: r["publisher"], Source: "programs"})
		}
		return host, pkgs, errs
	}

	// Linux: each package manager's table exists only where it is installed;
	// a missing table is not an error, an unreadable present one is.
	found := false
	if rows, err := q(ctx, "SELECT name, version, arch FROM deb_packages;"); err == nil {
		found = found || len(rows) > 0
		for _, r := range rows {
			pkgs = append(pkgs, inventoryPackage{Name: r["name"], Version: r["version"], Arch: r["arch"], Source: "deb"})
		}
	}
	if rows, err := q(ctx, "SELECT name, version, release, epoch, arch FROM rpm_packages;"); err == nil {
		found = found || len(rows) > 0
		for _, r := range rows {
			pkgs = append(pkgs, inventoryPackage{Name: r["name"], Version: rpmEVR(r["epoch"], r["version"], r["release"]), Arch: r["arch"], Source: "rpm"})
		}
	}
	if rows, err := q(ctx, "SELECT name, version, arch FROM apk_packages;"); err == nil {
		found = found || len(rows) > 0
		for _, r := range rows {
			pkgs = append(pkgs, inventoryPackage{Name: r["name"], Version: r["version"], Arch: r["arch"], Source: "apk"})
		}
	}
	if !found {
		errs = append(errs, "no package table readable")
	}
	return host, pkgs, errs
}

// rpmEVR writes an RPM version the way distribution advisories do:
// [epoch:]version-release, with no epoch when it is empty or 0.
func rpmEVR(epoch, version, release string) string {
	v := version
	if release != "" {
		v += "-" + release
	}
	if epoch != "" && epoch != "0" {
		v = epoch + ":" + v
	}
	return v
}

// runInventory collects and pushes the inventory for a dispatched job.
func (a *Agent) runInventory(ctx context.Context, d jobDispatch) {
	ictx, cancel := context.WithTimeout(ctx, inventoryTimeout)
	defer cancel()
	_, lookErr := exec.LookPath("osqueryi")
	host, pkgs, errs := collectInventory(ictx, lookErr == nil, runOsqueryi)
	collector := "osquery"
	if lookErr != nil {
		collector = ""
		log.Printf("inventory %s: osqueryi not found — install osquery (see README)", d.JobID)
	}
	body := inventoryBody{JobID: d.JobID, Host: host, Packages: pkgs, Collector: collector, Errors: errs}
	if err := a.pushInventory(body); err != nil {
		log.Printf("inventory %s: push failed: %v", d.JobID, err)
		return
	}
	log.Printf("inventory %s: pushed %d packages", d.JobID, len(pkgs))
}

// pushInventory sends the inventory, signed like a scan push.
func (a *Agent) pushInventory(b inventoryBody) error {
	body, err := json.Marshal(b)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, []byte(a.State.PushSecret))
	mac.Write(body)
	req, _ := http.NewRequest(http.MethodPost, a.Server+"/api/v1/scanner/agent/inventory", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+a.State.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenRisk-Signature", hex.EncodeToString(mac.Sum(nil)))
	resp, err := a.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("inventory HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}
