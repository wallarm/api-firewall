#!/usr/bin/env bash
set -Eeuo pipefail

echo $BASH_SOURCE

cd "$(dirname "$(readlink -f "$BASH_SOURCE")")"

versions=( "$@" )
if [ ${#versions[@]} -eq 0 ]; then
	versions=( */ )
fi
versions=( "${versions[@]%/}" )

allVersions="$(
	git ls-remote --tags https://github.com/wallarm/api-firewall.git \
		| cut -d$'\t' -f2 \
		| grep -E '^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+' \
		| cut -dv -f2 \
		| sort -rV
)"

for version in "${versions[@]}"; do
	srcDownloadSha256="$(
		curl -fsSL "https://github.com/wallarm/api-firewall/releases/download/v${version}/api-firewall_linux-amd64" \
			| sha256sum \
			| cut -d' ' -f1
	)"
	echo "  - v${version}: $srcDownloadSha256"

	sed -ri \
		-e 's/^(ENV\s+APIFIREWALL_VERSION\s+).*/\1'"$version"'/' \
		-e 's/^(ENV\s+APIFIREWALL_URL\s+).*/\1'"https://github.com/wallarm/api-firewall/releases/download/v${version}/api-firewall_linux-amd64"'/' \
		-e 's/^(ENV\s+APIFIREWALL_SHA256\s+).*/\1'"$srcDownloadSha256"'/' \
		"$version/Dockerfile"
done