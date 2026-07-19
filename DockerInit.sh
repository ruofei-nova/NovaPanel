#!/bin/sh
set -eu

case "${1:-}" in
    amd64)
        ARCH="64"
        XRAY_FNAME="amd64"
        MTG_ARCH="amd64"
        MTG_FNAME="amd64"
        ;;
    386 | i386)
        ARCH="32"
        XRAY_FNAME="386"
        MTG_ARCH="386"
        MTG_FNAME="386"
        ;;
    armv8 | arm64 | aarch64)
        ARCH="arm64-v8a"
        XRAY_FNAME="arm64"
        MTG_ARCH="arm64"
        MTG_FNAME="arm64"
        ;;
    arm)
        case "${2:-v7}" in
            v6 | 6)
                ARCH="arm32-v6"
                MTG_ARCH="armv6"
                ;;
            v7 | 7 | "")
                ARCH="arm32-v7a"
                MTG_ARCH="armv7"
                ;;
            *)
                echo "DockerInit: unsupported ARM variant: ${2:-}" >&2
                exit 1
                ;;
        esac
        XRAY_FNAME="arm32"
        MTG_FNAME="arm"
        ;;
    armv7 | arm32)
        ARCH="arm32-v7a"
        XRAY_FNAME="arm32"
        MTG_ARCH="armv7"
        MTG_FNAME="arm"
        ;;
    armv6)
        ARCH="arm32-v6"
        XRAY_FNAME="arm32"
        MTG_ARCH="armv6"
        MTG_FNAME="arm"
        ;;
    armv5)
        ARCH="arm32-v5"
        XRAY_FNAME="arm32"
        MTG_ARCH="armv5"
        MTG_FNAME="arm"
        ;;
    s390x)
        ARCH="s390x"
        XRAY_FNAME="s390x"
        MTG_ARCH="s390x"
        MTG_FNAME="s390x"
        ;;
    *)
        echo "DockerInit: unsupported architecture: ${1:-<empty>}" >&2
        exit 1
        ;;
esac
MTG_MULTI_VER=$(curl -sfL "https://api.github.com/repos/mhsanaei/mtg-multi/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
if [ -z "$MTG_MULTI_VER" ]; then
    echo "DockerInit: could not resolve the latest mtg-multi release tag" >&2
    exit 1
fi
mkdir -p build/bin
cd build/bin
curl -sfLRO "https://github.com/XTLS/Xray-core/releases/download/v26.7.11/Xray-linux-${ARCH}.zip"
unzip "Xray-linux-${ARCH}.zip"
rm -f "Xray-linux-${ARCH}.zip" geoip.dat geosite.dat
mv xray "xray-linux-${XRAY_FNAME}"
# mtg-multi (MTProto sidecar) ships prebuilt release binaries for every target
# we package, so download and unpack the matching one instead of compiling.
MTG_PKG="mtg-multi-${MTG_MULTI_VER#v}-linux-${MTG_ARCH}"
curl -sfLRO "https://github.com/mhsanaei/mtg-multi/releases/download/${MTG_MULTI_VER}/${MTG_PKG}.tar.gz"
tar -xzf "${MTG_PKG}.tar.gz"
mv "${MTG_PKG}/mtg-multi" "mtg-linux-${MTG_FNAME}"
rm -rf "${MTG_PKG}" "${MTG_PKG}.tar.gz"
chmod +x "mtg-linux-${MTG_FNAME}"
curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
curl -sfLRo geoip_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat
curl -sfLRo geosite_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat
curl -sfLRo geoip_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat
curl -sfLRo geosite_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat
cd ../../
