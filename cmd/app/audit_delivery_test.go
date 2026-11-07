// Copyright 2026 Chainguard, Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	envConfig "github.com/octo-sts/app/pkg/envconfig"
)

func TestRequiredEventClientWithoutMetrics(t *testing.T) {
	for _, sink := range []string{"", "http://127.0.0.1:1"} {
		cfg := &envConfig.EnvConfigApp{RequireAuditDelivery: true, EventingIngress: sink}
		client, err := newEventClient(t.Context(), false, cfg)
		if sink == "" && (err == nil || client != nil) {
			t.Fatal("missing sink allowed startup")
		}
		if sink != "" && (err != nil || client == nil) {
			t.Fatal("metrics disabled required event client")
		}
	}
}
