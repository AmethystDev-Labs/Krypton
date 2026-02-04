# Configuration

Use `config.toml` for runtime configuration. It is ignored by git. Use `example.config.toml` as the template.

Key sections:
1. `[gateway]` runtime and transport settings
2. `[gateway.health_check_default]` active health check
3. `[gateway.retry]` retry policy
4. `[[nodes]]` upstreams

Minimal example:

```toml
[gateway]
listen = ":8080"
shards = 8
max_retries = 2
max_body_size = 1048576
retry_non_idempotent = false
admin_api_enabled = false
admin_api_token = "REPLACE_ME"
read_timeout = "15s"
write_timeout = "30s"
idle_timeout = "60s"
response_header_timeout = "60s"
idle_conn_timeout = "90s"
upstream_timeout = "60s"
max_idle_conns = 256
max_idle_conns_per_host = 64
max_conns_per_host = 0
trigger_script = "./scripts/trigger.star"
trigger_timeout = "2s"
trigger_body_limit = 4096
openai_check_key = "REPLACE_ME"
openai_check_model = "deepseek-chat"

[gateway.retry]
enabled = true
enable_post = false
max_retries = 2
retry_on_5xx = true
retry_on_error = true
retry_on_timeout = true

[gateway.health_check_default]
interval = "120s"
timeout = "60s"
script = "./scripts/openai_compat_check.star"

[[nodes]]
id = "srv-1"
address = "https://example-1.hf.space"
weight = 100
```
