#!/usr/bin/env bash
set -euo pipefail

target="${NOVAPANEL_GEOIP_DB:-/etc/x-ui/GeoLite2-City.mmdb}"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

mkdir -p "$(dirname "$target")"

download_month() {
  local month="$1"
  local url="https://download.db-ip.com/free/dbip-city-lite-${month}.mmdb.gz"
  echo "Downloading DB-IP City Lite ${month}..."
  curl --fail --location --silent --show-error "$url" | gzip -dc > "$tmp"
  test -s "$tmp"
}

current_month="$(date -u +%Y-%m)"
previous_month="$(date -u -d "$(date -u +%Y-%m-01) -1 month" +%Y-%m)"

if ! download_month "$current_month"; then
  rm -f "$tmp"
  tmp="$(mktemp)"
  download_month "$previous_month"
fi

install -m 0644 "$tmp" "$target"
echo "Installed GeoIP city database at $target"
echo "DB-IP Lite data is licensed under CC BY 4.0: https://db-ip.com"
