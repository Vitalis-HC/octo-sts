# Required exchange receipts

`OCTOSTS_REQUIRE_AUDIT_DELIVERY` defaults to `false`. Set it to `true` with
`EVENT_INGRESS_URI` to require an acknowledged receipt before any minted token
is returned. Startup requires a sink and event client, even with `METRICS=false`.
Use a durable receiver that acknowledges only after persistence; an arbitrary
HTTP 2xx endpoint cannot establish durability.

Required mode hashes the minted token, encodes its CloudEvent once and sends it
synchronously with one event ID across SDK retries and a ten-second deadline.
Only an explicit ACK counts; nil results, errors, rejection, encoding failure,
timeout or caller cancellation withhold the token and return `Unavailable`.
The SDK retries its configured transient HTTP codes; a 500 is a refusal, not
an implied acknowledgement. Revocation is best effort with a separate two-second
context that survives caller cancellation. Revocation failure never releases a
token, and neither raw sender/revocation errors nor tokens are logged here.
Denied-exchange telemetry retains its separate optional metrics-controlled path.

This is not an atomic transaction across GitHub and the sink. A process crash
between mint and delivery can leave an unreturned token live until expiry.
An ambiguous timeout can leave a durable receipt for a token never returned.

Fault tests use synthetic tokens, local HTTP servers and injected clients:
`go test ./pkg/octosts ./pkg/envconfig ./cmd/app -run TestRequired`.
The complete upstream suite has historical revocation calls defaulting to
api.github.com. Run local verification with HTTP(S)/ALL_PROXY pointing to a
closed loopback port and NO_PROXY restricted to loopback; never supply live
credentials. Successful local fixtures do not prove the deployed sink's durability.
