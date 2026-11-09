// Copyright 2026 Chainguard, Inc.
// SPDX-License-Identifier: Apache-2.0

package octosts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pboidc "chainguard.dev/sdk/proto/platform/oidc/v1"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type auditClient struct {
	cloudevents.Client
	send func(context.Context, cloudevents.Event) cloudevents.Result
}

func (c auditClient) Send(ctx context.Context, e cloudevents.Event) cloudevents.Result {
	return c.send(ctx, e)
}

func TestRequiredAuditDeliveryFaults(t *testing.T) {
	for _, fault := range []string{"ack", "nil-result", "no-client", "encoding", "nack", "403", "500", "transport", "timeout", "cancel", "revoke-failure"} {
		t.Run(fault, func(t *testing.T) {
			var revoked atomic.Int32
			sends := 0
			sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/installation/token" || r.Header.Get("Authorization") != "Bearer synthetic-token" {
					t.Error("incorrect revocation request")
				}
				revoked.Add(1)
				code := http.StatusNoContent
				if fault == "revoke-failure" {
					code = http.StatusInternalServerError
				}
				w.WriteHeader(code)
			}))
			defer sink.Close()
			client := auditClient{send: func(ctx context.Context, e cloudevents.Event) cloudevents.Result {
				sends++
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > auditDeliveryTimeout || e.ID() == "" || strings.Contains(string(e.Data()), "synthetic-token") {
					t.Error("missing bounded deadline/id or token leaked in record")
				}
				switch fault {
				case "ack":
					return cloudevents.ResultACK
				case "nil-result":
					return nil
				case "403":
					return cloudevents.NewHTTPResult(403, "synthetic-token")
				case "500":
					return cloudevents.NewHTTPResult(500, "synthetic-token")
				case "transport":
					return errors.New("synthetic-token")
				case "timeout", "cancel":
					<-ctx.Done()
					return ctx.Err()
				default:
					return cloudevents.ResultNACK
				}
			}}
			s := &sts{ceclient: client, baseURL: sink.URL, requireAuditDelivery: true}
			if fault == "no-client" {
				s.ceclient = nil
			}
			record := Event{Time: time.Now(), TokenSHA256: strings.Repeat("a", 64)}
			if fault == "encoding" {
				record.Time = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
			}
			ctx, cancel := context.WithCancel(t.Context())
			if fault == "timeout" {
				cancel()
				ctx, cancel = context.WithTimeout(t.Context(), 20*time.Millisecond)
			}
			defer cancel()
			if fault == "cancel" {
				cancel()
			}
			token, err := s.auditedToken(ctx, record, "synthetic-token")
			if fault == "ack" {
				if err != nil || token.GetToken() != "synthetic-token" || revoked.Load() != 0 {
					t.Fatal("ack failed")
				}
			} else if token != nil || status.Code(err) != codes.Unavailable || revoked.Load() != 1 || strings.Contains(err.Error(), "synthetic-token") {
				t.Fatalf("failure did not withhold/revoke token: code=%s revokes=%d", status.Code(err), revoked.Load())
			}
			if (fault == "encoding" || fault == "no-client") && sends != 0 {
				t.Fatal("unexpected event send")
			}
		})
	}
}

func TestRequiredAuditRetryID(t *testing.T) {
	var ids []string
	var mu sync.Mutex
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(204)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		ids = append(ids, r.Header.Get("Ce-Id"))
		if len(ids) == 1 {
			w.WriteHeader(503)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer sink.Close()
	client, err := cloudevents.NewClientHTTP(cehttp.WithClient(http.Client{Transport: http.DefaultTransport}), cloudevents.WithTarget(sink.URL))
	if err != nil {
		t.Fatal(err)
	}
	s := &sts{ceclient: client, domain: "fixture.invalid", baseURL: sink.URL}
	token, err := s.auditedToken(t.Context(), Event{Time: time.Now()}, "synthetic-token")
	mu.Lock()
	defer mu.Unlock()
	if err != nil || token == nil || len(ids) != 2 || ids[0] == "" || ids[0] != ids[1] {
		t.Fatalf("retry did not reuse event ID: code=%s count=%d", status.Code(err), len(ids))
	}
}

type auditRevokeTransport struct{}

func (auditRevokeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: make(http.Header), Request: r}, nil
}

func TestRequiredAuditExchangeWithoutMetrics(t *testing.T) {
	previous := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: auditRevokeTransport{}}
	defer func() { http.DefaultClient = previous }()
	for _, ack := range []bool{true, false} {
		t.Run(fmt.Sprint(ack), func(t *testing.T) {
			cleanupOrgIssuers(t, "org")
			forgetTrustPolicy(t, "org")
			ctx := newExchangeContext(t)
			sends := 0
			client := auditClient{send: func(_ context.Context, e cloudevents.Event) cloudevents.Result {
				sends++
				var record Event
				if err := json.Unmarshal(e.Data(), &record); err != nil || len(record.TokenSHA256) != 64 {
					t.Fatal("missing token hash")
				}
				if ack {
					return cloudevents.ResultACK
				}
				return cloudevents.ResultNACK
			}}
			server := NewSecurityTokenServiceServer(routerFor(&fakeInstallMgr{atr: newAppsTransport(t, newFakeGitHub())}, 1), nil, AppSet{}, client, "octosts", false, "", "", "", nil, WithRequiredAuditDelivery(true))
			token, err := server.Exchange(ctx, &pboidc.ExchangeRequest{Identity: "foo", Scopes: []string{"org/repo"}})
			if sends != 1 || (ack && (err != nil || token.GetToken() == "")) || (!ack && (token != nil || status.Code(err) != codes.Unavailable)) {
				t.Fatal("required exchange bypassed receipt")
			}
		})
	}
}
