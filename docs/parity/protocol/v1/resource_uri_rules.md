# Resource URI Rules

## Primary Scheme
`
amitia://
`

## Supported Paths
| Path | Description | Example |
|------|-------------|---------|
| workspace | Workspace resources | amitia://workspace/projects/my-project |
| attachment | Attachment resources | amitia://attachment/images/photo.png |
| browser | Browser resources | amitia://browser/tabs/current |
| offload | Offloaded tasks | amitia://offloads/task-123 |
| memory | Memory resources | amitia://memory/episodes/recent |
| runtime | Runtime resources | amitia://runtime/sessions/active |
| extension | Extension resources | amitia://extension/skills/my-skill |
| temp | Temporary resources | amitia://temp/cache/tmp-file |

## URI Format
`
amitia://<path>/<resource-type>/<identifier>
`

## Examples
`
amitia://workspace/projects/my-agent-project
amitia://attachment/files/document.pdf
amitia://memory/characters/main-character
amitia://browser/screenshots/capture-001
amitia://offloads/analysis-task-456
`

## Query Parameters
- ?format=json - Response format
- ?version=1 - Resource version
- ?access=read - Access level
