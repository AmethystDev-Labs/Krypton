# 日志

格式：

```
[LEVEL] (YYYY-MM-DD HH:MM:SS) 内容
```

等级：
1. INFO：请求日志、健康检查成功
2. DEBUG：INFO + 响应体预览
3. WARN：超时、重试错误
4. ERROR：致命错误

设置级别：

```powershell
$env:KRYPTON_LOG_LEVEL="debug"
```
