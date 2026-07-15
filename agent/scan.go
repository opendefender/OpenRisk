// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ScanTimeout is the hard per-scan cap (matches the SaaS 15-minute deadline).
const ScanTimeout = 15 * time.Minute

// minPrefixIPv4 — targets must be /24 or smaller (mirrors the SaaS scope rule).
const minPrefixIPv4 = 24

var cveRe = regexp.MustCompile(`CVE-\d{4}-\d{4,7}`)

// --- discovery DTOs (JSON shapes match internal/scanner in the backend) -----

type AssetDiscovery struct {
	ExternalID  string   `json:"external_id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	IP          *string  `json:"ip,omitempty"`
	Hostname    *string  `json:"hostname,omitempty"`
	OS          *string  `json:"os,omitempty"`
	CPE         []string `json:"cpe"`
	Criticality float64  `json:"criticality"`
	Environment string   `json:"environment"`
	Tags        []string `json:"tags"`
}

type FindingDiscovery struct {
	CVE             *string  `json:"cve,omitempty"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Severity        string   `json:"severity"`
	AffectedCPE     []string `json:"affected_cpe"`
	Evidence        string   `json:"evidence"`
	RemediationHint string   `json:"remediation_hint"`
	Source          string   `json:"source"`
	AssetExternalID string   `json:"asset_external_id,omitempty"`
}

type pushBody struct {
	JobID    string             `json:"job_id"`
	Assets   []AssetDiscovery   `json:"assets"`
	Findings []FindingDiscovery `json:"findings"`
	Errors   []string           `json:"errors"`
}

// runJob runs a scan for a dispatched job and pushes the results.
func (a *Agent) runJob(ctx context.Context, d jobDispatch) {
	log.Printf("job %s: scanning %v", d.JobID, d.Targets)
	_ = a.heartbeat("scanning")
	defer func() { _ = a.heartbeat("online") }()

	var scanErrs []string
	var valid []string
	for _, t := range d.Targets {
		if err := validateTarget(t); err != nil {
			scanErrs = append(scanErrs, err.Error())
		} else {
			valid = append(valid, t)
		}
	}

	var assets []AssetDiscovery
	var findings []FindingDiscovery
	if len(valid) > 0 {
		sctx, cancel := context.WithTimeout(ctx, ScanTimeout)
		xmlOut, err := runNmap(sctx, valid)
		cancel()
		if len(xmlOut) > 0 {
			as, fs, perr := parseNmap(xmlOut)
			assets, findings = as, fs
			if perr != nil {
				scanErrs = append(scanErrs, "nmap parse: "+perr.Error())
			}
		}
		if err != nil && len(assets) == 0 {
			scanErrs = append(scanErrs, "nmap: "+err.Error())
		}
		if oa := runOsquery(ctx); len(oa) > 0 {
			assets = append(assets, oa...)
		}
	}

	if err := a.push(d.JobID, assets, findings, scanErrs); err != nil {
		log.Printf("job %s: push failed: %v", d.JobID, err)
		return
	}
	log.Printf("job %s: pushed %d assets, %d findings", d.JobID, len(assets), len(findings))
}

// --- nmap ------------------------------------------------------------------

func runNmap(ctx context.Context, targets []string) ([]byte, error) {
	args := []string{"-sV", "--script", "vuln", "-oX", "-", "--min-rate", "1000", "--max-retries", "3", "--host-timeout", "15m"}
	if os.Geteuid() == 0 {
		args = append(args, "-O") // OS detection needs raw sockets (root)
	}
	args = append(args, targets...)
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "nmap", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.Bytes(), fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

type nmapRun struct {
	Hosts []nmapHost `xml:"host"`
}
type nmapHost struct {
	Status struct {
		State string `xml:"state,attr"`
	} `xml:"status"`
	Addresses []struct {
		Addr string `xml:"addr,attr"`
		Type string `xml:"addrtype,attr"`
	} `xml:"address"`
	Hostnames struct {
		Names []struct {
			Name string `xml:"name,attr"`
		} `xml:"hostname"`
	} `xml:"hostnames"`
	Ports struct {
		Ports []nmapPort `xml:"port"`
	} `xml:"ports"`
	OS struct {
		Matches []struct {
			Name string `xml:"name,attr"`
		} `xml:"osmatch"`
	} `xml:"os"`
}
type nmapPort struct {
	Protocol string `xml:"protocol,attr"`
	PortID   string `xml:"portid,attr"`
	State    struct {
		State string `xml:"state,attr"`
	} `xml:"state"`
	Service struct {
		Name    string   `xml:"name,attr"`
		Product string   `xml:"product,attr"`
		Version string   `xml:"version,attr"`
		CPEs    []string `xml:"cpe"`
	} `xml:"service"`
	Scripts []struct {
		ID     string `xml:"id,attr"`
		Output string `xml:"output,attr"`
	} `xml:"script"`
}

func parseNmap(data []byte) ([]AssetDiscovery, []FindingDiscovery, error) {
	var run nmapRun
	if err := xml.Unmarshal(data, &run); err != nil {
		return nil, nil, err
	}
	var assets []AssetDiscovery
	var findings []FindingDiscovery
	for _, h := range run.Hosts {
		if h.Status.State != "up" {
			continue
		}
		ip := ""
		for _, ad := range h.Addresses {
			if ad.Type == "ipv4" || ad.Type == "ipv6" {
				ip = ad.Addr
			}
		}
		hostname := ""
		if len(h.Hostnames.Names) > 0 {
			hostname = h.Hostnames.Names[0].Name
		}
		extID := ip
		if extID == "" {
			extID = hostname
		}
		if extID == "" {
			continue
		}

		cpeSet := map[string]struct{}{}
		var openPorts []string
		seenCVE := map[string]struct{}{}
		for _, p := range h.Ports.Ports {
			if p.State.State != "open" {
				continue
			}
			openPorts = append(openPorts, fmt.Sprintf("%s/%s %s", p.PortID, p.Protocol, p.Service.Name))
			for _, c := range p.Service.CPEs {
				cpeSet[strings.ToLower(strings.TrimSpace(c))] = struct{}{}
			}
			for _, s := range p.Scripts {
				for _, cve := range cveRe.FindAllString(s.Output, -1) {
					cve = strings.ToUpper(cve)
					if _, dup := seenCVE[cve]; dup {
						continue
					}
					seenCVE[cve] = struct{}{}
					c := cve
					svc := strings.TrimSpace(p.Service.Product + " " + p.Service.Version)
					findings = append(findings, FindingDiscovery{
						CVE:             &c,
						Title:           fmt.Sprintf("%s on %s:%s", cve, extID, p.PortID),
						Description:     fmt.Sprintf("nmap %s script reported %s.", s.ID, cve),
						Severity:        "medium",
						Evidence:        fmt.Sprintf("port %s/%s (%s %s)", p.PortID, p.Protocol, p.Service.Name, svc),
						RemediationHint: "Patch or upgrade the affected service to a fixed version.",
						Source:          "nmap",
						AssetExternalID: extID,
						AffectedCPE:     cpeList(p.Service.CPEs),
					})
				}
			}
		}

		os := ""
		if len(h.OS.Matches) > 0 {
			os = h.OS.Matches[0].Name
		}
		a := AssetDiscovery{
			ExternalID: extID,
			Name:       firstNonEmptyStr(hostname, ip),
			Type:       "Server",
			CPE:        setToSorted(cpeSet),
			Tags:       append([]string{"nmap", "on-prem"}, openPorts...),
		}
		if ip != "" {
			a.IP = &ip
		}
		if hostname != "" {
			a.Hostname = &hostname
		}
		if os != "" {
			a.OS = &os
		}
		assets = append(assets, a)
	}
	return assets, findings, nil
}

// --- osquery (optional inventory) ------------------------------------------

// runOsquery augments discovery with local inventory when osqueryi is present.
// Absent → returns nil (nmap alone is enough). Kept intentionally small.
func runOsquery(ctx context.Context) []AssetDiscovery {
	if _, err := exec.LookPath("osqueryi"); err != nil {
		return nil
	}
	octx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(octx, "osqueryi", "--json",
		"SELECT hostname, computer_name, hardware_serial FROM system_info;").Output()
	if err != nil {
		return nil
	}
	var rows []map[string]string
	if json.Unmarshal(out, &rows) != nil || len(rows) == 0 {
		return nil
	}
	host := firstNonEmptyStr(rows[0]["computer_name"], rows[0]["hostname"])
	if host == "" {
		return nil
	}
	return []AssetDiscovery{{
		ExternalID: "osquery:" + host,
		Name:       host,
		Type:       "Workstation",
		Tags:       []string{"osquery", "on-prem"},
	}}
}

// --- push (HMAC-signed) ----------------------------------------------------

func (a *Agent) push(jobID string, assets []AssetDiscovery, findings []FindingDiscovery, errs []string) error {
	body, _ := json.Marshal(pushBody{JobID: jobID, Assets: assets, Findings: findings, Errors: errs})
	mac := hmac.New(sha256.New, []byte(a.State.PushSecret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest(http.MethodPost, a.Server+"/api/v1/scanner/agent/push", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+a.State.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenRisk-Signature", sig)
	resp, err := a.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("push HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// --- helpers ---------------------------------------------------------------

func validateTarget(t string) error {
	t = strings.TrimSpace(t)
	if t == "" {
		return fmt.Errorf("empty target")
	}
	if strings.Contains(t, "/") {
		ip, ipnet, err := net.ParseCIDR(t)
		if err != nil {
			return fmt.Errorf("invalid CIDR %q", t)
		}
		ones, _ := ipnet.Mask.Size()
		if ip.To4() != nil && ones < minPrefixIPv4 {
			return fmt.Errorf("target %q wider than /24 — refused (scope limit)", t)
		}
		if ip.To4() == nil && ones < 120 {
			return fmt.Errorf("IPv6 target %q wider than /120 — refused", t)
		}
		return nil
	}
	if net.ParseIP(t) != nil {
		return nil
	}
	// bare hostname
	if len(t) <= 253 {
		return nil
	}
	return fmt.Errorf("invalid target %q", t)
}

func cpeList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, c := range in {
		if c = strings.ToLower(strings.TrimSpace(c)); c != "" {
			out = append(out, c)
		}
	}
	return out
}

func setToSorted(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
