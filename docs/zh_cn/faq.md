# 常见问题（FAQ）

1. **`no upstream available`**
   - 节点不可用或全部被判为不健康
   - 请确认 `[[nodes]]` 可访问，且健康检查通过

2. **健康检查总是超时**
   - 调大 `gateway.health_check_default.timeout`
   - 确认上游健康接口存在

3. **`http2: timeout awaiting response headers`**
   - 调大 `response_header_timeout`（不是 `upstream_timeout`）

4. **流式响应被截断**
   - Trigger 只处理非流式且体积较小的响应
   - `text/event-stream` 不会做 body 缓存

5. **管理 API 返回 403**
   - 必须设置 `admin_api_token`

6. **脚本报错 `config.get ... default`**
   - 使用 `config.get("path", "fallback")`
   - 更新到最新构建
