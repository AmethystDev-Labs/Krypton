# 触发器（Trigger）

触发器在请求完成后执行（不用于主动探测）。可以调节权重或触发重试。

支持返回字段：
1. `score`：直接设置被动分数
2. `penalty`：降低被动分数
3. `reward`：提高被动分数
4. `retry`：触发重试
5. `message`：日志消息

示例：

```python
def trigger(ctx):
    req = ctx["request"]
    resp = ctx["response"]

    if req.get("path", "") != "/v1/chat/completions":
        return None

    body = resp.get("body", "")
    if body == "" or body == None:
        log.warn("empty response body for chat completions")
        return {"penalty": 20, "retry": True, "message": "empty response body"}

    if "\"role\":\"assistant\"" in body and "\"content\":\"\"" in body:
        log.warn("empty assistant reply detected")
        return {"penalty": 20, "retry": True, "message": "empty assistant reply"}

    return None
```
