# 主动健康检查（Starlark）

主动健康检查由 `[gateway.health_check_default]` 驱动。

内置模块：
1. `http.get(url, headers=...)`
2. `http.get_json(url, headers=...)`
3. `http.post_json(url, json=..., headers=...)`
4. `log.info(msg)`, `log.warn(msg)`, `log.error(msg)`
5. `config.get("key.path", default)`

脚本必须实现：

```python
def check(node_ctx):
    return {"score": 100, "status": "healthy"}
```

可选示例（仅用于 OpenAI 兼容上游）：

```python
def check(node_ctx):
    base = node_ctx["address"]
    model = config.get("gateway.openai_check_model", "deepseek-chat")
    key = config.get("gateway.openai_check_key", "")

    payload = {
        "model": model,
        "messages": [
            {"role": "system", "content": "ping"},
            {"role": "user", "content": "ping"},
        ],
        "max_tokens": 1,
        "temperature": 0,
    }

    res = http.post_json(
        base + "/v1/chat/completions",
        json=payload,
        headers={
            "Authorization": "Bearer " + key,
            "Content-Type": "application/json",
            "X-Check": "krypton",
        },
    )

    if res["status"] != 200:
        return {"score": 10, "status": "unhealthy", "message": "non-200"}

    data = res["json"]
    if not data:
        return {"score": 20, "status": "degraded", "message": "json missing"}

    return {"score": 100, "status": "healthy"}
```
