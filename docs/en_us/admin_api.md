# Admin API

Admin endpoints live under `/.krypton/` and are disabled by default.

Configuration:
1. `admin_api_enabled = true`
2. `admin_api_token = "strong-token"`

Requests must include:

```
X-Krypton-Token: strong-token
```

Endpoints:
1. `GET /.krypton/health`
2. `POST /.krypton/reload/config`
3. `POST /.krypton/reload/scripts`

Examples:

```bash
curl -s http://127.0.0.1:8080/.krypton/health \
  -H "X-Krypton-Token: strong-token"
```
