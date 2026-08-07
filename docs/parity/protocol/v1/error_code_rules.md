# Error Code Rules

## Format
`
ERR_<DOMAIN>_<CATEGORY>_<NUMBER>
`

## Components
- **DOMAIN**: Functional area (uppercase)
- **CATEGORY**: Error category (uppercase)
- **NUMBER**: 5-digit numeric code

## Domain List
| Domain | Code Range | Description |
|--------|------------|-------------|
| AGENT | 1xxx | Agent errors |
| MODEL | 2xxx | Model errors |
| TOOL | 3xxx | Tool errors |
| TASK | 4xxx | Task errors |
| RUNTIME | 5xxx | Runtime errors |
| PERMISSION | 6xxx | Permission errors |
| FILE | 7xxx | File errors |
| NETWORK | 8xxx | Network errors |
| BROWSER | 9xxx | Browser errors |
| WORKSPACE | 10xxx | Workspace errors |
| MCP | 11xxx | MCP errors |
| SKILL | 12xxx | Skill errors |
| EXTENSION | 13xxx | Extension errors |
| VOICE | 14xxx | Voice errors |
| MEMORY | 15xxx | Memory errors |
| CHANNEL | 16xxx | Channel errors |
| DATABASE | 17xxx | Database errors |
| PLATFORM | 18xxx | Platform errors |
| SECURITY | 19xxx | Security errors |
| INTERNAL | 99xxx | Internal errors |

## Category Examples
- INITIALIZATION
- CONNECTION
- TIMEOUT
- INVALID_ARGUMENT
- EXECUTION
- NOT_FOUND
- NOT_READY
- DENIED
- AUTH
- UNKNOWN
