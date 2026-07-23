$path = 'D:\桌面\跟进项目\U-Ai\backend\internal\interaction\orchestrator.go'
$content = [System.IO.File]::ReadAllText($path)
# Fix the imports — add outbox import after "time"
$content = $content -replace '("time"\r?\n)', "`$1`r`n`r`n`t`"github.com/u-ai/backend/internal/outbox`""
# Replace Events types
$content = $content -replace '(\tEvents\s+)\[\]OutboxRecord', '${1}[]outbox.OutboxRecord'
# Replace outbox field type
$content = $content -replace '(\toutbox\s+)OutboxStore', '${1}*outbox.SQLiteOutboxStore'
[System.IO.File]::WriteAllText($path, $content, [System.Text.UTF8Encoding]::new($false))
Write-Host "Done"
