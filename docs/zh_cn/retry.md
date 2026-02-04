# 重试策略

重试由 `[gateway.retry]` 控制：
1. `enabled`：总开关
2. `enable_post`：是否允许 POST 重试
3. `max_retries`：最大重试次数
4. `retry_on_5xx`：是否重试 5xx
5. `retry_on_error`：是否重试网络错误
6. `retry_on_timeout`：是否重试超时

Trigger 脚本可通过返回 `retry = True` 强制重试。
