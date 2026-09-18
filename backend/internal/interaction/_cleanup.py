with open('internal/interaction/orchestrator.go', 'r', encoding='utf-8') as f:
    content = f.read()

old = '\tnewoutbox "github.com/u-ai/backend/internal/outbox"\n\t"github.com/u-ai/backend/internal/queue"'
new = '\t"github.com/u-ai/backend/internal/queue"'
content = content.replace(old, new)

with open('internal/interaction/orchestrator.go', 'w', encoding='utf-8') as f:
    f.write(content)
print('Done')
