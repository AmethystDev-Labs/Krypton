# OpenAI-compatible API health check.
# - Calls /v1/chat/completions with a tiny payload
# - Parses JSON
# - Verifies key fields are not None/False

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

    # NOTE: requires http.post_json builtin. If not available, use http.get on /v1/models instead.
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
        return {
            "score": 10,
            "status": "unhealthy",
            "labels": {"phase": "http", "reason": "non-200"},
            "message": "non-200 from upstream",
        }

    data = res["json"]
    if not data:
        return {
            "score": 20,
            "status": "degraded",
            "labels": {"phase": "json", "reason": "empty"},
            "message": "json missing",
        }

    # Validate critical fields (None/False are considered unhealthy)
    choices = data.get("choices", None)
    if choices == None or choices == False:
        return {
            "score": 30,
            "status": "degraded",
            "labels": {"phase": "content", "reason": "choices-none-or-false"},
            "message": "choices is None/False",
        }

    # Common OpenAI-compatible field
    first = choices[0] if len(choices) > 0 else None
    if first == None or first == False:
        return {
            "score": 30,
            "status": "degraded",
            "labels": {"phase": "content", "reason": "first-choice-none-or-false"},
            "message": "first choice is None/False",
        }

    # Empty assistant reply should be penalized
    msg = first.get("message", None) if first else None
    role = msg.get("role", None) if msg else None
    content = msg.get("content", None) if msg else None
    if role == "assistant" and (content == "" or content == None):
        return {
            "score": 40,
            "status": "degraded",
            "labels": {"phase": "content", "reason": "empty-assistant"},
            "message": "assistant reply empty",
        }

    return {
        "score": 100,
        "status": "healthy",
        "labels": {"phase": "ok"},
        "message": "openai compat ok",
    }
