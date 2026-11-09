// Copyright 2026 Chainguard, Inc.
// SPDX-License-Identifier: Apache-2.0

package octosts

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	pboidc "chainguard.dev/sdk/proto/platform/oidc/v1"
	"github.com/chainguard-dev/clog"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ServerOption configures optional STS behavior without changing existing callers.
type ServerOption func(*sts)

// WithRequiredAuditDelivery withholds tokens unless the event sink acknowledges them.
func WithRequiredAuditDelivery(required bool) ServerOption {
	return func(s *sts) { s.requireAuditDelivery = required }
}

const auditDeliveryTimeout = 10 * time.Second

// auditedToken is called only after minting and hashing. No token-bearing error
// or transport result may reach logs or the response on this path.
func (s *sts) auditedToken(ctx context.Context, record Event, token string) (*pboidc.RawToken, error) {
	if s.ceclient != nil {
		event := cloudevents.NewEvent()
		event.SetID(rand.Text()) // One ID and one encoding across all SDK retries.
		event.SetType("dev.octo-sts.exchange")
		event.SetSubject(fmt.Sprintf("%s/%s", record.Scope, record.Identity))
		event.SetSource(fmt.Sprintf("https://%s", s.domain))
		if err := event.SetData(cloudevents.ApplicationJSON, record); err == nil {
			delivery, cancel := context.WithTimeout(ctx, auditDeliveryTimeout)
			retries := cloudevents.ContextWithRetriesExponentialBackoff(delivery, retryDelay, maxRetry)
			result := s.ceclient.Send(retries, event)
			ack := result != nil && cloudevents.IsACK(result) && delivery.Err() == nil
			cancel()
			if ack {
				return &pboidc.RawToken{Token: token}, nil
			}
		}
	}
	clog.ErrorContext(ctx, "required audit delivery failed; withholding token")
	// Cancellation of the caller or delivery must not cancel best-effort revocation.
	revoke, cancel := context.WithTimeout(context.WithoutCancel(ctx), policyReadRevokeTimeout)
	defer cancel()
	if err := Revoke(revoke, token, s.baseURL); err != nil {
		clog.ErrorContext(ctx, "audit failure token revocation failed")
	}
	return nil, status.Error(codes.Unavailable, "required audit delivery unavailable")
}
