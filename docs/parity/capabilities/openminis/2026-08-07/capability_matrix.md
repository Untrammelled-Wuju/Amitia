# OpenMinis Capability Matrix (B5)

**Baseline**: OMN-2026-08-07-CROSSPLATFORM
**Source Commit**: `9cf3a85`
**Android Tag**: 0.22-preview
**iOS Version**: 1.11
**Total Capabilities**: 145

| # | Capability ID | Name | Category | Scope | iOS | Android | State | Tool ID |
|---|---------------|------|----------|-------|-----|---------|-------|---------|
| 1 | OMN-0001 | AI Chat Conversation | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 2 | OMN-0002 | Tool Call Execution | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 3 | OMN-0003 | Multi-turn Task Planning | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 4 | OMN-0004 | Context Management | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 5 | OMN-0005 | Session Persistence | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 6 | OMN-0006 | Message Editing | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 7 | OMN-0007 | Cancellation & Stop | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 8 | OMN-0008 | Image/Multimodal Input | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | read_image |
| 9 | OMN-0009 | Fallback & Retry | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 10 | OMN-0010 | Concurrent Tool Execution | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 11 | OMN-00143 | Reasoning/Thinking Display | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 12 | OMN-00144 | Sub-model Spawn | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 13 | OMN-00145 | Tool Result Observation | 01_AGENT_AND_CHAT | SHARED | Y | Y | IMPLEMENTED | - |
| 14 | OMN-0011 | Tool Registry | 03_TOOL_RUNTIME | SHARED | Y | Y | IMPLEMENTED | shell_execute, file_read, file_write, file_edit, browser_use, memory_write, memory_get, read_image |
| 15 | OMN-0012 | Shell Command Execution | 05_SHELL_AND_PROCESS | SHARED | Y | Y | IMPLEMENTED | shell_execute |
| 16 | OMN-0013 | File Read | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | file_read |
| 17 | OMN-0014 | File Write | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | file_write |
| 18 | OMN-0015 | File Edit | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | file_edit |
| 19 | OMN-0016 | Filesystem Listing | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | - |
| 20 | OMN-0017 | Filesystem Search | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | - |
| 21 | OMN-0136 | File Download in Sandbox | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | - |
| 22 | OMN-0137 | File Upload from Sandbox | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | - |
| 23 | OMN-0018 | Linux Sandbox (iOS iSH) | 04_SANDBOX_RUNTIME | IOS_ONLY | Y |  | IMPLEMENTED | shell_execute |
| 24 | OMN-0019 | Linux Sandbox (Android PRoot) | 04_SANDBOX_RUNTIME | ANDROID_ONLY |  | Y | IMPLEMENTED | shell_execute |
| 25 | OMN-0020 | Sandbox Process Execution | 04_SANDBOX_RUNTIME | SHARED | Y | Y | IMPLEMENTED | shell_execute |
| 26 | OMN-0021 | Sandbox stdin/stdout/stderr | 04_SANDBOX_RUNTIME | SHARED | Y | Y | IMPLEMENTED | shell_execute |
| 27 | OMN-0022 | Sandbox Timeout & Cancellation | 04_SANDBOX_RUNTIME | SHARED | Y | Y | IMPLEMENTED | shell_execute |
| 28 | OMN-0023 | Sandbox Package Management | 04_SANDBOX_RUNTIME | SHARED | Y | Y | IMPLEMENTED | - |
| 29 | OMN-0024 | Sandbox File Mount | 04_SANDBOX_RUNTIME | SHARED | Y | Y | IMPLEMENTED | - |
| 30 | OMN-0025 | Sandbox DNS Configuration | 04_SANDBOX_RUNTIME | SHARED | Y | Y | IMPLEMENTED | - |
| 31 | OMN-0142 | Sandbox Crash Recovery | 04_SANDBOX_RUNTIME | SHARED | Y | Y | IMPLEMENTED | - |
| 32 | OMN-0026 | Browser Page Navigation | 08_BROWSER_AUTOMATION | SHARED | Y | Y | IMPLEMENTED | browser_use |
| 33 | OMN-0027 | Browser Content Extraction | 08_BROWSER_AUTOMATION | SHARED | Y | Y | IMPLEMENTED | browser_use |
| 34 | OMN-0028 | Browser Screenshot | 08_BROWSER_AUTOMATION | SHARED | Y | Y | IMPLEMENTED | browser_use |
| 35 | OMN-0029 | Browser Click Element | 08_BROWSER_AUTOMATION | SHARED | Y | Y | IMPLEMENTED | browser_use |
| 36 | OMN-0030 | Browser Form Input | 08_BROWSER_AUTOMATION | SHARED | Y | Y | IMPLEMENTED | browser_use |
| 37 | OMN-0031 | Browser Cookie Management | 08_BROWSER_AUTOMATION | SHARED | Y | Y | IMPLEMENTED | browser_use |
| 38 | OMN-0032 | Browser JavaScript Execution | 08_BROWSER_AUTOMATION | SHARED | Y | Y | IMPLEMENTED | browser_use |
| 39 | OMN-0033 | Browser Back/Forward/Refresh | 08_BROWSER_AUTOMATION | SHARED | Y | Y | IMPLEMENTED | browser_use |
| 40 | OMN-0034 | Browser Multi-tab Support | 08_BROWSER_AUTOMATION | SHARED | Y | Y | IMPLEMENTED | browser_use |
| 41 | OMN-0035 | Browser File Upload/Download | 08_BROWSER_AUTOMATION | SHARED | Y | Y | IMPLEMENTED | browser_use |
| 42 | OMN-0036 | Browser Scroll | 08_BROWSER_AUTOMATION | SHARED | Y | Y | IMPLEMENTED | browser_use |
| 43 | OMN-0037 | Skill Markdown Format (SKILL.md) | 09_SKILL | SHARED | Y | Y | IMPLEMENTED | - |
| 44 | OMN-0038 | Skill Installation/Import | 09_SKILL | SHARED | Y | Y | IMPLEMENTED | - |
| 45 | OMN-0039 | Skill Enable/Disable | 09_SKILL | SHARED | Y | Y | IMPLEMENTED | - |
| 46 | OMN-0040 | Skill Context Injection | 09_SKILL | SHARED | Y | Y | IMPLEMENTED | - |
| 47 | OMN-0041 | Skill Script Execution | 09_SKILL | SHARED | Y | Y | IMPLEMENTED | - |
| 48 | OMN-0140 | Skill Deletion | 09_SKILL | SHARED | Y | Y | IMPLEMENTED | - |
| 49 | OMN-0141 | Skill Built-in Set | 09_SKILL | SHARED | Y | Y | IMPLEMENTED | - |
| 50 | OMN-0042 | MCP Client Configuration | 10_MCP_AND_EXTENSION | SHARED | Y | Y | IMPLEMENTED | - |
| 51 | OMN-0043 | MCP STDIO Transport | 10_MCP_AND_EXTENSION | SHARED | Y | Y | IMPLEMENTED | - |
| 52 | OMN-0044 | MCP SSE Transport | 10_MCP_AND_EXTENSION | SHARED | Y | Y | IMPLEMENTED | - |
| 53 | OMN-0045 | MCP HTTP Transport | 10_MCP_AND_EXTENSION | SHARED | Y | Y | IMPLEMENTED | - |
| 54 | OMN-0046 | MCP Tool Discovery | 10_MCP_AND_EXTENSION | SHARED | Y | Y | IMPLEMENTED | - |
| 55 | OMN-0047 | MCP OAuth Support | 10_MCP_AND_EXTENSION | SHARED | Y | Y | IMPLEMENTED | - |
| 56 | OMN-0048 | Memory Write | 11_MEMORY | SHARED | Y | Y | IMPLEMENTED | memory_write |
| 57 | OMN-0049 | Memory Get/Retrieve | 11_MEMORY | SHARED | Y | Y | IMPLEMENTED | memory_get |
| 58 | OMN-0050 | Memory Fuzzy Search | 11_MEMORY | SHARED | Y | Y | IMPLEMENTED | memory_get |
| 59 | OMN-0051 | Memory Auto-summary | 11_MEMORY | SHARED | Y | Y | IMPLEMENTED | - |
| 60 | OMN-0052 | Cross-session Persistence | 11_MEMORY | SHARED | Y | Y | IMPLEMENTED | memory_write, memory_get |
| 61 | OMN-0138 | Memory Delete | 11_MEMORY | SHARED | Y | Y | IMPLEMENTED | - |
| 62 | OMN-0139 | Memory Edit/Update | 11_MEMORY | SHARED | Y | Y | IMPLEMENTED | - |
| 63 | OMN-0053 | Anthropic Provider | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 64 | OMN-0054 | OpenAI Provider | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 65 | OMN-0055 | Gemini Provider | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 66 | OMN-0056 | OpenRouter Provider | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 67 | OMN-0057 | Antigravity Provider | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 68 | OMN-0058 | Kimi/Moonshot Provider | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 69 | OMN-0059 | xAI Provider | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 70 | OMN-0060 | Custom Provider | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 71 | OMN-0061 | Model Tool Use | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 72 | OMN-0062 | Multimodal Model Input | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 73 | OMN-0063 | Streaming Output | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 74 | OMN-0064 | Thinking/Reasoning Mode | 12_MODEL_PROVIDER | SHARED | Y | Y | IMPLEMENTED | - |
| 75 | OMN-0065 | Text-to-Speech | 14_VOICE | SHARED_WITH_PLATFORM_ADAPTER | Y | Y | IMPLEMENTED | - |
| 76 | OMN-0066 | Speech-to-Text | 14_VOICE | SHARED_WITH_PLATFORM_ADAPTER | Y | Y | IMPLEMENTED | - |
| 77 | OMN-0067 | Voice Input in Chat | 14_VOICE | SHARED | Y | Y | IMPLEMENTED | - |
| 78 | OMN-0068 | TTS Voice Selection | 14_VOICE | SHARED | Y | Y | IMPLEMENTED | - |
| 79 | OMN-0083 | iOS Native TTS (AVSpeech) | 14_VOICE | IOS_ONLY | Y |  | IMPLEMENTED | apple-speak |
| 80 | OMN-0084 | iOS Speech Recognition | 14_VOICE | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | apple-speech |
| 81 | OMN-0069 | Read Health Data (HealthKit) | 15_IOS_HEALTH | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | apple-healthkit |
| 82 | OMN-0070 | Write Health Data (HealthKit) | 15_IOS_HEALTH | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | apple-healthkit |
| 83 | OMN-0071 | Calendar Event Create | 16_IOS_CALENDAR_REMINDERS | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | - |
| 84 | OMN-0072 | Reminder Create | 16_IOS_CALENDAR_REMINDERS | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | apple-reminders |
| 85 | OMN-0073 | Bluetooth Device Detection | 18_IOS_HOMEKIT_BLUETOOTH_NFC | IOS_ONLY | Y |  | IMPLEMENTED | apple-bluetooth |
| 86 | OMN-0074 | HomeKit Device Control | 18_IOS_HOMEKIT_BLUETOOTH_NFC | IOS_ONLY | Y |  | IMPLEMENTED | apple-homekit |
| 87 | OMN-0075 | NFC Tag Read | 18_IOS_HOMEKIT_BLUETOOTH_NFC | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | apple-nfc |
| 88 | OMN-0076 | Clipboard Read | 21_ANDROID_SYSTEM_SERVICE | IOS_ONLY | Y |  | IMPLEMENTED | apple-clipboard |
| 89 | OMN-0077 | Clipboard Write | 21_ANDROID_SYSTEM_SERVICE | IOS_ONLY | Y |  | IMPLEMENTED | apple-clipboard |
| 90 | OMN-0078 | Location Get | 17_IOS_CONTACTS_LOCATION_MAPS | IOS_ONLY | Y |  | IMPLEMENTED | apple-location |
| 91 | OMN-0079 | Weather Query | 29_OTHER | IOS_ONLY | Y |  | IMPLEMENTED | apple-weather |
| 92 | OMN-0080 | Media Playback Control | 19_IOS_PHOTOS_MEDIA_VISION | IOS_ONLY | Y |  | IMPLEMENTED | apple-player |
| 93 | OMN-0081 | Photo Library Access | 19_IOS_PHOTOS_MEDIA_VISION | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | apple-photos |
| 94 | OMN-0082 | Vision/OCR | 19_IOS_PHOTOS_MEDIA_VISION | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | apple-vision |
| 95 | OMN-0085 | Alarm Create (AlarmKit) | 22_NOTIFICATION_BACKGROUND_SCHEDULE | IOS_ONLY | Y |  | IMPLEMENTED | apple-alarm |
| 96 | OMN-0086 | Shortcuts/App Intents | 26_UI_AND_PRODUCT_EXPERIENCE | IOS_ONLY | Y |  | IMPLEMENTED | - |
| 97 | OMN-0087 | Share Sheet | 26_UI_AND_PRODUCT_EXPERIENCE | IOS_ONLY | Y |  | IMPLEMENTED | - |
| 98 | OMN-0088 | File Provider Extension | 26_UI_AND_PRODUCT_EXPERIENCE | IOS_ONLY | Y |  | IMPLEMENTED | - |
| 99 | OMN-0089 | iOS Notifications | 22_NOTIFICATION_BACKGROUND_SCHEDULE | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | apple-notification |
| 100 | OMN-0090 | NLP Processing | 29_OTHER | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | apple-nlp |
| 101 | OMN-0091 | Device Info | 29_OTHER | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | apple-device |
| 102 | OMN-0092 | Open URL/App | 29_OTHER | IOS_ONLY | Y |  | SOURCE_NOT_VERIFIED_IOS | apple-open |
| 103 | OMN-0093 | Android Clipboard Read | 21_ANDROID_SYSTEM_SERVICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-clipboard |
| 104 | OMN-0094 | Android Clipboard Write | 21_ANDROID_SYSTEM_SERVICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-clipboard |
| 105 | OMN-0095 | Android Notification Read | 22_NOTIFICATION_BACKGROUND_SCHEDULE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-notification |
| 106 | OMN-0096 | Android Notification Action | 22_NOTIFICATION_BACKGROUND_SCHEDULE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-notification |
| 107 | OMN-0097 | Android Location Get | 21_ANDROID_SYSTEM_SERVICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-location |
| 108 | OMN-0098 | Android Calendar Access | 21_ANDROID_SYSTEM_SERVICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-calendar |
| 109 | OMN-0099 | Android Contacts Read | 21_ANDROID_SYSTEM_SERVICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-contacts |
| 110 | OMN-0100 | Android Weather Query | 29_OTHER | ANDROID_ONLY |  | Y | IMPLEMENTED | android-weather |
| 111 | OMN-0101 | Android Speech-to-Text | 14_VOICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-speech |
| 112 | OMN-0102 | Android Text-to-Speech | 14_VOICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-speak |
| 113 | OMN-0103 | Android Scheduled Task | 22_NOTIFICATION_BACKGROUND_SCHEDULE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-alarm |
| 114 | OMN-0104 | Android Shizuku Execution | 21_ANDROID_SYSTEM_SERVICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-shizuku |
| 115 | OMN-0105 | Android A11y CLI | 21_ANDROID_SYSTEM_SERVICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-a11y-cli |
| 116 | OMN-0106 | Android Media Control | 21_ANDROID_SYSTEM_SERVICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-player |
| 117 | OMN-0107 | Android Photos Access | 21_ANDROID_SYSTEM_SERVICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-photos |
| 118 | OMN-0108 | Android Open URL/App | 21_ANDROID_SYSTEM_SERVICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-open |
| 119 | OMN-0109 | Android Device Info | 21_ANDROID_SYSTEM_SERVICE | ANDROID_ONLY |  | Y | IMPLEMENTED | android-device |
| 120 | OMN-0110 | Android Sessions CLI | 27_DEVELOPER_DIAGNOSTICS | ANDROID_ONLY |  | Y | IMPLEMENTED | android-sessions-cli |
| 121 | OMN-0111 | Workspace Create | 07_WORKSPACE | SHARED | Y | Y | IMPLEMENTED | - |
| 122 | OMN-0112 | Workspace Select/Switch | 07_WORKSPACE | SHARED | Y | Y | IMPLEMENTED | - |
| 123 | OMN-0113 | Workspace File Management | 07_WORKSPACE | SHARED | Y | Y | IMPLEMENTED | - |
| 124 | OMN-0114 | Workspace Import/Export | 07_WORKSPACE | SHARED | Y | Y | IMPLEMENTED | - |
| 125 | OMN-0115 | External Folder Mount (iOS) | 07_WORKSPACE | IOS_ONLY | Y |  | IMPLEMENTED | - |
| 126 | OMN-0131 | minis:// URI Scheme | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | - |
| 127 | OMN-0132 | minis://workspace/ | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | - |
| 128 | OMN-0133 | minis://attachments/ | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | - |
| 129 | OMN-0134 | minis://browser/ | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | - |
| 130 | OMN-0135 | minis://offloads/ | 06_FILE_AND_RESOURCE_URI | SHARED | Y | Y | IMPLEMENTED | - |
| 131 | OMN-0116 | iOS Permission Flow | 23_PERMISSION_SECURITY_PRIVACY | IOS_ONLY | Y |  | IMPLEMENTED | - |
| 132 | OMN-0117 | Android Permission Flow | 23_PERMISSION_SECURITY_PRIVACY | ANDROID_ONLY |  | Y | IMPLEMENTED | - |
| 133 | OMN-0118 | Keychain Storage (iOS) | 23_PERMISSION_SECURITY_PRIVACY | IOS_ONLY | Y |  | IMPLEMENTED | - |
| 134 | OMN-0119 | Android Keystore | 23_PERMISSION_SECURITY_PRIVACY | ANDROID_ONLY |  | Y | IMPLEMENTED | - |
| 135 | OMN-0120 | API Key Encryption | 23_PERMISSION_SECURITY_PRIVACY | SHARED | Y | Y | IMPLEMENTED | - |
| 136 | OMN-0121 | Log Sanitization | 23_PERMISSION_SECURITY_PRIVACY | SHARED | Y | Y | IMPLEMENTED | - |
| 137 | OMN-0122 | Background Execution (iOS) | 22_NOTIFICATION_BACKGROUND_SCHEDULE | IOS_ONLY | Y |  | IMPLEMENTED | - |
| 138 | OMN-0123 | Background Execution (Android) | 22_NOTIFICATION_BACKGROUND_SCHEDULE | ANDROID_ONLY |  | Y | IMPLEMENTED | - |
| 139 | OMN-0124 | Foreground Service (Android) | 22_NOTIFICATION_BACKGROUND_SCHEDULE | ANDROID_ONLY |  | Y | IMPLEMENTED | - |
| 140 | OMN-0125 | iOS Live Activity | 22_NOTIFICATION_BACKGROUND_SCHEDULE | IOS_ONLY | Y |  | IMPLEMENTED | - |
| 141 | OMN-0126 | iOS Background Fetch | 22_NOTIFICATION_BACKGROUND_SCHEDULE | IOS_ONLY | Y |  | IMPLEMENTED | - |
| 142 | OMN-0127 | Data Export | 24_IMPORT_EXPORT_BACKUP | SHARED | Y | Y | IMPLEMENTED | - |
| 143 | OMN-0128 | Data Import | 24_IMPORT_EXPORT_BACKUP | SHARED | Y | Y | IMPLEMENTED | - |
| 144 | OMN-0129 | Database Migration | 25_UPDATE_RECOVERY_MIGRATION | SHARED | Y | Y | IMPLEMENTED | - |
| 145 | OMN-0130 | Corruption Recovery | 25_UPDATE_RECOVERY_MIGRATION | SHARED | Y | Y | IMPLEMENTED | - |