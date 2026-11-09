# Reproducible fork image

The release line `vitalis/v0.14.2` starts at upstream commit
270e72e2ac2b1f6830c3d15987697d37bd5d65c4. It is separate from upstream main.
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
The workflow does not choose a registry or publish an image. Repository policy
must permit its SHA-pinned actions, including actions/attest, before native use.

Upstream container publishing and stale mutations are guarded to octo-sts/app;
reusable deployment was already upstream-only. Other inherited main-only workflows
may need separate enablement or replacement by the maintainers.
