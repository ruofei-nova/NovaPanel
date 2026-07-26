# NovaPanel multi-tenant network map

NovaPanel supports a central control panel with two account roles:

- `admin` can manage every customer and VPS.
- `customer` is restricted by the server to the read-only dashboard and sees
  only nodes whose `owner_user_id` matches the authenticated account.

The browser never receives raw client IP addresses. The server resolves recent
China source IPs with a local MMDB city database, rounds the result to 0.1
degrees, aggregates identical locations, and returns only coordinates, counts,
and last-seen timestamps.

## Deployment topology

Install one central NovaPanel for the administrator, then install NovaPanel on
each overseas VPS as a remote node. Keep every remote panel's administrator
credential and API token private. Customer credentials are created on the
central panel under **Customer accounts**.

On the central panel:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/ruofei-nova/NovaPanel/main/install.sh)
bash <(curl -Ls https://raw.githubusercontent.com/ruofei-nova/NovaPanel/main/deploy/install-geoip.sh)
systemctl restart x-ui
```

The installer prints a random administrator username, password, port and web
base path. It also writes them with mode `0600` to:

```text
/etc/x-ui/install-result.env
```

The GeoIP installer downloads DB-IP City Lite. DB-IP Lite is licensed under
CC BY 4.0 and requires the attribution link included in the dashboard.

## Customer provisioning

1. Sign in to the central panel as administrator.
2. Open **Customer accounts** and select **Create customer**.
3. Leave the password empty to generate a cryptographically random password.
   The plain password is shown once; save it in a password manager.
4. Open **Nodes**, add or edit the customer's VPS, select the customer under
   **Assigned customer**, and enter the VPS datacenter latitude/longitude.
5. Give the customer the central panel URL, username, and one-time password.

Disabling a customer immediately invalidates existing sessions. Resetting the
password also invalidates all of that customer's existing sessions.

## Location semantics

Connection locations are based on the public source IP observed by Xray, not
GPS. Only China (`CN`) results seen during the last 15 minutes are drawn.
Carrier NAT, mobile gateways, VPNs, and incomplete Lite database coverage may
place a connection in a nearby city or province.
