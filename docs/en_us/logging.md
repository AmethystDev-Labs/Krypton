# Logging

Format:

```
[LEVEL] (YYYY-MM-DD HH:MM:SS) Content
```

Levels:
1. INFO: request logs, health ok
2. DEBUG: INFO plus response body preview
3. WARN: timeout, retryable errors
4. ERROR: fatal errors

Set level:

```powershell
$env:KRYPTON_LOG_LEVEL="debug"
```
