# 运维指南

**启动**
1. 复制模板到 `config.toml`
2. 填写节点地址与密钥
3. 启动服务

**安全上线**
1. 先只配置一个节点
2. 验证健康检查与触发器
3. 再逐步加入其他节点

**配置热更新**
1. 修改 `config.toml`
2. 调用 `/.krypton/reload/config`

**Token 轮换**
1. 更新 `config.toml`
2. Reload 配置
3. 若未启用管理 API，则重启

**监控要点**
1. INFO：请求路由
2. WARN：重试/超时
3. DEBUG：响应体预览

**扩展建议**
1. 增大 `shards` 降低锁竞争
2. 提高 `max_idle_conns_per_host`
3. 慢上游调大 `response_header_timeout`
