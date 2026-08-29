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

echo "done: gerritclient/ regenerated from $SPEC"
