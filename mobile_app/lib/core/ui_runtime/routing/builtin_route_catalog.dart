import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../app/route_transitions.dart';
import '../../../features/chat/presentation/pages/chat_page.dart';
import '../../../features/conversations/presentation/pages/conversation_list_page.dart';
import '../../../features/agent/presentation/pages/agent_page.dart';
import '../../../features/agent/presentation/pages/agent_task_detail_page.dart';
import '../../../features/characters/presentation/pages/character_list_page.dart';
import '../../../features/characters/presentation/pages/character_detail_page.dart';
import '../../../features/characters/presentation/pages/character_life_rules_page.dart';
import '../../../features/characters/presentation/pages/character_voice_page.dart';
import '../../../features/characters/presentation/pages/character_memory_page.dart';
import '../../../features/characters/presentation/pages/character_timeline_page.dart';
import '../../../features/characters/presentation/pages/character_proactive_page.dart';
import '../../../features/characters/presentation/pages/character_psyche_page.dart';
import '../../../features/characters/presentation/pages/character_debug_page.dart';
import '../../../features/memory/presentation/pages/memory_page.dart';
import '../../../features/memory/presentation/pages/memory_manager_page.dart';
import '../../../features/memory/presentation/pages/episodic_memory_page.dart';
import '../../../features/memory/presentation/pages/memory_graph_page.dart';
import '../../../features/memory/presentation/pages/memory_timeline_page.dart';
import '../../../features/memory/presentation/pages/user_profiles_page.dart';
import '../../../features/memory/presentation/pages/world_book_page.dart';
import '../../../features/reminders/presentation/pages/reminders_page.dart';
import '../../../features/emotes/presentation/pages/emotes_page.dart';
import '../../../features/chat_logs/presentation/pages/chat_logs_page.dart';
import '../../../features/chat_import/presentation/pages/chat_import_page.dart';
import '../../../features/extensions/presentation/pages/extension_center_page.dart';
import '../../../features/extensions/presentation/pages/extension_packages_page.dart';
import '../../../features/extensions/presentation/pages/mcp_list_page.dart';
import '../../../features/extensions/presentation/pages/mcp_detail_page.dart';
import '../../../features/extensions/presentation/pages/mcp_edit_page.dart';
import '../../../features/extensions/presentation/pages/agent_skills_page.dart';
import '../../../features/extensions/presentation/pages/system_plugins_page.dart';
import '../../../features/extensions/presentation/pages/compatible_skills_page.dart';
import '../../../features/extensions/presentation/pages/execution_runs_page.dart';
import '../../../features/extensions/presentation/pages/extension_run_detail_page.dart';
import '../../../features/extensions/presentation/pages/extension_page_host_page.dart';
import '../../../features/extensions/presentation/pages/skill_detail_page.dart';
import '../../../features/extensions/presentation/pages/plugin_detail_page.dart';
import '../../../features/game_center/presentation/pages/game_center_page.dart';
import '../../../features/desktop_pet/presentation/pages/desktop_pet_page.dart';
import '../../../features/workshop/presentation/pages/workshop_home_page.dart';
import '../../../features/workshop/presentation/pages/skill_workshop_page.dart';
import '../../../features/workshop/presentation/pages/skill_draft_editor_page.dart';
import '../../../features/workshop/presentation/pages/pet_center_page.dart';
import '../../../features/workshop/presentation/pages/pet_create_page.dart';
import '../../../features/workshop/presentation/pages/pet_tasks_page.dart';
import '../../../features/workshop/presentation/pages/pet_processing_page.dart';
import '../../../features/workshop/presentation/pages/pet_action_editor_page.dart';
import '../../../features/workshop/presentation/pages/pet_installations_page.dart';
import '../../../features/dashboard/presentation/pages/dashboard_page.dart';
import '../../../features/channels/presentation/pages/wechat_page.dart';
import '../../../features/channels/presentation/pages/qq_page.dart';
import '../../../features/channels/presentation/pages/channel_center_page.dart';
import '../../../features/characters/presentation/pages/character_create_page.dart';
import '../../../features/settings/presentation/pages/settings_page.dart';
import '../../../features/settings/presentation/pages/model_settings_page.dart';
import '../../../features/settings/presentation/pages/appearance_settings_page.dart';
import '../../../features/runtime/presentation/pages/runtime_page.dart';
import '../../../features/settings/presentation/pages/runtime_mode_page.dart';
import '../../../features/settings/presentation/pages/long_running_page.dart';
import '../../../features/settings/presentation/pages/advanced_system_page.dart';
import '../../../features/settings/presentation/pages/decision_viz_page.dart';
import '../../../features/permissions/presentation/pages/permissions_page.dart';
import '../../../features/settings/presentation/pages/backup_page.dart';
import '../../../features/settings/presentation/pages/ai_config_page.dart';
import '../../../features/settings/presentation/pages/asr_page.dart';
import '../../../features/settings/presentation/pages/deployment_page.dart';
import '../../../features/settings/presentation/pages/system_settings_page.dart';
import '../../../features/settings/presentation/pages/temporal_settings_page.dart';
import '../../../features/settings/presentation/pages/model_config_page.dart';
import '../../../features/settings/presentation/pages/safety_page.dart';
import '../../../features/settings/presentation/pages/maintenance_page.dart';
import '../../../features/settings/presentation/pages/theme_settings_page.dart';
import '../../../features/settings/presentation/pages/storage_page.dart';
import '../../../features/settings/presentation/pages/user_settings_page.dart';
import '../../../features/settings/presentation/pages/devices_page.dart';
import '../../../features/settings/presentation/pages/privacy_scan_page.dart';
import '../../../features/settings/presentation/pages/about_page_new.dart';
import '../../../features/toolbox/presentation/pages/toolbox_page.dart';
import '../../../features/toolbox/presentation/pages/toolbox_file_browser_page.dart';
import '../../../features/toolbox/presentation/pages/toolbox_workspace_page.dart';
import '../../../features/toolbox/presentation/pages/toolbox_task_log_page.dart';
import '../../../features/toolbox/presentation/pages/toolbox_log_page.dart';
import '../../../features/toolbox/presentation/pages/toolbox_prompt_trace_page.dart';
import '../../../features/toolbox/presentation/pages/toolbox_runtime_status_page.dart';
import '../../../features/toolbox/presentation/pages/toolbox_database_status_page.dart';
import '../../../features/toolbox/presentation/pages/toolbox_device_status_page.dart';
import '../../../features/developer/presentation/pages/developer_home_page.dart';
import '../../../features/developer/presentation/pages/kernel_home_page.dart';
import '../../../features/developer/presentation/pages/wasm_page.dart';
import '../../../features/developer/presentation/pages/hooks_page.dart';
import '../../../features/developer/presentation/pages/trusted_services_page.dart';
import '../../../features/developer/presentation/pages/kernel_tasks_page.dart';
import '../../../features/developer/presentation/pages/events_page.dart';
import '../../../features/developer/presentation/pages/schedules_page.dart';
import '../../../features/developer/presentation/pages/desktop_contributions_page.dart';
import '../../../features/developer/presentation/pages/updates_page.dart';
import '../../../features/developer/presentation/pages/dev_console_page.dart';
import '../../../features/developer/presentation/pages/migrations_page.dart';
import '../../../features/developer/presentation/pages/dev_mode_page.dart';

/// All built-in business routes live in this catalog instead of the app
/// bootstrap router. The router only owns recovery/authentication boundaries,
/// the shell, and extension supplied routes.
List<RouteBase> buildBuiltinBusinessRoutes() => <RouteBase>[
GoRoute(
  path: '/chat',
  pageBuilder: (context, state) => chatRootPage(
    state: state,
    child: ChatPage(
      initialConversationId: state.uri.queryParameters['conversationId'],
      initialCharacterId: state.uri.queryParameters['characterId'],
    ),
  ),
),
GoRoute(
  path: '/conversations',
  pageBuilder: (context, state) => drawerSlideFadePage(
    state: state,
    child: const ConversationListPage(),
  ),
),
GoRoute(
  path: '/dashboard',
  pageBuilder: (context, state) =>
      drawerSlideFadePage(state: state, child: const DashboardPage()),
),
GoRoute(
  path: '/channels',
  pageBuilder: (context, state) => drawerSlideFadePage(
    state: state,
    child: const ChannelCenterPage(),
  ),
),
GoRoute(
  path: '/channels/wechat',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const WechatPage(),
  ),
),
GoRoute(
  path: '/channels/qq',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const QqPage(),
  ),
),
GoRoute(
  path: '/agent',
  pageBuilder: (context, state) =>
      drawerSlideFadePage(state: state, child: const AgentPage()),
),
GoRoute(
  path: '/agent/task/:id',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: AgentTaskDetailPage(taskId: state.pathParameters['id']!),
  ),
),
GoRoute(
  path: '/characters',
  pageBuilder: (context, state) => drawerSlideFadePage(
    state: state,
    child: const CharacterListPage(),
  ),
),
GoRoute(
  path: '/characters/create',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const CharacterCreatePage(),
  ),
),
GoRoute(
  path: '/characters/:id',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: CharacterDetailPage(
      characterId: state.pathParameters['id']!,
    ),
  ),
),
GoRoute(
  path: '/characters/:id/life-rules',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: CharacterLifeRulesPage(
      characterId: state.pathParameters['id']!,
    ),
  ),
),
GoRoute(
  path: '/characters/:id/voice',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: CharacterVoicePage(
      characterId: state.pathParameters['id']!,
    ),
  ),
),
GoRoute(
  path: '/characters/:id/memory',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: CharacterMemoryPage(
      characterId: state.pathParameters['id']!,
    ),
  ),
),
GoRoute(
  path: '/characters/:id/timeline',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: CharacterTimelinePage(
      characterId: state.pathParameters['id']!,
    ),
  ),
),
GoRoute(
  path: '/characters/:id/proactive',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: CharacterProactivePage(
      characterId: state.pathParameters['id']!,
    ),
  ),
),
GoRoute(
  path: '/characters/:id/psyche',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: CharacterPsychePage(
      characterId: state.pathParameters['id']!,
    ),
  ),
),
GoRoute(
  path: '/characters/:id/debug',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: CharacterDebugPage(
      characterId: state.pathParameters['id']!,
    ),
  ),
),
GoRoute(
  path: '/memory',
  pageBuilder: (context, state) =>
      drawerSlideFadePage(state: state, child: const MemoryPage()),
),
GoRoute(
  path: '/memory/manager',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const MemoryManagerPage(),
  ),
),
GoRoute(
  path: '/memory/episodic',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const EpisodicMemoryPage(),
  ),
),
GoRoute(
  path: '/memory/graph',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const MemoryGraphPage(),
  ),
),
GoRoute(
  path: '/memory/timeline',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const MemoryTimelinePage(),
  ),
),
GoRoute(
  path: '/memory/profiles',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const UserProfilesPage(),
  ),
),
GoRoute(
  path: '/memory/world-book',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const WorldBookPage(),
  ),
),
GoRoute(
  path: '/reminders',
  pageBuilder: (context, state) =>
      drawerSlideFadePage(state: state, child: const RemindersPage()),
),
GoRoute(
  path: '/emotes',
  pageBuilder: (context, state) =>
      drawerSlideFadePage(state: state, child: const EmotesPage()),
),
GoRoute(
  path: '/chat-logs',
  pageBuilder: (context, state) =>
      drawerSlideFadePage(state: state, child: const ChatLogsPage()),
),
GoRoute(
  path: '/chat-import',
  pageBuilder: (context, state) => drawerSlideFadePage(
    state: state,
    child: const ChatImportPage(),
  ),
),
GoRoute(
  path: '/extensions',
  pageBuilder: (context, state) => drawerSlideFadePage(
    state: state,
    child: const ExtensionCenterPage(),
  ),
),
GoRoute(
  path: '/extensions/packages',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ExtensionPackagesPage(),
  ),
),
GoRoute(
  path: '/extensions/mcp',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const McpListPage(),
  ),
),
GoRoute(
  path: '/extensions/mcp/new',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const McpEditPage(mcpId: 'new'),
  ),
),
GoRoute(
  path: '/extensions/mcp/:id',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: McpDetailPage(mcpId: state.pathParameters['id']!),
  ),
),
GoRoute(
  path: '/extensions/mcp/:id/edit',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: McpEditPage(mcpId: state.pathParameters['id']!),
  ),
),
GoRoute(
  path: '/extensions/agent-skills',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const AgentSkillsPage(),
  ),
),
GoRoute(
  path: '/extensions/skills',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const CompatibleSkillsPage(),
  ),
),
GoRoute(
  path: '/extensions/skills/:id',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: SkillDetailPage(skillId: state.pathParameters['id']!),
  ),
),
GoRoute(
  path: '/extensions/plugins',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const SystemPluginsPage(),
  ),
),
GoRoute(
  path: '/extensions/plugins/:id',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: PluginDetailPage(pluginId: state.pathParameters['id']!),
  ),
),
GoRoute(
  path: '/extensions/runs',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ExecutionRunsPage(),
  ),
),
GoRoute(
  path: '/extensions/runs/:id',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: ExtensionRunDetailPage(runId: state.pathParameters['id']!),
  ),
),
GoRoute(
  path: '/extension/page/:pageId',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: ExtensionPageHostPage(
      pageId: state.pathParameters['pageId']!,
      extensionId: state.uri.queryParameters['extensionId'] ?? '',
    ),
  ),
),
GoRoute(
  path: '/game-center',
  pageBuilder: (context, state) => drawerSlideFadePage(
    state: state,
    child: const GameCenterPage(),
  ),
),
GoRoute(
  path: '/desktop-pet',
  pageBuilder: (context, state) => drawerSlideFadePage(
    state: state,
    child: const DesktopPetPage(),
  ),
),
GoRoute(
  path: '/workshop',
  pageBuilder: (context, state) => drawerSlideFadePage(
    state: state,
    child: const WorkshopHomePage(),
  ),
),
GoRoute(
  path: '/workshop/skills',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const SkillWorkshopPage(),
  ),
),
GoRoute(
  path: '/workshop/skills/:id/editor',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: SkillDraftEditorPage(draftId: state.pathParameters['id']!),
  ),
),
GoRoute(
  path: '/workshop/pet',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const PetCenterPage(),
  ),
),
GoRoute(
  path: '/workshop/pet/create',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const PetCreatePage(),
  ),
),
GoRoute(
  path: '/workshop/pet/tasks',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const PetTasksPage(),
  ),
),
GoRoute(
  path: '/workshop/pet/processing/:taskId',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: PetProcessingPage(taskId: state.pathParameters['taskId']!),
  ),
),
GoRoute(
  path: '/workshop/pet/processing/:taskId/actions/:actionKey/editor',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: PetActionEditorPage(
      taskId: state.pathParameters['taskId']!,
      actionKey: state.pathParameters['actionKey']!,
    ),
  ),
),
GoRoute(
  path: '/workshop/pet/installations',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const PetInstallationsPage(),
  ),
),
GoRoute(
  path: '/settings',
  pageBuilder: (context, state) =>
      drawerSlideFadePage(state: state, child: const SettingsPage()),
),
GoRoute(
  path: '/settings/models',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ModelSettingsPage(),
  ),
),
GoRoute(
  path: '/settings/models/:modelType',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: ModelConfigPage(
      modelType: state.pathParameters['modelType']!,
    ),
  ),
),
GoRoute(
  path: '/settings/appearance',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const AppearanceSettingsPage(),
  ),
),
GoRoute(
  path: '/settings/runtime',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const RuntimePage(),
  ),
),
GoRoute(
  path: '/settings/permissions',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const PermissionsPage(),
  ),
),
GoRoute(
  path: '/settings/backup',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const BackupPage(),
  ),
),
GoRoute(
  path: '/settings/ai',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const AiConfigPage(),
  ),
),
GoRoute(
  path: '/settings/asr',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const AsrPage(),
  ),
),
GoRoute(
  path: '/settings/deployment',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const DeploymentPage(),
  ),
),
GoRoute(
  path: '/settings/system',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const SystemSettingsPage(),
  ),
),
GoRoute(
  path: '/settings/temporal',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const TemporalSettingsPage(),
  ),
),
GoRoute(
  path: '/settings/safety',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const SafetyPage(),
  ),
),
GoRoute(
  path: '/settings/maintenance',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const MaintenancePage(),
  ),
),
GoRoute(
  path: '/settings/runtime-mode',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const RuntimeModePage(),
  ),
),
GoRoute(
  path: '/settings/long-running',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const LongRunningPage(),
  ),
),
GoRoute(
  path: '/settings/advanced',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const AdvancedSystemPage(),
  ),
),
GoRoute(
  path: '/settings/decision-viz',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const DecisionVizPage(),
  ),
),
GoRoute(
  path: '/settings/theme',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ThemeSettingsPage(),
  ),
),
GoRoute(
  path: '/settings/storage',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const StoragePage(),
  ),
),
GoRoute(
  path: '/settings/user',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const UserSettingsPage(),
  ),
),
GoRoute(
  path: '/settings/devices',
  pageBuilder: (context, state) =>
      drawerSlideFadePage(state: state, child: const DevicesPage()),
),
GoRoute(
  path: '/settings/privacy-scan',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const PrivacyScanPage(),
  ),
),
GoRoute(
  path: '/settings/about',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const AboutPageNew(),
  ),
),
GoRoute(
  path: '/settings/toolbox',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ToolboxPage(),
  ),
),
GoRoute(
  path: '/settings/toolbox/file-browser',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ToolboxFileBrowserPage(),
  ),
),
GoRoute(
  path: '/settings/toolbox/workspace',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ToolboxWorkspacePage(),
  ),
),
GoRoute(
  path: '/settings/toolbox/task-log',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ToolboxTaskLogPage(),
  ),
),
GoRoute(
  path: '/settings/toolbox/log',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ToolboxLogPage(),
  ),
),
GoRoute(
  path: '/settings/toolbox/prompt-trace',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ToolboxPromptTracePage(),
  ),
),
GoRoute(
  path: '/settings/toolbox/runtime-status',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ToolboxRuntimeStatusPage(),
  ),
),
GoRoute(
  path: '/settings/toolbox/database-status',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ToolboxDatabaseStatusPage(),
  ),
),
GoRoute(
  path: '/settings/toolbox/device-status',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const ToolboxDeviceStatusPage(),
  ),
),
GoRoute(
  path: '/developer',
  pageBuilder: (context, state) => drawerSlideFadePage(
    state: state,
    child: const DeveloperHomePage(),
  ),
),
GoRoute(
  path: '/developer/kernel',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const KernelHomePage(),
  ),
),
GoRoute(
  path: '/developer/kernel/wasm',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const WasmPage(),
  ),
),
GoRoute(
  path: '/developer/kernel/hooks',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const HooksPage(),
  ),
),
GoRoute(
  path: '/developer/kernel/trusted-services',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const TrustedServicesPage(),
  ),
),
GoRoute(
  path: '/developer/kernel/tasks',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const KernelTasksPage(),
  ),
),
GoRoute(
  path: '/developer/kernel/events',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const EventsPage(),
  ),
),
GoRoute(
  path: '/developer/kernel/schedules',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const SchedulesPage(),
  ),
),
GoRoute(
  path: '/developer/kernel/desktop',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const DesktopContributionsPage(),
  ),
),
GoRoute(
  path: '/developer/kernel/updates',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const UpdatesPage(),
  ),
),
GoRoute(
  path: '/developer/kernel/dev-console',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const DevConsolePage(),
  ),
),
GoRoute(
  path: '/developer/kernel/migrations',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const MigrationsPage(),
  ),
),
GoRoute(
  path: '/developer/kernel/dev-mode',
  pageBuilder: (context, state) => slideFadePage(
    context: context,
    state: state,
    child: const DevModePage(),
  ),
),
];
