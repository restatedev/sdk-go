#!/usr/bin/env bash
#
# Tags a release across this multi-module repo. The main module is tagged when you
# pass an <sdk-version>; every submodule rides its own version line and is released
# only when you ask.
#
#   main module    github.com/restatedev/sdk-go                        ->  v1.2.3        (when given)
#   testing        github.com/restatedev/sdk-go/testing                ->  testing/v1.y.z
#   x/mocks        github.com/restatedev/sdk-go/x/mocks                ->  x/mocks/v0.y.z
#   x/tunnel       github.com/restatedev/sdk-go/x/tunnel               ->  x/tunnel/v0.y.z
#   x/protoc...    github.com/restatedev/sdk-go/x/protoc-gen-go-restate -> x/protoc-gen-go-restate/v0.y.z
#
# By convention `testing` tracks the stable 1.x line and the experimental `x/*`
# modules track 0.x (a v0 module may break between minors). The script doesn't
# enforce this — you pass each version explicitly. v0 and v1 modules share the
# same import path (no /v2 suffix), so a v0.x x/mocks requiring a v1.x SDK is fine.
#
# The <sdk-version> is optional: pass it to (re)tag the main module, or omit it to
# cut submodules on their own (e.g. bump only x/tunnel). A submodule is released
# only when you pass `<submodule>=<version>`.
#
# Pinning: submodules that import the main module (testing, x/mocks, x/tunnel) get
# their go.mod `require github.com/restatedev/sdk-go` pinned before tagging (to the
# <sdk-version> you gave, or — when omitted — to the latest released sdk-go tag),
# and the bump is committed (the committed `replace` keeps local dev building
# against the working tree; consumers ignore it and use the require).
# x/protoc-gen-go-restate carries no SDK dependency, so it is only tagged.
#
# Heads-up: x/mocks imports the SDK's internal/* packages. When a release changes
# those internals, cut a fresh x/mocks (bump its 0.x patch) re-pinned to the new
# SDK, e.g. `release.sh v1.1.0 x/mocks=v0.1.4`. x/tunnel depends only on the SDK's
# public API, so it is not subject to that internal-churn caveat.
#
# Usage:
#   .tools/release.sh [<sdk-version>] [<submodule>=<version> ...] [--push]
#
#   .tools/release.sh v1.0.0                                                              # main only
#   .tools/release.sh v1.0.0 testing=v1.0.0 x/mocks=v0.1.0 x/tunnel=v0.1.0 x/protoc-gen-go-restate=v0.1.0 # full first release
#   .tools/release.sh v1.0.1 x/mocks=v0.1.1                                               # SDK patch + re-cut x/mocks (pinned to v1.0.1)
#   .tools/release.sh x/tunnel=v0.1.0                                                     # cut only x/tunnel (pinned to the latest released sdk-go)
#   .tools/release.sh v1.0.0 --push                                                       # also push the commit + tags
#
# What it does:
#   1. checks the working tree is clean and the tags don't already exist
#   2. pins each released, SDK-dependent submodule's go.mod to the pin version, commits
#   3. tags the main module (if an sdk-version was given) and the submodules being released
#   4. prints the push command (nothing is pushed unless you pass --push)
set -euo pipefail
cd "$(dirname "$0")/.."

# Submodules, each on its own version line; released only when you pass
# <submodule>=<version>.
SUBMODULES=(testing x/mocks x/tunnel x/protoc-gen-go-restate)
# Of those, the ones that import the main module: pin their go.mod require to the SDK version.
PINNED_SUBMODULES=(testing x/mocks x/tunnel)

die() { echo "error: $*" >&2; exit 1; }
semver_re='^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'
mainver_re='^v[0-9]+\.[0-9]+\.[0-9]+$'
in_list() { local x="$1"; shift; printf '%s\n' "$@" | grep -qx "$x"; }

[ $# -ge 1 ] || die "usage: .tools/release.sh [<sdk-version>] [<submodule>=<version> ...] [--push]"

# The first arg is the SDK version only when it looks like one; otherwise the main
# module is left untouched and every arg is a submodule spec or flag.
SDK_VERSION=""
if [[ "$1" =~ $semver_re ]]; then
	SDK_VERSION="$1"; shift
fi

PUSH=0
declare -A RELEASE   # submodule -> version, populated from <submodule>=<version> args
while [ $# -gt 0 ]; do
	case "$1" in
		--push) PUSH=1 ;;
		*=v*)   m="${1%%=*}"; in_list "$m" "${SUBMODULES[@]}" || die "unknown submodule: $m"
		        RELEASE["$m"]="${1#*=}" ;;
		*)      die "unknown argument: $1 (an <sdk-version> must come first and look like vX.Y.Z)" ;;
	esac
	shift
done

[ -z "$(git status --porcelain)" ] || die "working tree is not clean; commit or stash first"

# The version submodule requires are pinned to: the given sdk-version, or — when
# releasing submodules on their own — the latest released sdk-go tag.
if [ -n "$SDK_VERSION" ]; then
	PIN_VERSION="$SDK_VERSION"
else
	[ "${#RELEASE[@]}" -gt 0 ] || die "nothing to release: pass an <sdk-version> and/or <submodule>=<version>"
	PIN_VERSION="$(git tag -l 'v[0-9]*' | grep -E "$mainver_re" | sort -V | tail -n1)"
	[ -n "$PIN_VERSION" ] || die "no released sdk-go tag to pin against; pass an explicit <sdk-version>"
	echo "no sdk-version given; releasing submodules only, pinned to the latest sdk-go $PIN_VERSION"
fi

# Build the tag list (main, if given, + each requested submodule) and validate versions.
TAGS=()
[ -n "$SDK_VERSION" ] && TAGS+=("$SDK_VERSION")
for m in "${!RELEASE[@]}"; do
	v="${RELEASE[$m]}"
	[[ "$v" =~ $semver_re ]] || die "bad version '$v' for $m"
	TAGS+=("$m/$v")
done
[ "${#TAGS[@]}" -gt 0 ] || die "nothing to release"
for t in "${TAGS[@]}"; do
	git rev-parse -q --verify "refs/tags/$t" >/dev/null && die "tag already exists: $t"
done

# Pin the released, SDK-dependent submodules to PIN_VERSION and commit the bump.
changed=0
for m in "${!RELEASE[@]}"; do
	in_list "$m" "${PINNED_SUBMODULES[@]}" || continue
	( cd "$m" && go mod edit -require="github.com/restatedev/sdk-go@$PIN_VERSION" )
	git diff --quiet -- "$m/go.mod" || { git add "$m/go.mod"; changed=1; }
done

# Re-tidy the non-published modules - the examples and test-services. They are never
# tagged and build against the working tree via `replace github.com/restatedev/sdk-go
# => ../`, so they carry no real SDK version of their own. A floor-free module keeps the
# zero pseudo-version (v0.0.0-00010101000000-000000000000) and tidy leaves it untouched;
# one that also pulls in a just-pinned submodule (e.g. ticketreservation depends on
# x/mocks) inherits that submodule's SDK require as its MVS floor, and tidy raises its
# require to match. Either way the go.mod stays tidy, so CI's readonly `go build`/`go vet`
# keep passing - with no version hardcoded here and no churn for the floor-free ones.
# Runs after the pin loop above so the floor is already in place; the published
# submodules carry the same replace but are handled there, so skip them here.
while IFS= read -r gomod; do
	grep -qE '^replace github\.com/restatedev/sdk-go +=>' "$gomod" || continue
	d="$(dirname "${gomod#./}")"
	in_list "$d" "${SUBMODULES[@]}" && continue
	( cd "$d" && go mod tidy )
	git diff --quiet -- "$d/go.mod" "$d/go.sum" || { git add "$d/go.mod" "$d/go.sum"; changed=1; }
done < <(find . -name go.mod)

if [ "$changed" -eq 1 ]; then
	git commit -m "release ${SDK_VERSION:-${TAGS[*]}}" >/dev/null
	echo "pinned submodules and re-tidied examples + test-services against sdk-go $PIN_VERSION, committed"
fi

for t in "${TAGS[@]}"; do git tag "$t"; echo "tagged $t"; done

# The proto contract is published separately on the BSR, out of band of git tags.
if [ -n "${RELEASE[x/protoc-gen-go-restate]:-}" ]; then
	echo
	echo "note: x/protoc-gen-go-restate also owns the BSR contract (buf.build/restatedev/sdk-go)."
	echo "      after pushing, publish it:  ( cd x/protoc-gen-go-restate && buf push )"
fi

BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$PUSH" -eq 1 ]; then
	git push origin "$BRANCH"
	git push origin "${TAGS[@]}"
	echo "pushed $BRANCH and tags: ${TAGS[*]}"
else
	echo
	echo "nothing pushed. when ready:"
	echo "  git push origin $BRANCH && git push origin ${TAGS[*]}"
fi
