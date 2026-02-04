# 管理 API

管理接口路径为 `/.krypton/`，默认关闭。

启用方式：
1. `admin_api_enabled = true`
2. `admin_api_token = "strong-token"`

请求必须带：

```
X-Krypton-Token: strong-token
```

端点：
1. `GET /.krypton/health`
2. `POST /.krypton/reload/config`
3. `POST /.krypton/reload/scripts`

示例：

```bash
curl -s http://127.0.0.1:8080/.krypton/health \
  -H "X-Krypton-Token: strong-token"
```
