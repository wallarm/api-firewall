#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -eq 0 ]]; then
    echo "Current API-Firewall version not supplied"
    exit 1
fi

version="$1"

allVersions="$(
	git ls-remote --tags https://github.com/wallarm/api-firewall.git \
		| cut -d$'\t' -f2 \
		| grep -E '^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+' \
		| cut -dv -f2 \
		| sort -rV
)"


if [[ "${allVersions[*]}" =~ "${version}" ]]; then
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
                "Dockerfile"
else
   echo "Please pass correct API-Firewall release version in argument"
   exit 1
fi

