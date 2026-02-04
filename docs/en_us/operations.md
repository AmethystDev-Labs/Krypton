# Operations Guide

**Startup**
1. Copy template to `config.toml`.
2. Fill node addresses and tokens.
3. Start gateway.

**Safe rollout**
1. Start with a single upstream node.
2. Verify health checks and trigger behavior.
3. Add remaining nodes.

**Reload config**
1. Edit `config.toml`.
2. Call `/.krypton/reload/config`.

**Rotate tokens**
1. Update `config.toml`.
2. Reload config.
3. Restart if admin API is disabled.

**Monitoring**
1. INFO logs show request routing.
2. WARN logs show retry reasons and timeouts.
3. DEBUG logs show response preview (trimmed).

**Scale hints**
1. Increase `shards` for higher contention.
2. Tune `max_idle_conns_per_host` for concurrency.
3. Raise `response_header_timeout` for slow upstreams.
