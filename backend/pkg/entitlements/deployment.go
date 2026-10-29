// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entitlements

import (
	"fmt"
	"strings"
)

// Deployment says who runs this instance. It is a property of the process, read
// once at boot from DeploymentEnv, never a tenant value (#612).
type Deployment string

const (
	// DeploymentHosted is the owner's SaaS: tenants resolve by subscription/plan.
	DeploymentHosted Deployment = "hosted"
	// DeploymentSelfHosted is an operator's own install: every tenant resolves to
	// PlanSelfHosted (D-040).
	DeploymentSelfHosted Deployment = "self_hosted"
)

// DeploymentEnv is the environment variable that declares the deployment.
const DeploymentEnv = "OPENRISK_DEPLOYMENT"

// ParseDeployment reads the DeploymentEnv value. Empty means hosted, which keeps
// an instance that never set the variable resolving exactly as before. Any other
// unknown value is an error: a typo silently read as hosted would cap a
// self-hoster at Free, so the server refuses to boot instead.
func ParseDeployment(s string) (Deployment, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "hosted", "saas":
		return DeploymentHosted, nil
	case "self_hosted", "self-hosted", "selfhosted":
		return DeploymentSelfHosted, nil
	default:
		return "", fmt.Errorf("%s=%q: want %q or %q", DeploymentEnv, s, DeploymentSelfHosted, DeploymentHosted)
	}
}
