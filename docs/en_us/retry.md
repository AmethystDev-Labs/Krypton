# Retry Policy

Retry is controlled by `[gateway.retry]`:
1. `enabled`: master switch
2. `enable_post`: allow retry for POST
3. `max_retries`: number of extra attempts
4. `retry_on_5xx`: retry on 5xx
5. `retry_on_error`: retry on network errors
6. `retry_on_timeout`: retry on timeouts

Trigger scripts can force retry by returning `retry = True`.
