// Copyright 2026 Chainguard, Inc.
// SPDX-License-Identifier: Apache-2.0

package octosts

import (
	"net/http"
	"strings"
	"testing"

	pboidc "chainguard.dev/sdk/proto/platform/oidc/v1"
	"github.com/octo-sts/app/pkg/ghinstall"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRequiredTrustedIssuers(t *testing.T) {
	previous := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: auditRevokeTransport{}}
	defer func() { http.DefaultClient = previous }()
	for _, fault := range []string{"absent", "unreadable", "invalid", "audit", "unknown", "unlisted", "listed"} {
		t.Run(fault, func(t *testing.T) {
			cleanupOrgIssuers(t, "org")
			forgetTrustPolicy(t, "org")
			atr := newAppsTransport(t, newFakeGitHub())
			manager := ghinstall.Manager(&fakeInstallMgr{atr: atr})
			switch fault {
			case "absent":
				orgIssuers.Add("org", absentOrgIssuerEntry())
			case "invalid":
				orgIssuers.Add("org", invalidOrgIssuerEntry(status.Error(codes.Internal, "synthetic-token")))
			case "unknown":
				orgIssuers.Add("org", orgIssuerEntry{})
			case "unreadable":
				atr = newAppsTransport(t, newOrgFakeGitHub(withContentsRateLimited()))
				manager = &enumMgr{installs: []ghinstall.Installation{{Transport: atr, ID: 1, AppID: 10}}}
			default:
				cfg := OrgTrustedIssuers{Mode: ModeEnforce, Issuers: []string{testGitHubIssuer}}
				if fault == "audit" {
					cfg.Mode = ModeAudit
				}
				if fault == "unlisted" {
					cfg.Issuers = []string{"https://accounts.google.com"}
				}
				allow, err := cfg.Compile()
				if err != nil {
					t.Fatal(err)
				}
				orgIssuers.Add("org", presentOrgIssuerEntry(allow))
			}
			server := NewSecurityTokenServiceServer(routerFor(manager, 1), nil, AppSet{}, nil, "octosts", false, "", "", "", []string{testGitHubIssuer}, WithRequiredTrustedIssuers(true))
			token, err := server.Exchange(newExchangeContext(t), &pboidc.ExchangeRequest{Identity: "foo", Scopes: []string{"org/repo"}})
			if fault == "listed" {
				if err != nil || token.GetToken() == "" {
					t.Fatal("issuer present in both lists was refused")
				}
				return
			}
			if token != nil || err == nil || strings.Contains(err.Error(), "synthetic-token") {
				t.Fatal("issuer-list failure returned a token or leaked details")
			}
			if fault == "unlisted" {
				if status.Code(err) != codes.PermissionDenied {
					t.Fatal("deployment list bypassed org list")
				}
			} else if status.Code(err) != codes.FailedPrecondition || status.Convert(err).Message() != msgRequiredIssuerList {
				t.Fatalf("missing sanitized issuer-list error: %s", status.Code(err))
			}
		})
	}
}
