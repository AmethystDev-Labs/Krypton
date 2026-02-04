# Trigger script for OpenAI-compatible responses.
# Applies penalty when assistant reply is empty.

def trigger(ctx):
    req = ctx["request"]
    resp = ctx["response"]

    # Only apply to chat completions
    if req.get("path", "") != "/v1/chat/completions":
        return None

    body = resp.get("body", "")
    if body == "" or body == None:
        log.warn("empty response body for chat completions")
        return {"penalty": 20, "retry": True, "message": "empty response body"}

    # Naive JSON check for empty assistant content
    if "\"role\":\"assistant\"" in body and "\"content\":\"\"" in body:
        log.warn("empty assistant reply detected")
        return {"penalty": 20, "retry": True, "message": "empty assistant reply"}

    return None
