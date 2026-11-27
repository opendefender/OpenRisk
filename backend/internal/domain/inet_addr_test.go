// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// #1519: Postgres hands an inet column back as text. *net.IP could not be
// scanned from it, so every audit row that carried an address failed the read.

func TestINetAddr_Scan_Success(t *testing.T) {
	for name, in := range map[string]interface{}{
		"text":          "203.0.113.7",
		"text+prefix":   "203.0.113.7/32",
		"bytes as text": []byte("203.0.113.7"),
		"raw 4 bytes":   []byte{203, 0, 113, 7},
	} {
		var a INetAddr
		require.NoError(t, a.Scan(in), name)
		require.True(t, net.IP(a).Equal(net.ParseIP("203.0.113.7")), name)
		require.Equal(t, "203.0.113.7", a.String(), name)
	}

	var v6 INetAddr
	require.NoError(t, v6.Scan("2001:db8::1"))
	require.Equal(t, "2001:db8::1", v6.String())
}

func TestINetAddr_Scan_NullAndInvalid(t *testing.T) {
	a := INetAddr(net.ParseIP("10.0.0.1"))
	require.NoError(t, a.Scan(nil))
	require.Empty(t, a.String())

	require.Error(t, a.Scan("not-an-ip"))
	require.Error(t, a.Scan(42))
}

func TestINetAddr_Value_WritesTextAndNullForEmpty(t *testing.T) {
	v, err := INetAddr(net.ParseIP("203.0.113.7")).Value()
	require.NoError(t, err)
	require.Equal(t, "203.0.113.7", v)

	v, err = INetAddr(nil).Value()
	require.NoError(t, err)
	require.Nil(t, v)
}
