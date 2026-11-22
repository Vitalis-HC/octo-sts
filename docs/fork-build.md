# Reproducible fork image

The release line `vitalis/main` starts at upstream commit
270e72e2ac2b1f6830c3d15987697d37bd5d65c4 (upstream v0.14.2) and has since merged
upstream release v0.14.3. It takes upstream releases, not upstream main. It was
previously named `vitalis/v0.14.2`; images built under that name attest
`refs/heads/vitalis/v0.14.2` as their source ref.
Maintainers choose the default branch and protect the release line before use.

With Go1.27.3 and ko0.19.1 on PATH, from a clean checkout, run:
```sh
.github/scripts/build-fork.sh /absolute/path/to/new-output-directory
```
The script builds linux/amd64 from the checkout and an independent git archive,
using a pinned static base, fixed epoch and static, trimpath, no-VCS/build-ID
flags. OCI digests must match. It needs no Docker daemon or registry credentials
and never pushes. The output archive contains OCI, SPDX SBOM and source/digest
metadata. The build script refuses existing output directories or a dirty tree.

The fork workflow tests and builds PRs with Contents read only. A release-line
push has a separate attestation job that downloads the immutable artifact and
attests its archive without checking out or executing source. Verify the signer,
source ref/commit, archive and contained OCI digest before deploying by digest.
Unsigned local metadata or a successful PR build does not prove signed provenance.
A promote job then makes those checks itself, again without source, and pushes
the image to the `agent-tools` registry tagged with its commit, digest unchanged.
Google accepts only this workflow on a `vitalis/main` push, through the trust and
push-only account in quality's `infra/agent-broker/promoter.tf`; set that root's
`octo_sts_release_variables` output as this repository's Actions variables, and
until then the job is skipped. Use repository variables (Settings → Secrets and
variables → Actions → Variables), not secrets or environment variables: the job
reads `vars` and has no environment. Re-running an earlier run does not pick up
newly set variables; the next push to `vitalis/main` does. It never deploys: the
broker runs a digest only after a reviewed apply. Repository policy must permit the workflow's SHA-pinned
actions, including actions/attest, before native use.

Upstream container publishing and stale mutations are guarded to octo-sts/app;
reusable deployment was already upstream-only. Other inherited main-only workflows
may need separate enablement or replacement by the maintainers.
