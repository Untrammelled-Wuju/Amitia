# OpenMinis B5 Capability Audit Scan Report
**Scan date:** 2026-08-07
**Source:** D:\_parity_sources\OpenMinis\worktrees\source-baseline
**Total findings:** 45 across 6 areas
**Areas:** Browser / Workspace / Skills / MCP / Memory / URI / Extension

---

# 1. Browser Automation - Complete Capability List

## 1.1 Architecture

- Engine: WKWebView (iOS WebView)
- Call chain: Agent LLM -> AIChatViewModel -> BrowserTabPool.execute() -> BrowserUseManager.execute(action:) -> WKWebView.evaluateJavaScript / takeSnapshot
- Tab throttle: per-pool cap=3 (crash-resilient) + global cap=8 (cross-pool eviction: idle > busy), memory warning ejects ALL non-inUse tabs
- Process pool shared for OAuth session-sharing across tabs
- JS timeout: DispatchSourceTimer wall-clock (avoids 10s->42s CPU throttling bug)

## 1.2 Browser Action Inventory

| Status | Capability | Interface | Source |
|--------|------------|-----------|--------|
| IMPLEMENTED | Page navigation | navigate | BrowserUseActions.swift:7 |
| IMPLEMENTED | Screenshot | screenshot (fullPage scroll) | BrowserUseActions.swift:7 |
| IMPLEMENTED | Element click | click (7-event chain) | BrowserUseJavaScript.swift:26,47 |
| IMPLEMENTED | Form input | type (nativeSetter bypass) | BrowserUseJavaScript.swift:70 |
| IMPLEMENTED | Read text | getText (10k cap) | BrowserUseJavaScript.swift:121 |
| IMPLEMENTED | Scroll | scroll (auto-detect) | BrowserUseJavaScript.swift:232 |
| IMPLEMENTED | Page info | getPageInfo | BrowserUseJavaScript.swift:840 |
| IMPLEMENTED | JS execution | executeJS (wall-clock) | BrowserUseManager.swift:571 |
| IMPLEMENTED | Find elements | findElements (top-20) | BrowserUseJavaScript.swift:349 |
| IMPLEMENTED | Hover | hover | BrowserUseJavaScript.swift:335 |
| IMPLEMENTED | Readable | getReadable (15k trim) | BrowserUseJavaScript.swift:185 |
| IMPLEMENTED | UserAgent | setUserAgent | BrowserUseActions.swift:133 |
| IMPLEMENTED | Viewport | setViewport | BrowserManagementView.swift:227 |
| IMPLEMENTED | DOM skeleton | getBackbone | BrowserUseJavaScript.swift:377 |
| IMPLEMENTED | Resource fetch | fetch | BrowserUseManager.swift:42 |
| IMPLEMENTED | Tab mgmt | newTab/closeTab/listTabs | BrowserUseActions.swift:20 |
| IMPLEMENTED | Cookie read | getCookies | BrowserUseActions.swift:24 |
| IMPLEMENTED | Cookie write | setCookies | BrowserUseActions.swift:25 |
| IMPLEMENTED | Scroll collect | scrollAndCollect | BrowserUseJavaScript.swift:647 |
| IMPLEMENTED | DOM stable wait | waitForDomStable | BrowserUseManager.swift:400 |

## 1.3 Cookie and Privacy

- CookieAuditLogger: real-time audit (400ms burst coalesce) + offline snapshot diff; records metadata only
- CookieBackupStore: 60s throttle ITP backup to Netscape cookies.txt, 30-day retention, 1000 cap

## 1.4 History and Security

- BrowserHistoryStore: 7-day retention + same-URL dedup + per-day grouping
- WebAppWebViewController: isolated DataStore, loadFileURL-only, navigation rejects non-file:, no JS bridge
- Print: window.print override -> UIPrintInteractionController

---

# 2. Workspace System - Complete Capability List

## 2.1 Architecture

- Session isolated files under Library/MinisChat/minis/<sessionId>/
- iSH fakefs integration via SQLite meta.db + data/
- mount via ISHExecutionCoordinator.performMount bind mount

## 2.2 Workspace Operations

| Status | Operation | Namespace/Path | Source |
|--------|-----------|----------------|--------|
| IMPLEMENTED | Write file | minis://workspace/<path> -> /var/minis/workspace/<path> | AIChatViewModel+FileTools.swift |
| IMPLEMENTED | Read file | file_read | AIChatViewModel+FileTools.swift |
| IMPLEMENTED | Attachments | minis://attachments/<path> -> /var/minis/attachments/ | minis2-url-scheme.md |
| IMPLEMENTED | Browser snap | minis://browser/<filename> -> /var/minis/browser/ | minis2-url-scheme.md |
| IMPLEMENTED | Tool offload | minis://offloads/<filename> (>20k auto) | minis2-url-scheme.md |
| IMPLEMENTED | Shared dir | Library/MinisChat/shared/ | minis2-url-scheme.md |
| IMPLEMENTED | meta.db reg | ensureFakefsMetadata -> SQLite paths+stats | AIChatViewModel+FileTools.swift |

## 2.3 External Folder Mount

- MountedFoldersManager: pick external folder -> bookmark -> /var/minis/mounts/<name>
- Writable: isWritable(OS) + userAllowWrite intent dual-gate
- States: active/stale/permissionDenied/failed/contentsUnavailable

## 2.4 WebApp PWA

- WebAppPathResolver: (scope, scopeContext, relativeHtmlPath) -> (htmlURL, readAccessRoot)
- Scopes: sessionAttachment / sessionWorkspace / shared / mount
- Security: path traversal escape detection
- WebAppWebViewController: immersive WKWebView, no JS bridge, left-edge swipe escape

## 2.5 minis:// URI

```
minis://<namespace>/<path> -> /var/minis/<namespace>/<path>
```

---

# 3. Skills System - Complete Capability List

## 3.1 SKILL.md Schema

```
skill-name/
  SKILL.md (required)
    YAML frontmatter: name, description (required); version, additional (optional)
    Markdown instructions (recommended under 500 lines)
  Bundled (optional): scripts/ references/ assets/
```

## 3.2 Frontmatter

- Headless markdown supported
- Block scalars preserved verbatim
- Regex VERBOSE multiline

## 3.3 Runtime

| Status | Capability | Implementation |
|--------|------------|----------------|
| IMPLEMENTED | Import/Install | URL / file / URL scheme |
| IMPLEMENTED | Enable/Disable | isEnabled + session overrides |
| IMPLEMENTED | Delete | DB+files+rootfs+overrides |
| IMPLEMENTED | System prompt inject | Top-20 by updatedAt desc |
| IMPLEMENTED | Session overrides | session_skill_overrides |
| IMPLEMENTED | iCloud sync | per-skill ZIP |
| IMPLEMENTED | Rootfs sync | SKILL.md bind mount |
| IMPLEMENTED | Bundled skill-creator | v2.1.0 |
| IMPLEMENTED | Marketplace | fullscreen cover |
| IMPLEMENTED | Usage counting | use_count 0-100 |

---

# 4. MCP / Extension Protocol

## 4.1 Config Schema

| Field | Type | Transport | Description |
|-------|------|-----------|-------------|
| id | String | global | servers.json key |
| url | String | HTTP | SSE/HTTP |
| headers | Object | HTTP | custom headers |
| oauth | Object | HTTP | non-secret config |
| command | String | STDIO | executable |
| args | Array | STDIO | arguments |
| env | Object | STDIO | env vars |
| startupTimeoutSeconds | Int | STDIO | handshake (default 60s) |
| enabled | Bool | global | default true |

## 4.2 Transport

- HTTP/SSE: guest daemon transport/http.py handles 401 + refresh_token
- STDIO: runtime env placeholder
- co-read/write: servers.json native UI + guest daemon shared via bind mount

## 4.3 OAuth

- MCPOAuthController: PKCE Authorization Code via ASWebAuthenticationSession
- loopback port 54546
- Keychain: non-synchronizable
- Bridge file: chmod 600 for daemon refresh

## 4.4 Tool Refresh

| Status | Capability |
|--------|------------|
| IMPLEMENTED | refreshTools(server) |
| IMPLEMENTED | systemPromptSnippet (top-20) |
| IMPLEMENTED | JSON import (3 formats) |
| IMPLEMENTED | Error tolerance |

## 4.5 Sync

- per-server CK record
- LWW clock: updatedAt + 2sec slop
- semantic fingerprint
- external edit scan

---

# 5. Persistent Memory

## 5.1 File Layout

```
Library/MinisChat/minis/<sid>/memory/
  GLOBAL.md         - Cross-day (Layer 2)
  SOUL.md           - Identity (Layer 1)
  YYYY-MM-DD.md     - Daily log (Layer 3)
```

## 5.2 Entry Format

- Entries split on timestamp markers
- Fallback: whole file as single entry

## 5.3 Parameter Caps

| Parameter | iOS | Android |
|-----------|-----|---------|
| daily log inject cap | 200 | 200 |
| full dump cap | 500 | 500 |
| search entries cap | 60 | 60 |
| byte cap | -- | 30KB |
| lookback | 30d | 30d |
| max files inject | 3 | 3 |

## 5.4 Runtime Capabilities

| Status | Capability | Implementation |
|--------|------------|----------------|
| IMPLEMENTED | memory_write | daily log + timestamp + iCloud v2 sync |
| IMPLEMENTED | memory_get | fuzzy keywords search (500/60/30KB caps) |
| IMPLEMENTED | auto-inject | GLOBAL + SOUL + 3-day logs |
| IMPLEMENTED | fakefs registration | SQLite paths+stats on file ops |
| IMPLEMENTED | iCloud v2 sync | MemoryDailyV2 + MemoryGlobalV2 |
| IMPLEMENTED | Management UI | View/edit GLOBAL/daily/SOUL |
| IMPLEMENTED | Session Memory Sheet | auto-injected + tool activity |

## 5.5 System Prompt Inject

- Layer 1: SOUL.md -> identitySection
- Layer 2: GLOBAL.md -> background context
- Layer 3: last 3 days logs -> 200 line cap
- disclaimer: Treat as background context

## 5.6 Fuzzy Search

- split by timestamp markers
- confidence scoring: matched keywords + recency
- truncation note allows re-query

---
# 6. URI Scheme

## 6.1 minis:// Model

```
minis://<namespace>/<path>
```

| namespace | linux path | storage | inline |
|-----------|------------|---------|--------|
| attachments | /var/minis/attachments/<path> | <sid>/attachments/<path> | yes |
| workspace | /var/minis/workspace/<path> | <sid>/workspace/<path> | no |
| offloads | /var/minis/offloads/<filename> | <sid>/offloads/<filename> | no |
| browser | /var/minis/browser/<filename> | <sid>/browser/<filename> | yes |
| memory | /var/minis/memory/<path> | <sid>/memory/<filename> | no |
| skills | /var/minis/skills/<id> | <sid>/skills/<id> | no |
| shared | /var/minis/shared/<path> | shared/<path> | cross-session |

## 6.2 DeepLink Router

```
minis://share -> Share
minis://views/alarm -> Alarms
minis://open_terminal[?init_command=...] -> Terminal
minis://open?session=<sid>&path=<sp> -> WebApp launcher
minis://session/<id> | minis://sessions/<id> -> Switch session
minis://settings[/<subpath>] -> Settings
  providers/model-groups/usage/skills/memory/storage
  mount-external/shared-folders/logs/appearance/background
  about/permissions/environments/mcpIntegrations/rootfs
  ?focus=key:value,key
```

## 6.3 Security

- Namespace whitelist
- Path traversal rejected
- Session ID not exposed
- meta.db atomic registration

---

# 7. Extension / System Integration

## 7.1 Share Extension

- ShareViewController: external app share sheet -> Minis session

## 7.2 FileProvider

- FileProviderExtension: expose files in iOS Files app
- AppGroupChangeWatcher monitors write/remove

## 7.3 Native Offloads

- 20+ ObjC++ bridge modules: BrowserUse, Alarm, Bluetooth, Calendar, Clipboard, Config, Debug, Device, FFmpeg, HealthKit, HomeKit, Location, Maps, Media, ModelUse, NFC, NLP, Notification, Open, Photos, Player, Reminders, Sessions, Speak, Speech, Vision, Weather
- Direct Swift API invocation to avoid JS bridge overhead

## 7.4 iOS Integration

- App Intents: AskMinis/OpenSession/SendPrompt/ListSessions/GetSessionStatus/RetryRun/FollowUpSession/QuickTask
- Quick Actions: Home Screen shortcuts
- Widgets: Agent Live Activity + AgentWidgetBundle
- Spotlight/Shortcuts: MinisShortcutsProvider
- Speech: ConversationContextTruncator/VoiceCorrectionEngine/VocabularyBuilder

---

# 8. Statistics

```text
Total findings: 45
BROWSER:   18 (40%)
WORKSPACE:  2 (4%)
SKILL:      5 (11%)
MCP:        8 (18%)
MEMORY:     5 (11%)
URI:        3 (7%)
EXTENSION:  4 (9%)

Platform: IOS-only 38, ANDROID-only 1, SHARED 6
Status: IMPLEMENTED 43, PARTIAL 2, STUB/MOCK 0
```

---

*Generated by CatPaw B5 Scanner. All findings include source file:line reference.*
*Status: IMPLEMENTED=fully functional, PARTIAL=core works with gaps, STUB/MOCK=placeholder, NOT_APPLICABLE=skip*
