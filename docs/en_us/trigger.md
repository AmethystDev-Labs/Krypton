# Trigger Script (Post-Response)

Trigger scripts run after a request completes (not for active health checks). They can adjust weights and request retries.

Supported output keys:
1. `score`: set passive score directly
2. `penalty`: subtract from passive score
3. `reward`: add to passive score
4. `retry`: request another node retry
5. `message`: log message

Example `scripts/trigger.star`:

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
