#!/usr/bin/env bash
# Regenerate the Gerrit Go SDK from the OpenAPI document.
#
#   openapi-generator (go) -> drop non-shipped scaffolding
#
# No generated-code patching: the one Gerrit-specific concern (the )]}' XSSI guard)
# is handled by the hand-written transport in gerritxssi/, not by editing output. The
# go generator also maps the case-colliding query params O (scalar) / o (array) to
# distinct struct fields on its own, so unlike the Rust SDK there is no query patch.
#
# Usage: ./generate.sh [path-or-url]   (default: ./rest-api-openapi.json)
set -euo pipefail
cd "$(dirname "$0")"
SPEC="${1:-rest-api-openapi.json}"

# Reuse the spec straight from a running Gerrit: pass a URL (e.g. a plugin's served
# document) and it is fetched into the checked-in snapshot before generation.
if [[ "$SPEC" == http://* || "$SPEC" == https://* ]]; then
  echo "0/2 fetch spec from $SPEC"
  curl -fsSL "$SPEC" -o rest-api-openapi.json
  SPEC=rest-api-openapi.json
fi

VERSION=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["info"]["version"])' "$SPEC")

echo "1/2 generate go client (Gerrit $VERSION)"
rm -rf gerritclient
# enumClassPrefix=true prefixes enum constants with their type name; without it the go
# generator emits bare names (OK, NONE, ALL, ...) that collide at package scope.
# --parameter-name-mappings renames a colliding Go identifier while keeping the wire
# name: the plugin-list regex query param is named "r" (Gerrit's @Option(name="-r")),
# but the go generator uses "r" as its request-builder receiver, so `func (r ...) R(r
# string)` shadows the receiver and will not compile. Map it to a descriptive Go name;
# the query string sent is still ?r=.
npx --yes @openapitools/openapi-generator-cli@2.41.0 generate \
  -g go -i "$SPEC" -o gerritclient \
  --parameter-name-mappings r=regexFilter \
  --additional-properties=packageName=gerritclient,withGoMod=false,isGoSubmodule=true,enumClassPrefix=true \
  >/dev/null

echo "2/2 drop non-shipped generator scaffolding"
# test/docs stubs carry placeholder GIT_USER_ID/GIT_REPO_ID imports; the rest is noise.
(cd gerritclient && rm -rf test docs api .travis.yml git_push.sh .openapi-generator-ignore README.md .gitignore)

# README is protected by .openapi-generator-ignore, so the generator never rewrites it.
# Keep the Gerrit-version tokens in sync with the spec's info.version ($VERSION, derived
# above) so a version bump can't silently rot the README (the 3.15->3.16 class of drift).
# The substitutions are deliberately anchored to the Gerrit-version context so they only
# ever touch the two Gerrit-version token shapes and nothing else:
#   - "Gerrit X.Y.Z[-SNAPSHOT]"  (the prose generation line)
#   - "vX.Y.Z[-SNAPSHOT]"        (the mirrored git tag and the @version in go run/get)
# Requiring the full dotted X.Y.Z form means the bare "/v3/" and "/v3@" module-path
# majors are never matched, and unrelated versions (OkHttp, pydantic, generator-cli,
# "OpenAPI 3.1", Go toolchain) carry no "Gerrit "/"v" anchor + dotted-triplet shape.
sed -E -i.bak \
  -e "s/Gerrit [0-9]+\.[0-9]+\.[0-9]+(-SNAPSHOT)?/Gerrit ${VERSION}/g" \
  -e "s/v[0-9]+\.[0-9]+\.[0-9]+(-SNAPSHOT)?/v${VERSION}/g" \
  README.md
rm -f README.md.bak

# Keep the surface counts honest too: the operation count from the spec (generator-agnostic)
# and the model-type count from the generated package. Anchored to the "**N operations**" and
# "**N model types**" tokens so only those two numbers move -- the surface analogue of the
# version-token sync above, so a REST-surface change (the 338->339 ref-storage class of drift)
# cannot silently rot the README.
OPS=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(sum(1 for p in d["paths"].values() for m in p if m.lower() in ("get","post","put","delete","patch")))' "$SPEC")
MODELS=$(ls gerritclient/model_*.go 2>/dev/null | wc -l | tr -d ' ')
sed -E -i.bak \
  -e "s#\*\*[0-9]+ operations\*\*#**${OPS} operations**#" \
  -e "s#\*\*[0-9]+ model types\*\*#**${MODELS} model types**#" \
  README.md
rm -f README.md.bak

echo "done: gerritclient/ regenerated from $SPEC"
