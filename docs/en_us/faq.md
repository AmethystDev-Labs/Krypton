# Troubleshooting (FAQ)

1. **`no upstream available`**
   - Node list is empty or all nodes are considered unhealthy.
   - Ensure `[[nodes]]` are reachable and health checks pass.

2. **Health check keeps timing out**
   - Increase `gateway.health_check_default.timeout`.
   - Ensure the health endpoint exists on upstream.

3. **`http2: timeout awaiting response headers`**
   - Increase `response_header_timeout` (not `upstream_timeout`).

4. **Streaming responses truncated**
   - Trigger only buffers small non-streaming responses.
   - Streaming (`text/event-stream`) is not buffered.

5. **Admin API returns 403**
   - `admin_api_enabled = true` requires `admin_api_token` to be set.

6. **Script error: `config.get ... default`**
   - Use `config.get("path", "fallback")` in new versions.
   - Update to latest build if you still see type errors.
