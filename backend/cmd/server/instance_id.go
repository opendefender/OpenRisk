// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"os"

	"github.com/google/uuid"
)

// instanceID names this process among the replicas: the hostname (the pod name
// on Kubernetes) plus a random suffix, so two processes on one host differ.
func instanceID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "backend"
	}
	return host + "-" + uuid.NewString()[:8]
}
