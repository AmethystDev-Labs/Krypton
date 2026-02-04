# Complex health check demo: multi-endpoint, headers, JSON/text fallback,
# and graded scoring with labels.

def check(node_ctx):
    base = node_ctx["address"]

    # 1) Basic health probe with a custom header
    h = http.get(base + "/health", headers={"X-Check": "krypton"})
    if h["status"] != 200:
        return {
            "score": 15,
            "status": "unhealthy",
            "labels": {"phase": "basic", "reason": "status"},
            "message": "basic health failed",
        }

    # 2) Deep probe, prefer JSON but allow text fallback
    deep = http.get(base + "/health/deep")
    if deep["status"] != 200:
        return {
            "score": 40,
            "status": "degraded",
            "labels": {"phase": "deep", "reason": "status"},
            "message": "deep health non-200",
        }

    data = deep["json"]
    if not data:
        if "ok" in deep["text"]:
            return {
                "score": 70,
                "status": "degraded",
                "labels": {"phase": "deep", "reason": "text-fallback"},
                "message": "json missing; text ok",
            }
        return {
            "score": 30,
            "status": "degraded",
            "labels": {"phase": "deep", "reason": "no-json"},
            "message": "json missing",
        }

    # 3) Business metrics scoring
    db_conn = data.get("db_conn", 0)
    redis_ok = data.get("redis_ok", True)
    latency = data.get("latency_ms", 0)

    score = 100
    labels = {}

    if db_conn > 90:
        score -= 25
        labels["db"] = "hot"
    if not redis_ok:
        score -= 30
        labels["redis"] = "down"
    if latency > 200:
        score -= 20
        labels["latency"] = "high"

    if score >= 90:
        status = "healthy"
    elif score >= 60:
        status = "degraded"
    else:
        status = "unhealthy"

    return {
        "score": score,
        "status": status,
        "labels": labels,
        "message": "deep check ok",
    }
