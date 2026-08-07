# Operit v1.12.0 Capability Matrix

| Capability ID | Name | Normalized Name | Category | Evidence | State | Agent Callable | Platforms |
|---|---|---|---|---|---|---|---|
| OPR-0001 | start_chat_service | Start Chat Service | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0002 | stop_chat_service | Stop Chat Service | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0003 | create_new_chat | Create New Chat | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0004 | list_chats | List Chats | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0005 | find_chat | Find Chat | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0006 | agent_status | Agent Status | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0007 | switch_chat | Switch Chat | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0008 | update_chat_title | Update Chat Title | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0009 | delete_chat | Delete Chat | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0010 | send_message_to_ai | Send Message To AI | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0011 | send_message_to_ai_streaming | Send Message To AI (Streaming) | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0012 | list_character_cards | List Character Cards | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0013 | get_chat_messages | Get Chat Messages | 01_AGENT_AND_CHAT | E2 | IMPLEMENTED | Yes | android |
| OPR-0014 | send_message_to_ai_streaming | 发送AI消息(流式) | 01_AGENT_AND_CHAT | E3 | IMPLEMENTED | Yes | android |
| OPR-0015 | run_ui_subagent | 运行UI子代理 | 02_TASK_AND_PLANNING | E3 | IMPLEMENTED | Yes | android |
| OPR-0016 | execute_shell | Execute Shell | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0017 | create_terminal_session | Create Terminal Session | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0018 | execute_in_terminal_session | Execute In Terminal Session | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0019 | execute_in_terminal_session_streaming | Execute In Terminal Session (Streaming) | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0020 | execute_hidden_terminal_command | Execute Hidden Terminal Command | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0021 | close_terminal_session | Close Terminal Session | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0022 | input_in_terminal_session | Input In Terminal Session | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0023 | get_terminal_session_screen | Get Terminal Session Screen | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0024 | use_package | Use Package | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0025 | package_proxy | Package Proxy | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0026 | MCP tools (dynamic) | MCP Server Tools | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0027 | JS Package tools (dynamic) | JavaScript Package Tools | 03_TOOL_RUNTIME | E2 | IMPLEMENTED | Yes | android |
| OPR-0028 | adb_shizuku_shell_command_execution | ADB/Shizuku Shell Command Execution | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0029 | adb_shizuku_shell_command_execution | ADB/Shizuku Shell Command Execution | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0030 | root_shell_command_execution | Root Shell Command Execution | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0031 | standard_runtime_shell_execution | Standard Runtime Shell Execution | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0032 | accessibility_service_shell_wrapper | Accessibility Service Shell Wrapper | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0033 | shizuku_permission_management | Shizuku Permission Management | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0034 | shizuku_bundled_installer | Shizuku Bundled Installer | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0035 | shizuku_action_listener_(ui) | Shizuku Action Listener (UI) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0036 | screen_capture_(screenshot) | Screen Capture (Screenshot) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0037 | screen_capture_(screenshot) | Screen Capture (Screenshot) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0038 | screen_capture_(imagereader) | Screen Capture (ImageReader) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0039 | screen_capture_(virtualdisplay_manager) | Screen Capture (VirtualDisplay Manager) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0040 | screen_recording___casting_(shower) | Screen Recording / Casting (Shower) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0041 | screen_casting_(virtualdisplay_overlay) | Screen Casting (VirtualDisplay Overlay) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0042 | application_install_uninstall | Application Install/Uninstall | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0043 | application_install_(apk) | Application Install (APK) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0044 | input_event_injection_(shizuku_adb) | Input Event Injection (Shizuku/ADB) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0045 | system_settings_manipulation | System Settings Manipulation | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0046 | system_settings_via_shell | System Settings via Shell | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0047 | notification_management | Notification Management | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0048 | notification_fgs_management | Notification FGS Management | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0049 | clipboard_access | Clipboard Access | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0050 | clipboard_fallback | Clipboard Fallback | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0051 | share_intent_(action_send) | Share Intent (ACTION_SEND) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0052 | screenshot_share | Screenshot Share | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0053 | bluetooth_device_scan | Bluetooth Device Scan | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0054 | bluetooth_device_management | Bluetooth Device Management | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0055 | location_tracking | Location Tracking | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0056 | location_tracking_(continuous) | Location Tracking (Continuous) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0057 | microphone_recording | Microphone Recording | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0058 | microphone_(oboe) | Microphone (Oboe) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0059 | camera_capture_(imagereader) | Camera Capture (ImageReader) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0060 | floating_window_overlay | Floating Window Overlay | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0061 | phone_agent_ui_automation | Phone Agent UI Automation | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0062 | mcp_plugin_install_&_server | MCP Plugin Install & Server | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0063 | external_http_chat_server | External HTTP Chat Server | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0064 | external_http_web_chat_bridge | External HTTP Web Chat Bridge | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0065 | external_intent_chat_receiver | External Intent Chat Receiver | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0066 | javascript_execution_(js_bridge) | JavaScript Execution (JS Bridge) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0067 | package_plugin_install_receiver | Package/Plugin Install Receiver | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0068 | voice_interaction_service | Voice Interaction Service | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0069 | assistant_intent_activity | Assistant Intent Activity | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0070 | ui_debugger_overlay_service | UI Debugger Overlay Service | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0071 | work_scheduler_(alarm) | Work Scheduler (Alarm) | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0072 | device_admin_privilege | Device Admin Privilege | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0073 | tasker_integration | Tasker Integration | 04_ANDROID_DEVICE_CONTROL | E3 | IMPLEMENTED | Yes | android |
| OPR-0074 | shellexecutor | ShellExecutor | 05_LINUX_AND_SHELL | E2 | IMPLEMENTED | Yes | android |
| OPR-0075 | standardshellexecutor | StandardShellExecutor | 05_LINUX_AND_SHELL | E2 | IMPLEMENTED | Yes | android |
| OPR-0076 | rootshellexecutor | RootShellExecutor | 05_LINUX_AND_SHELL | E2 | IMPLEMENTED | Yes | android |
| OPR-0077 | adminshellexecutor | AdminShellExecutor | 05_LINUX_AND_SHELL | E2 | IMPLEMENTED | Yes | android |
| OPR-0078 | debuggershellexecutor | DebuggerShellExecutor | 05_LINUX_AND_SHELL | E2 | IMPLEMENTED | Yes | android |
| OPR-0079 | accessibilityshellexecutor | AccessibilityShellExecutor | 05_LINUX_AND_SHELL | E2 | IMPLEMENTED | Yes | android |
| OPR-0080 | shellexecutorfactory | ShellExecutorFactory | 05_LINUX_AND_SHELL | E2 | IMPLEMENTED | Yes | android |
| OPR-0081 | androidshellexecutor | AndroidShellExecutor | 05_LINUX_AND_SHELL | E2 | IMPLEMENTED | Yes | android |
| OPR-0082 | operitshowershellrunner | OperitShowerShellRunner | 05_LINUX_AND_SHELL | E2 | IMPLEMENTED | Yes | android |
| OPR-0083 | proot_linux_sandbox | PRoot Linux沙盒 | 05_LINUX_AND_SHELL | E2 | IMPLEMENTED | Yes | android |
| OPR-0084 | list_files | List Files | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0085 | read_file | Read File | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0086 | read_file_part | Read File Part | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0087 | read_file_full | Read File Full | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0088 | read_file_binary | Read File Binary | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0089 | write_file | Write File | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0090 | write_file_binary | Write File Binary | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0091 | delete_file | Delete File | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0092 | create_file | Create File | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0093 | edit_file | Edit File | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0094 | apply_file | Apply File | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0095 | make_directory | Make Directory | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0096 | find_files | Find Files | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0097 | grep_code | Grep Code | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0098 | grep_context | Grep Context | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0099 | download_file | Download File | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0100 | file_exists | File Exists | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0101 | file_info | File Info | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0102 | move_file | Move File | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0103 | copy_file | Copy File | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0104 | zip_files | Zip Files | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0105 | unzip_files | Unzip Files | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0106 | open_file | Open File | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0107 | share_file | Share File | 06_FILE_AND_NETWORK | E3 | IMPLEMENTED | Yes | android |
| OPR-0108 | http_request | HTTP Request | 06_FILE_AND_NETWORK | E2 | IMPLEMENTED | Yes | android |
| OPR-0109 | multipart_request | Multipart Request | 06_FILE_AND_NETWORK | E2 | IMPLEMENTED | Yes | android |
| OPR-0110 | manage_cookies | Manage Cookies | 06_FILE_AND_NETWORK | E2 | IMPLEMENTED | Yes | android |
| OPR-0111 | standardfilesystemtools | StandardFileSystemTools | 06_FILE_AND_NETWORK | E1 | IMPLEMENTED | Yes | android |
| OPR-0112 | linuxfilesystemtools | LinuxFileSystemTools | 06_FILE_AND_NETWORK | E1 | IMPLEMENTED | Yes | android |
| OPR-0113 | saffilesystemtools | SafFileSystemTools | 06_FILE_AND_NETWORK | E1 | IMPLEMENTED | Yes | android |
| OPR-0114 | debuggerfilesystemtools | DebuggerFileSystemTools | 06_FILE_AND_NETWORK | E1 | IMPLEMENTED | Yes | android |
| OPR-0115 | filesystemprovider_(interface) | FileSystemProvider (interface) | 06_FILE_AND_NETWORK | E1 | IMPLEMENTED | Yes | android |
| OPR-0116 | localfilesystemprovider | LocalFileSystemProvider | 06_FILE_AND_NETWORK | E1 | IMPLEMENTED | Yes | android |
| OPR-0117 | sshfilesystemprovider | SSHFileSystemProvider | 06_FILE_AND_NETWORK | E1 | IMPLEMENTED | Yes | android |
| OPR-0118 | sshfileconnectionmanager | SSHFileConnectionManager | 06_FILE_AND_NETWORK | E1 | IMPLEMENTED | Yes | android |
| OPR-0119 | filemanager_(workspace侧) | FileManager (workspace侧) | 06_FILE_AND_NETWORK | E1 | IMPLEMENTED | Yes | android |
| OPR-0120 | filemanagerviewmodel/screen | FileManagerViewModel/Screen | 06_FILE_AND_NETWORK | E1 | IMPLEMENTED | Yes | android |
| OPR-0121 | ssh_file_operations | SSH远程文件操作 | 06_FILE_AND_NETWORK | E1 | IMPLEMENTED | Yes | android |
| OPR-0122 | visit_web | Visit Web | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0123 | browser_click | Browser Click | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0124 | browser_close | Browser Close | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0125 | browser_close_all | Browser Close All | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0126 | browser_console_messages | Browser Console Messages | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0127 | browser_drag | Browser Drag | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0128 | browser_evaluate | Browser Evaluate | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0129 | browser_file_upload | Browser File Upload | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0130 | browser_fill_form | Browser Fill Form | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0131 | browser_handle_dialog | Browser Handle Dialog | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0132 | browser_hover | Browser Hover | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0133 | browser_navigate | Browser Navigate | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0134 | browser_navigate_back | Browser Navigate Back | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0135 | browser_network_requests | Browser Network Requests | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0136 | browser_press_key | Browser Press Key | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0137 | browser_resize | Browser Resize | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0138 | browser_run_code | Browser Run Code | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0139 | browser_select_option | Browser Select Option | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0140 | browser_snapshot | Browser Snapshot | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0141 | browser_take_screenshot | Browser Take Screenshot | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0142 | browser_tabs | Browser Tabs | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0143 | browser_type | Browser Type | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0144 | browser_wait_for | Browser Wait For | 07_BROWSER_AND_SEARCH | E2 | IMPLEMENTED | Yes | android |
| OPR-0145 | music_play | Music Play | 08_MEDIA_AND_VISION | E2 | IMPLEMENTED | Yes | android |
| OPR-0146 | music_play_queue | Music Play Queue | 08_MEDIA_AND_VISION | E2 | IMPLEMENTED | Yes | android |
| OPR-0147 | music_pause | Music Pause | 08_MEDIA_AND_VISION | E2 | IMPLEMENTED | Yes | android |
| OPR-0148 | music_resume | Music Resume | 08_MEDIA_AND_VISION | E2 | IMPLEMENTED | Yes | android |
| OPR-0149 | music_stop | Music Stop | 08_MEDIA_AND_VISION | E2 | IMPLEMENTED | Yes | android |
| OPR-0150 | music_seek | Music Seek | 08_MEDIA_AND_VISION | E2 | IMPLEMENTED | Yes | android |
| OPR-0151 | music_set_volume | Music Set Volume | 08_MEDIA_AND_VISION | E2 | IMPLEMENTED | Yes | android |
| OPR-0152 | music_status | Music Status | 08_MEDIA_AND_VISION | E2 | IMPLEMENTED | Yes | android |
| OPR-0153 | ffmpeg_execute | FFmpeg Execute | 08_MEDIA_AND_VISION | E2 | IMPLEMENTED | Yes | android |
| OPR-0154 | ffmpeg_info | FFmpeg Info | 08_MEDIA_AND_VISION | E2 | IMPLEMENTED | Yes | android |
| OPR-0155 | ffmpeg_convert | FFmpeg Convert | 08_MEDIA_AND_VISION | E2 | IMPLEMENTED | Yes | android |
| OPR-0156 | camera_image_capture | 相机图片捕获 | 08_MEDIA_AND_VISION | E1 | IMPLEMENTED | No | android |
| OPR-0157 | workspace_dev_server | Workspace开发服务器 | 09_WORKSPACE_AND_REMOTE | E1 | IMPLEMENTED | Yes | android |
| OPR-0158 | mcp_local_server_launch | MCP本地服务器启动 | 10_MCP | E3 | IMPLEMENTED | Yes | android |
| OPR-0159 | mcp_remote_server_connect | MCP远程服务器连接 | 10_MCP | E2 | IMPLEMENTED | Yes | android |
| OPR-0160 | mcp_tool_invoke | MCP工具调用 | 10_MCP | E3 | IMPLEMENTED | Yes | android |
| OPR-0161 | skill_import_from_zip | 技能包ZIP导入 | 11_SKILL | E2 | IMPLEMENTED | No | android |
| OPR-0162 | skill_install_from_github | 技能包GitHub安装 | 11_SKILL | E2 | IMPLEMENTED | No | android |
| OPR-0163 | skill_system_prompt_injection | 技能系统提示注入 | 11_SKILL | E2 | IMPLEMENTED | No | android |
| OPR-0164 | toolpkg_install_zip | 工具包ZIP安装 | 12_TOOL_PACKAGE | E2 | IMPLEMENTED | No | android |
| OPR-0165 | toolpkg_hook_bridge_chat_input | 工具包聊天输入钩子桥接 | 12_TOOL_PACKAGE | E2 | IMPLEMENTED | No | android |
| OPR-0166 | toolpkg_hook_bridge_prompt_injection | 工具包提示词钩子桥接 | 12_TOOL_PACKAGE | E2 | IMPLEMENTED | No | android |
| OPR-0167 | workflow_create | 创建工作流 | 13_WORKFLOW_AUTOMATION | E2 | IMPLEMENTED | Yes | android |
| OPR-0168 | workflow_trigger_scheduled | 定时触发工作流 | 13_WORKFLOW_AUTOMATION | E2 | IMPLEMENTED | Yes | android |
| OPR-0169 | workflow_trigger_tasker | Tasker触发工作流 | 13_WORKFLOW_AUTOMATION | E2 | IMPLEMENTED | Yes | android |
| OPR-0170 | workflow_dag_execute | 工作流DAG拓扑执行 | 13_WORKFLOW_AUTOMATION | E3 | IMPLEMENTED | Yes | android |
| OPR-0171 | hook_tool_lifecycle_intercept | 工具生命周期拦截钩子 | 14_HOOK_EVENT_SCHEDULE | E3 | IMPLEMENTED | Yes | android |
| OPR-0172 | prompt_hook_system_compose | 系统提示组合钩子 | 14_HOOK_EVENT_SCHEDULE | E2 | IMPLEMENTED | No | android |
| OPR-0173 | openaiprovider | OpenAIProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0174 | openairesponsesprovider | OpenAIResponsesProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0175 | claudeprovider | ClaudeProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0176 | geminiprovider | GeminiProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0177 | ollamaprovider | OllamaProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0178 | qwenaiprovider | QwenAIProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0179 | kimiprovider | KimiProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0180 | deepseekprovider | DeepseekProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0181 | mimoprovider | MimoProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0182 | mistralprovider | MistralProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0183 | openrouterprovider | OpenRouterProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0184 | fourrouterprovider | FourRouterProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0185 | nousportalprovider | NousPortalProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0186 | doubaoaiprovider | DoubaoAIProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0187 | nvidiaaiprovider | NvidiaAIProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0188 | mnnprovider | MNNProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0189 | llamaprovider | LlamaProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0190 | toolpkgjsaiproviderservice | ToolPkgJsAiProviderService | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0191 | apikeyprovider_/_multiapikeyprovider | ApiKeyProvider / MultiApiKeyProvider | 15_MODEL_PROVIDER | E2 | IMPLEMENTED | No | android |
| OPR-0192 | local_model_llama_cpp | 本地模型 LLAMA_CPP | 16_LOCAL_MODEL | E2 | IMPLEMENTED | No | android |
| OPR-0193 | local_model_mnn | 本地模型 MNN | 16_LOCAL_MODEL | E2 | IMPLEMENTED | No | android |
| OPR-0194 | speechservice_interface_(stt) | SpeechService Interface (STT) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0195 | speechservicefactory_(stt) | SpeechServiceFactory (STT) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0196 | sherpaspeechprovider_(local_stt) | SherpaSpeechProvider (Local STT) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0197 | sherpamnnspeechprovider_(local_stt) | SherpaMnnSpeechProvider (Local STT) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0198 | deepgramsttprovider_(cloud_stt) | DeepgramSttProvider (Cloud STT) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0199 | openaisttprovider_(cloud_stt) | OpenAISttProvider (Cloud STT) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0200 | onnxsilerovad | OnnxSileroVad | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0201 | personalwakelistener_(custom_wake_word) | PersonalWakeListener (Custom Wake Word) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0202 | personalwakefeatureextractor | PersonalWakeFeatureExtractor | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0203 | personalwakeenrollment | PersonalWakeEnrollment | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0204 | speechprerollstore | SpeechPrerollStore | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0205 | voiceservice | VoiceService Interface (TTS) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0206 | voiceservicefactory | VoiceServiceFactory (TTS) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0207 | openairealtimevoiceprovider | OpenAIRealtimeVoiceProvider (TTS + Realtime) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0208 | openaivoiceprovider | OpenAIVoiceProvider (TTS) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0209 | siliconflowvoiceprovider | SiliconFlowVoiceProvider (TTS) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0210 | minimaxvoiceprovider | MiniMaxVoiceProvider (TTS) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0211 | mimovoiceprovider | MimoVoiceProvider (TTS) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0212 | doubaovoiceprovider | DoubaoVoiceProvider (TTS) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0213 | httpvoiceprovider | HttpVoiceProvider (TTS) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0214 | vitsvoiceprovider | VitsVoiceProvider (Local TTS) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0215 | accessibilityvoiceprovider | AccessibilityVoiceProvider (TTS) | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0216 | queuedttsplayback | QueuedTtsPlayback | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0217 | voicelistfetcher | VoiceListFetcher | 17_VOICE | E1 | IMPLEMENTED | No | android |
| OPR-0218 | device_info | Device Info | 25_DEVELOPER_AND_DIAGNOSTICS | E2 | IMPLEMENTED | Yes | android |
| OPR-0219 | execute_intent | Execute Intent | 24_RUNTIME_AND_INFRASTRUCTURE | E2 | IMPLEMENTED | Yes | android |
| OPR-0220 | send_broadcast | Send Broadcast | 24_RUNTIME_AND_INFRASTRUCTURE | E2 | IMPLEMENTED | Yes | android |
| OPR-0221 | trigger_tasker_event | Trigger Tasker Event | 24_RUNTIME_AND_INFRASTRUCTURE | E2 | IMPLEMENTED | Yes | android |
| OPR-0222 | RootFileSystemTools | Root File System Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0223 | AdminFileSystemTools | Admin File System Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0224 | DebuggerFileSystemTools | Debugger File System Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0225 | AccessibilityFileSystemTools | Accessibility File System Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0226 | RootUITools | Root UI Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0227 | AdminUITools | Admin UI Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0228 | DebuggerUITools | Debugger UI Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0229 | AccessibilityUITools | Accessibility UI Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0230 | RootSystemOperationTools | Root System Operation Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0231 | AdminSystemOperationTools | Admin System Operation Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0232 | DebuggerSystemOperationTools | Debugger System Operation Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0233 | AccessibilitySystemOperationTools | Accessibility System Operation Tools | 03_TOOL_RUNTIME | E1 | IMPLEMENTED | Yes | android |
| OPR-0234 | query_memory | Query Memory | 18_MEMORY | E2 | IMPLEMENTED | Yes | android |
| OPR-0235 | get_memory_by_title | Get Memory By Title | 18_MEMORY | E2 | IMPLEMENTED | Yes | android |
| OPR-0236 | update_user_preferences | Update User Preferences | 18_MEMORY | E2 | IMPLEMENTED | Yes | android |
| OPR-0237 | create_memory | Create Memory | 18_MEMORY | E2 | IMPLEMENTED | Yes | android |
| OPR-0238 | update_memory | Update Memory | 18_MEMORY | E2 | IMPLEMENTED | Yes | android |
| OPR-0239 | delete_memory | Delete Memory | 18_MEMORY | E2 | IMPLEMENTED | Yes | android |
| OPR-0240 | move_memory | Move Memory | 18_MEMORY | E2 | IMPLEMENTED | Yes | android |
| OPR-0241 | link_memories | Link Memories | 18_MEMORY | E2 | IMPLEMENTED | Yes | android |
| OPR-0242 | query_memory_links | Query Memory Links | 18_MEMORY | E2 | IMPLEMENTED | Yes | android |
| OPR-0243 | update_memory_link | Update Memory Link | 18_MEMORY | E2 | IMPLEMENTED | Yes | android |
| OPR-0244 | delete_memory_link | Delete Memory Link | 18_MEMORY | E2 | IMPLEMENTED | Yes | android |
| OPR-0245 | memoryrepository | MemoryRepository | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0246 | memory_entity | Memory Entity | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0247 | memorylink_entity | MemoryLink Entity | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0248 | memorytag_entity | MemoryTag Entity | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0249 | vectorindexmanager | VectorIndexManager | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0250 | indexitem | IndexItem | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0251 | memorylibrary | MemoryLibrary | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0252 | memoryautosavescheduler | MemoryAutoSaveScheduler | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0253 | memoryautosavecandidaterepository | MemoryAutoSaveCandidateRepository | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0254 | memorysearchsettingspreferences | MemorySearchSettingsPreferences | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0255 | embedding_/_embeddingconverter | Embedding / EmbeddingConverter | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0256 | cloudembeddingservice | CloudEmbeddingService | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0257 | documentchunk_entity | DocumentChunk Entity | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0258 | memorydocumentsprovider | MemoryDocumentsProvider | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0259 | memorysearchconfig_/_memorysearchdebuginfo | MemorySearchConfig / MemorySearchDebugInfo | 18_MEMORY | E1 | IMPLEMENTED | No | android |
| OPR-0260 | character_card_crud | 角色卡CRUD管理 | 19_CHARACTER | E2 | IMPLEMENTED | No | android |
| OPR-0261 | character_card_sillytavern_import | SillyTavern角色卡导入 | 19_CHARACTER | E1 | IMPLEMENTED | No | android |
| OPR-0262 | character_card_tool_access_control | 角色卡工具访问控制 | 19_CHARACTER | E2 | IMPLEMENTED | No | android |
| OPR-0263 | permission_post_notifications | 权限: android.permission.POST_NOTIFICATIONS | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0264 | permission_manage_external_storage | 权限: android.permission.MANAGE_EXTERNAL_STORAGE | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0265 | permission_system_alert_window | 权限: android.permission.SYSTEM_ALERT_WINDOW | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0266 | permission_package_usage_stats | 权限: android.permission.PACKAGE_USAGE_STATS | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0267 | permission_write_settings | 权限: android.permission.WRITE_SETTINGS | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0268 | permission_record_audio | 权限: android.permission.RECORD_AUDIO | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0269 | permission_camera | 权限: android.permission.CAMERA | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0270 | permission_access_fine_location | 权限: android.permission.ACCESS_FINE_LOCATION | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0271 | permission_access_coarse_location | 权限: android.permission.ACCESS_COARSE_LOCATION | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0272 | permission_bluetooth_connect | 权限: android.permission.BLUETOOTH_CONNECT | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0273 | permission_bluetooth_scan | 权限: android.permission.BLUETOOTH_SCAN | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0274 | permission_call_phone | 权限: android.permission.CALL_PHONE | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0275 | permission_send_sms | 权限: android.permission.SEND_SMS | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0276 | permission_read_contacts | 权限: android.permission.READ_CONTACTS | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0277 | permission_write_contacts | 权限: android.permission.WRITE_CONTACTS | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0278 | permission_read_phone_state | 权限: android.permission.READ_PHONE_STATE | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0279 | permission_read_external_storage | 权限: android.permission.READ_EXTERNAL_STORAGE | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0280 | permission_api_v23 | 权限: moe.shizuku.manager.permission.API_V23 | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0281 | permission_request_install_packages | 权限: android.permission.REQUEST_INSTALL_PACKAGES | 20_PERMISSION_AND_SECURITY | E2 | IMPLEMENTED | No | android |
| OPR-0282 | floatingchatservice | FloatingChatService | 21_UI_AND_OVERLAY | E2 | IMPLEMENTED | No | android |
| OPR-0283 | uidebuggerservice | UIDebuggerService | 21_UI_AND_OVERLAY | E2 | IMPLEMENTED | No | android |
| OPR-0284 | uidebuggerwindowmanager | UIDebuggerWindowManager | 21_UI_AND_OVERLAY | E2 | IMPLEMENTED | No | android |
| OPR-0285 | floatingwindowmanager | FloatingWindowManager | 21_UI_AND_OVERLAY | E2 | IMPLEMENTED | No | android |
| OPR-0286 | virtualdisplayoverlay | VirtualDisplayOverlay | 21_UI_AND_OVERLAY | E2 | IMPLEMENTED | No | android |
| OPR-0287 | uioperationoverlay | UIOperationOverlay | 21_UI_AND_OVERLAY | E2 | IMPLEMENTED | No | android |
| OPR-0288 | permissionrequestoverlay | PermissionRequestOverlay | 21_UI_AND_OVERLAY | E2 | IMPLEMENTED | No | android |
| OPR-0289 | pluginloadingscreenwithstate | PluginLoadingScreenWithState | 21_UI_AND_OVERLAY | E2 | IMPLEMENTED | No | android |
| OPR-0290 | floating_chat_overlay | 悬浮聊天浮窗 | 21_UI_AND_OVERLAY | E2 | IMPLEMENTED | No | android |
| OPR-0291 | chat_export_html | 聊天记录HTML导出 | 22_IMPORT_EXPORT_BACKUP | E1 | IMPLEMENTED | No | android |
| OPR-0292 | chat_import_format_detect | 聊天记录格式检测 | 22_IMPORT_EXPORT_BACKUP | E2 | IMPLEMENTED | No | android |
| OPR-0293 | backup_room_database_zip | Room数据库ZIP备份 | 22_IMPORT_EXPORT_BACKUP | E2 | IMPLEMENTED | No | android |
| OPR-0294 | backup_raw_snapshot | 原始文件系统快照备份 | 22_IMPORT_EXPORT_BACKUP | E2 | IMPLEMENTED | No | android |
| OPR-0295 | workspace_version_backup | 工作区版本备份 | 22_IMPORT_EXPORT_BACKUP | E2 | IMPLEMENTED | No | android |
| OPR-0296 | update_patch_incremental | 增量补丁更新 | 23_UPDATE_AND_RECOVERY | E2 | IMPLEMENTED | No | android |
| OPR-0297 | update_full_apk | 完整APK更新 | 23_UPDATE_AND_RECOVERY | E2 | IMPLEMENTED | No | android |
| OPR-0298 | crash_global_exception_handler | 全局崩溃异常处理 | 23_UPDATE_AND_RECOVERY | E2 | IMPLEMENTED | No | android |
| OPR-0299 | shellexecutorfactory | ShellExecutorFactory | 05_LINUX_AND_SHELL | E1 | IMPLEMENTED | No | android |
| OPR-0300 | androidshellexecutor | AndroidShellExecutor | 05_LINUX_AND_SHELL | E1 | IMPLEMENTED | No | android |
| OPR-0301 | terminal_session_manager | 终端会话管理器 | 05_LINUX_AND_SHELL | E2 | IMPLEMENTED | Yes | android |
| OPR-0302 | external_chat_http_server | 外部聊天HTTP服务器 | 24_RUNTIME_AND_INFRASTRUCTURE | E2 | IMPLEMENTED | No | android |
| OPR-0303 | ftp_server_manager | FTP服务器管理 | 09_WORKSPACE_AND_REMOTE | E1 | IMPLEMENTED | No | android |
| OPR-0304 | sshd_server_manager | SSHD服务器管理 | 09_WORKSPACE_AND_REMOTE | E1 | IMPLEMENTED | No | android |
| OPR-0305 | js_quickjs_engine | QuickJS脚本执行引擎 | 24_RUNTIME_AND_INFRASTRUCTURE | E2 | IMPLEMENTED | Yes | android |
| OPR-0306 | ubuntudocumentsprovider | UbuntuDocumentsProvider | 24_RUNTIME_AND_INFRASTRUCTURE | E1 | IMPLEMENTED | No | android |
| OPR-0307 | operitdatadocumentsprovider | OperitDataDocumentsProvider | 24_RUNTIME_AND_INFRASTRUCTURE | E1 | IMPLEMENTED | No | android |
| OPR-0308 | workspacedocumentsprovider | WorkspaceDocumentsProvider | 24_RUNTIME_AND_INFRASTRUCTURE | E1 | IMPLEMENTED | No | android |
| OPR-0309 | memorydocumentsprovider | MemoryDocumentsProvider | 24_RUNTIME_AND_INFRASTRUCTURE | E1 | IMPLEMENTED | No | android |
| OPR-0310 | logcat_viewer | Logcat日志查看器 | 25_DEVELOPER_AND_DIAGNOSTICS | E1 | IMPLEMENTED | No | android |
| OPR-0311 | sql_database_viewer | SQL数据库查看器 | 25_DEVELOPER_AND_DIAGNOSTICS | E1 | IMPLEMENTED | No | android |
| OPR-0312 | ui_debugger_overlay | UI调试器叠加层 | 25_DEVELOPER_AND_DIAGNOSTICS | E2 | IMPLEMENTED | No | android |
| OPR-0313 | calculator | 计算器 | 26_OTHER | E2 | IMPLEMENTED | Yes | android |
| OPR-0314 | sleep_delay | 延迟休眠 | 26_OTHER | E2 | IMPLEMENTED | Yes | android |
| OPR-0315 | request_bluetooth_permission | Request Bluetooth Permission | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0316 | get_bluetooth_state | Get Bluetooth State | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0317 | request_enable_bluetooth | Request Enable Bluetooth | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0318 | list_bluetooth_bonded_devices | List Bluetooth Bonded Devices | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0319 | scan_bluetooth_devices | Scan Bluetooth Devices | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0320 | bluetooth_connect | Bluetooth Connect | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0321 | bluetooth_listen | Bluetooth Listen | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0322 | bluetooth_accept | Bluetooth Accept | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0323 | bluetooth_send | Bluetooth Send | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0324 | bluetooth_read | Bluetooth Read | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0325 | bluetooth_send_and_read | Bluetooth Send And Read | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0326 | bluetooth_close | Bluetooth Close | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0327 | bluetooth_ble_connect | Bluetooth BLE Connect | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0328 | bluetooth_ble_discover_services | Bluetooth BLE Discover Services | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0329 | bluetooth_ble_read_characteristic | Bluetooth BLE Read Characteristic | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0330 | bluetooth_ble_write_characteristic | Bluetooth BLE Write Characteristic | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0331 | bluetooth_ble_write_and_read_characteristic | BLE Write And Read Characteristic | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0332 | bluetooth_ble_subscribe_characteristic | BLE Subscribe Characteristic | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0333 | bluetooth_ble_read_notifications | BLE Read Notifications | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0334 | toast | Toast | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0335 | send_notification | Send Notification | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0336 | modify_system_setting | Modify System Setting | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0337 | get_system_setting | Get System Setting | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0338 | install_app | Install App | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0339 | uninstall_app | Uninstall App | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0340 | list_installed_apps | List Installed Apps | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0341 | start_app | Start App | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0342 | stop_app | Stop App | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0343 | get_notifications | Get Notifications | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0344 | get_app_usage_time | Get App Usage Time | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0345 | get_device_location | Get Device Location | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0346 | click_element | Click Element | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0347 | tap | Tap | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0348 | long_press | Long Press | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0349 | set_input_text | Set Input Text | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0350 | press_key | Press Key | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0351 | swipe | Swipe | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0352 | capture_screenshot | Capture Screenshot | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0353 | get_page_info | Get Page Info | 04_ANDROID_DEVICE_CONTROL | E2 | IMPLEMENTED | Yes | android |
| OPR-0354 | close_all_virtual_displays | Close All Virtual Displays | 26_OTHER | E2 | IMPLEMENTED | Yes | android |
| OPR-0355 | toolpkgtoollifecyclebridge | ToolPkgToolLifecycleBridge | 14_HOOK_EVENT_SCHEDULE | E1 | IMPLEMENTED | No | android |
| OPR-0356 | toolpkgprompthookbridge | ToolPkgPromptHookBridge | 14_HOOK_EVENT_SCHEDULE | E1 | IMPLEMENTED | No | android |
| OPR-0357 | toolpkgsummaryhookbridge | ToolPkgSummaryHookBridge | 14_HOOK_EVENT_SCHEDULE | E1 | IMPLEMENTED | No | android |
| OPR-0358 | toolpkgchatinputhookbridge | ToolPkgChatInputHookBridge | 14_HOOK_EVENT_SCHEDULE | E1 | IMPLEMENTED | No | android |
| OPR-0359 | toolpkgchatviewhookbridge | ToolPkgChatViewHookBridge | 14_HOOK_EVENT_SCHEDULE | E1 | IMPLEMENTED | No | android |
| OPR-0360 | toolpkghookbridgesupport | ToolPkgHookBridgeSupport | 14_HOOK_EVENT_SCHEDULE | E1 | IMPLEMENTED | No | android |
| OPR-0361 | workflowscheduler | WorkflowScheduler | 14_HOOK_EVENT_SCHEDULE | E1 | IMPLEMENTED | No | android |
| OPR-0362 | workflowbootreceiver | WorkflowBootReceiver | 14_HOOK_EVENT_SCHEDULE | E1 | IMPLEMENTED | No | android |
| OPR-0363 | roomdatabasebackupscheduler | RoomDatabaseBackupScheduler | 14_HOOK_EVENT_SCHEDULE | E1 | IMPLEMENTED | No | android |
| OPR-0364 | operitapplication | OperitApplication | 14_HOOK_EVENT_SCHEDULE | E1 | IMPLEMENTED | No | android |
| OPR-0365 | floatingchatservice | FloatingChatService | 14_HOOK_EVENT_SCHEDULE | E1 | IMPLEMENTED | No | android |
