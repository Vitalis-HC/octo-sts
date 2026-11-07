#!/usr/bin/env bash
# Build local OCI only. No daemon, registry credentials, push or signing.
set -euo pipefail
[[ $(go version) == 'go version go1.27.1 linux/amd64' ]]
[[ $(ko version) == '0.19.1' ]]
[[ -z $(git status --porcelain) ]]
out=$(realpath -m "$1")
[[ ! -e "$out" ]]
mkdir -p "$out/source2" "$out/sbom"
commit=$(git rev-parse HEAD)
export SOURCE_DATE_EPOCH
SOURCE_DATE_EPOCH=$(git show -s --format=%ct HEAD)
export KO_DOCKER_REPO=example.invalid/octo-sts
export GOTOOLCHAIN=local CGO_ENABLED=0
unset GOOS GOARCH KO_DEFAULTPLATFORMS # --platform is the single platform selector.
# Do not read a user's registry credentials, even for the public base pull.
export DOCKER_CONFIG="$out/empty-docker-config"
mkdir -p "$DOCKER_CONFIG"
git archive HEAD | tar -x -C "$out/source2"
ko build --bare --platform=linux/amd64 --push=false --oci-layout-path="$out/image" --image-refs="$out/one.refs" --sbom-dir="$out/sbom" ./cmd/app
(
  cd "$out/source2"
  export GIT_CEILING_DIRECTORIES="$out" # Do not discover an unrelated parent checkout.
  ko build --bare --platform=linux/amd64 --push=false --oci-layout-path="$out/rebuild" --image-refs="$out/two.refs" ./cmd/app
)
python3 - "$out" "$commit" <<'PY'
import json, pathlib, re, sys
out, commit = pathlib.Path(sys.argv[1]), sys.argv[2]
refs = [(out / name).read_text().strip() for name in ('one.refs', 'two.refs')]
digests = [ref.rsplit('@', 1)[-1] for ref in refs]
assert digests[0] == digests[1] and re.fullmatch(r'sha256:[0-9a-f]{64}', digests[0])
digest = digests[0]
index = json.loads((out / 'image/index.json').read_text())
assert digest in [m['digest'] for m in index['manifests']]
proof = dict(commit=commit, platform='linux/amd64', image_digest=digest,
             go='1.27.1', ko='0.19.1', independent_source_archive_rebuild=True)
(out / 'provenance.json').write_text(json.dumps(proof, sort_keys=True) + '\n')
print(digest)
PY
tar --sort=name --mtime="@$SOURCE_DATE_EPOCH" --owner=0 --group=0 --numeric-owner -cf "$out/image.tar" -C "$out" image sbom provenance.json
