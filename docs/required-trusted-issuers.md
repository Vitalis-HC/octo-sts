# Require an organization issuer list

`OCTOSTS_REQUIRE_TRUSTED_ISSUERS` defaults to `false`. Opt in to refuse exchanges
unless `ORG_POLICY_REPO` contains a readable, valid, enforcing
`OCTOSTS_POLICY_DIR/trusted-token-issuers.yaml`. Missing, unreadable, invalid,
unknown or audit-mode lists return a fixed `FailedPrecondition` naming the file,
with no installation token in the response. An unlisted issuer is still denied.

The deployment's `OCTOSTS_ALLOWED_ISSUERS` remains a separate discovery boundary;
an issuer must pass both that list and the organization list. Existing bounded
org-list caching stays in effect. The option does not bypass GitHub App access
checks or grant the runtime new repository permissions. Omitting the option
retains upstream behavior. Combine it with required audit delivery independently.

For operator acceptance, use a disposable, no-traffic tagged revision whose
ORG_POLICY_REPO names a repository where the App is not installed. Expect the
fixed issuer-list error and no token, then delete that revision. Never weaken
the production list to exercise the refusal. Detailed operator steps accompany
the fork build/rollout documentation; local fixtures are not live acceptance.
