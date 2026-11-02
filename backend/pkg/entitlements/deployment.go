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

// ParseDeployment reads the DeploymentEnv value. There is no default (D-081 C):
// an empty value is an error, because reading it as hosted caps a self-hoster at
// Free and reading it as self_hosted opens the SaaS to every tenant. An unknown
// value is an error for the same reason. Either way the server refuses to boot.
func ParseDeployment(s string) (Deployment, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return "", fmt.Errorf("%s is not set: set it to %q for your own install, or %q for the OpenDefender hosted service",
			DeploymentEnv, DeploymentSelfHosted, DeploymentHosted)
	case "hosted", "saas":
		return DeploymentHosted, nil
	case "self_hosted", "self-hosted", "selfhosted":
		return DeploymentSelfHosted, nil
	default:
		return "", fmt.Errorf("%s=%q: want %q or %q", DeploymentEnv, s, DeploymentSelfHosted, DeploymentHosted)
	}
}
