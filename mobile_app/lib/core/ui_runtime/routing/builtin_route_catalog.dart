import 'package:go_router/go_router.dart';

import '../../../app/amitia_page_route.dart';
import '../../../app/app_routes.dart';
import '../../../app/route_transitions.dart';
import '../../../features/chat/presentation/pages/chat_page.dart';
import '../../../features/chat/presentation/pages/realtime_call_page.dart';
import '../../../features/settings/presentation/pages/notification_settings_page.dart';
import '../../../features/conversations/presentation/pages/conversation_list_page.dart';
import '../../../features/conversations/presentation/pages/conversation_search_page.dart';
import '../../../features/agent/presentation/pages/agent_page.dart';
import '../../../features/agent/presentation/pages/agent_task_detail_page.dart';
import '../../../features/characters/presentation/pages/character_list_page.dart';
import '../../../features/characters/presentation/pages/character_card_workshop_page.dart';
import '../../../features/characters/presentation/pages/character_detail_page.dart';
import '../../../features/characters/presentation/pages/character_voice_page.dart';
import '../../../features/characters/presentation/pages/character_memory_page.dart';
import '../../../features/characters/presentation/pages/character_timeline_page.dart';
import '../../../features/characters/presentation/pages/character_psyche_page.dart';
import '../../../features/memory/presentation/pages/memory_overview_page.dart';
import '../../../features/memory/presentation/pages/episodic_memory_page.dart';
import '../../../features/memory/presentation/pages/memory_graph_page.dart';
import '../../../features/memory/presentation/pages/memory_timeline_page.dart';
import '../../../features/memory/presentation/pages/user_profiles_page.dart';
import '../../../features/memory/presentation/pages/world_book_page.dart';
import '../../../features/knowledge/presentation/pages/knowledge_page.dart';
import '../../../features/reminders/presentation/pages/reminders_page.dart';
import '../../../features/continuity/presentation/pages/continuity_page.dart';
import '../../../features/continuity/presentation/pages/continuity_detail_page.dart';
import '../../../features/chat_logs/presentation/pages/chat_logs_page.dart';
import '../../../features/chat_import/presentation/pages/chat_import_page.dart';
import '../../../features/channels/presentation/pages/channel_messages_page.dart';
import '../../../features/extensions/presentation/pages/extension_center_page.dart';
import '../../../features/extensions/presentation/pages/extension_packages_page.dart';
import '../../../features/extensions/presentation/pages/mcp_list_page.dart';
import '../../../features/extensions/presentation/pages/mcp_detail_page.dart';
import '../../../features/extensions/presentation/pages/mcp_edit_page.dart';
import '../../../features/extensions/presentation/pages/agent_skills_page.dart';
import '../../../features/extensions/presentation/pages/extension_page_host_page.dart';
import '../../../features/game_center/presentation/pages/game_center_page.dart';
import '../../../features/game_center/presentation/pages/plugin_detail_page.dart'
    as game_center;
import '../../../features/desktop_pet/presentation/pages/desktop_pet_page.dart';
import '../../../features/workshop/presentation/pages/workshop_home_page.dart';
import '../../../features/workshop/presentation/pages/workflow_list_page.dart';
import '../../../features/workshop/presentation/pages/workflow_editor_page.dart';
import '../../../features/workshop/presentation/pages/pet_center_page.dart';
import '../../../features/workshop/presentation/pages/pet_installations_page.dart';
import '../../../features/dashboard/presentation/pages/dashboard_page.dart';
import '../../../features/characters/presentation/pages/character_create_page.dart';
import '../../../features/settings/presentation/pages/settings_page.dart';
import '../../../features/tasks/presentation/pages/task_hub_page.dart';
import '../../../features/memory/presentation/pages/memory_hub_page.dart';
import '../../../features/profile/presentation/pages/profile_hub_page.dart';
import '../../../features/settings/presentation/pages/model_settings_page.dart';
import '../../../features/settings/presentation/pages/search_api_settings_page.dart';
import '../../../features/settings/presentation/pages/appearance_settings_page.dart';
import '../../../features/runtime/presentation/pages/runtime_page.dart';
import '../../../features/settings/presentation/pages/long_running_page.dart';
import '../../../features/settings/presentation/pages/advanced_system_page.dart';
import '../../../features/settings/presentation/pages/decision_viz_page.dart';
import '../../../features/permissions/presentation/pages/permissions_page.dart';
import '../../../features/permissions/presentation/pages/android_automation_page.dart';
import '../../../features/settings/presentation/pages/backup_page.dart';
import '../../../features/settings/presentation/pages/asr_page.dart';
import '../../../features/settings/presentation/pages/deployment_page.dart';
import '../../../features/settings/presentation/pages/system_settings_page.dart';
import '../../../features/settings/presentation/pages/temporal_settings_page.dart';
import '../../../features/settings/presentation/pages/model_config_page.dart';
import '../../../features/settings/presentation/pages/safety_page.dart';
import '../../../features/settings/presentation/pages/maintenance_page.dart';
import '../../../features/settings/presentation/pages/user_feedback_page.dart';
import '../../../features/settings/presentation/pages/storage_page.dart';
import '../../../features/settings/presentation/pages/user_settings_page.dart';
import '../../../features/settings/presentation/pages/devices_page.dart';
import '../../../features/settings/presentation/pages/device_add_page.dart';
import '../../../features/settings/presentation/pages/device_settings_page.dart';
import '../../../features/settings/presentation/pages/privacy_scan_page.dart';
import '../../../features/settings/presentation/pages/privacy_policy_page.dart';
import '../../../features/settings/presentation/pages/user_agreement_page.dart';
import '../../../features/settings/presentation/pages/app_update_page.dart';
import '../../../features/settings/presentation/pages/about_page_new.dart';
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
import '../../../core/widgets/plugin_unavailable_page.dart';
import '../../../core/widgets/main_tab_scaffold.dart';

/// All built-in business routes live in this catalog instead of the app
/// bootstrap router. The router only owns recovery/authentication boundaries,
/// the shell, and extension supplied routes.
List<RouteBase> buildBuiltinBusinessRoutes() => <RouteBase>[
  StatefulShellRoute.indexedStack(
    pageBuilder: (context, state, navigationShell) => AmitiaSlidePage<void>(
      key: state.pageKey,
      rootLevel: true,
      child: MainTabScaffold(navigationShell: navigationShell),
    ),
    branches: <StatefulShellBranch>[
      StatefulShellBranch(
        routes: <RouteBase>[
          GoRoute(
            path: AppRoutes.conversations,
            pageBuilder: (context, state) => amitiaSlidePage(
              context: context,
              state: state,
              rootLevel: true,
              child: const ConversationListPage(),
            ),
          ),
        ],
      ),
      StatefulShellBranch(
        routes: <RouteBase>[
          GoRoute(
            path: AppRoutes.tasks,
            pageBuilder: (context, state) => amitiaSlidePage(
              context: context,
              state: state,
              rootLevel: true,
              child: const TaskHubPage(),
            ),
          ),
        ],
      ),
      StatefulShellBranch(
        routes: <RouteBase>[
          GoRoute(
            path: AppRoutes.memoryHub,
            pageBuilder: (context, state) => amitiaSlidePage(
              context: context,
              state: state,
              rootLevel: true,
              child: const MemoryHubPage(),
            ),
          ),
        ],
      ),
      StatefulShellBranch(
        routes: <RouteBase>[
          GoRoute(
            path: AppRoutes.extensions,
            pageBuilder: (context, state) => amitiaSlidePage(
              context: context,
              state: state,
              rootLevel: true,
              child: const ExtensionCenterPage(),
            ),
          ),
        ],
      ),
      StatefulShellBranch(
        routes: <RouteBase>[
          GoRoute(
            path: AppRoutes.me,
            pageBuilder: (context, state) => amitiaSlidePage(
              context: context,
              state: state,
              rootLevel: true,
              child: const ProfileHubPage(),
            ),
          ),
        ],
      ),
    ],
  ),
  GoRoute(
    path: AppRoutes.conversationSearch,
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ConversationSearchPage(),
    ),
  ),
  GoRoute(
    path: '/settings/category/:categoryId',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: SettingsCategoryPage(
        categoryId: state.pathParameters['categoryId']!,
      ),
    ),
  ),
  GoRoute(
    path: '/chat',
    pageBuilder: (context, state) => amitiaSlidePage(
      state: state,
      child: ChatPage(
        initialConversationId: state.uri.queryParameters['conversationId'],
        initialCharacterId: state.uri.queryParameters['characterId'],
        initialProjectId: state.uri.queryParameters['projectId'],
      ),
    ),
  ),
  GoRoute(
    path: '/call/:conversationId',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: RealtimeCallPage(
        conversationId: state.pathParameters['conversationId']!,
        characterName: state.uri.queryParameters['caller'] ?? 'Amitia',
        initialMode: switch (state.uri.queryParameters['type']) {
          'video' => RealtimeCallMode.video,
          'screen' => RealtimeCallMode.screen,
          _ => RealtimeCallMode.voice,
        },
        incomingCallId: state.uri.queryParameters['call'] ?? '',
        initialAction: state.uri.queryParameters['action'] ?? '',
        characterId: state.uri.queryParameters['characterId'] ?? '',
        ownedInvitation: state.uri.queryParameters['owned'] == '1',
      ),
    ),
  ),
  GoRoute(path: '/dashboard', redirect: (_, _) => AppRoutes.settingsOverview),
  GoRoute(
    path: '/agent',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const AgentPage()),
  ),
  GoRoute(
    path: '/agent/task/:id',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: AgentTaskDetailPage(
        taskId: state.pathParameters['id']!,
        initialRun: state.extra is Map
            ? Map<String, dynamic>.from(state.extra as Map)
            : null,
      ),
    ),
  ),
  GoRoute(
    path: '/characters',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const CharacterListPage()),
  ),
  GoRoute(
    path: '/characters/create',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const CharacterCreatePage(),
    ),
  ),
  GoRoute(
    path: '/characters/:id',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: CharacterDetailPage(characterId: state.pathParameters['id']!),
    ),
  ),
  GoRoute(
    path: '/characters/:id/life-rules',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const PluginUnavailablePage(
        title: '生活',
        description: '请安装并启用角色生活插件后管理生活规则、日程和状态。',
        fallbackRoute: AppRoutes.characters,
      ),
    ),
  ),
  GoRoute(
    path: '/characters/:id/voice',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: CharacterVoicePage(characterId: state.pathParameters['id']!),
    ),
  ),
  GoRoute(
    path: '/characters/:id/memory',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: CharacterMemoryPage(characterId: state.pathParameters['id']!),
    ),
  ),
  GoRoute(
    path: '/characters/:id/timeline',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: CharacterTimelinePage(characterId: state.pathParameters['id']!),
    ),
  ),
  GoRoute(
    path: '/characters/:id/active-task',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: RemindersPage(
        initialCharacterId: state.pathParameters['id'] ?? '',
      ),
    ),
  ),
  GoRoute(
    path: '/characters/:id/proactive',
    redirect: (context, state) =>
        '/characters/${state.pathParameters['id']}/active-task',
  ),
  GoRoute(
    path: '/characters/:id/psyche',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: CharacterPsychePage(characterId: state.pathParameters['id']!),
    ),
  ),
  GoRoute(
    path: '/memory',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const MemoryOverviewPage(),
    ),
  ),
  GoRoute(
    path: '/memory/episodic',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const EpisodicMemoryPage(),
    ),
  ),
  GoRoute(
    path: '/memory/graph',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const MemoryGraphPage(),
    ),
  ),
  GoRoute(
    path: '/memory/timeline',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const MemoryTimelinePage(),
    ),
  ),
  GoRoute(
    path: '/memory/profiles',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const UserProfilesPage(),
    ),
  ),
  GoRoute(
    path: AppRoutes.knowledge,
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const KnowledgePage()),
  ),
  GoRoute(
    path: '/memory/world-book',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const WorldBookPage(),
    ),
  ),
  GoRoute(
    path: '/reminders',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const RemindersPage()),
  ),
  GoRoute(
    path: '/continuity',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const ContinuityPage()),
  ),
  GoRoute(
    path: '/continuity/:id',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: ContinuityDetailPage(
        threadId: state.pathParameters['id']!,
        initialDocument: state.extra is Map
            ? Map<String, dynamic>.from(state.extra as Map)
            : null,
      ),
    ),
  ),
  GoRoute(
    path: AppRoutes.channelMessages,
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ChannelMessagesPage(),
    ),
  ),
  GoRoute(
    path: '/emotes',
    pageBuilder: (context, state) => amitiaSlidePage(
      state: state,
      child: const PluginUnavailablePage(
        title: '表情包',
        description: '请安装并启用表情包插件后管理表情、分组和 AI 使用策略。',
        fallbackRoute: AppRoutes.chat,
      ),
    ),
  ),
  GoRoute(
    path: '/chat-logs',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const ChatLogsPage()),
  ),
  GoRoute(
    path: '/chat-import',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const ChatImportPage()),
  ),
  GoRoute(
    path: '/extensions/packages',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ExtensionPackagesPage(),
    ),
  ),
  GoRoute(
    path: '/extensions/mcp',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const McpListPage(),
    ),
  ),
  GoRoute(
    path: '/extensions/mcp/new',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const McpEditPage(mcpId: 'new'),
    ),
  ),
  GoRoute(
    path: '/extensions/mcp/:id',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: McpDetailPage(mcpId: state.pathParameters['id']!),
    ),
  ),
  GoRoute(
    path: '/extensions/mcp/:id/edit',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: McpEditPage(mcpId: state.pathParameters['id']!),
    ),
  ),
  GoRoute(
    path: '/extensions/agent-skills',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const AgentSkillsPage(),
    ),
  ),
  GoRoute(
    path: '/extension/page/:pageId',
    pageBuilder: (context, state) => amitiaSlidePage(
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
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const GameCenterPage()),
  ),
  GoRoute(
    path: AppRoutes.gamePlugin,
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: game_center.PluginDetailPage(
        pluginId: state.uri.queryParameters['pluginId'] ?? '',
        extensionId: state.uri.queryParameters['extensionId'] ?? '',
      ),
    ),
  ),
  GoRoute(
    path: '/desktop-pet',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const DesktopPetPage()),
  ),
  GoRoute(
    path: '/workshop',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const WorkshopHomePage()),
  ),
  GoRoute(
    path: '/workshop/character-cards',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const CharacterCardWorkshopPage(),
    ),
  ),
  GoRoute(
    path: '/workshop/workflows',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const WorkflowListPage(),
    ),
  ),
  GoRoute(
    path: '/workshop/workflows/:id',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: WorkflowEditorPage(
        workflowId: state.pathParameters['id']!,
        location: state.uri.queryParameters['location'] ?? 'cloud',
        deviceId: state.uri.queryParameters['deviceId'] ?? '',
      ),
    ),
  ),
  GoRoute(
    path: '/workshop/pet',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const PetCenterPage(),
    ),
  ),
  GoRoute(
    path: '/workshop/pet/installations',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: PetInstallationsPage(
        initialReleaseId: state.uri.queryParameters['releaseId'] ?? '',
      ),
    ),
  ),
  GoRoute(
    path: '/settings',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const SettingsPage()),
  ),
  GoRoute(
    path: '/settings/overview',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const DashboardPage(initialTab: 0),
    ),
  ),
  GoRoute(
    path: '/settings/data',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const DashboardPage(initialTab: 1),
    ),
  ),
  GoRoute(
    path: '/settings/models',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ModelSettingsPage(),
    ),
  ),
  GoRoute(
    path: '/settings/models/:modelType',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: ModelConfigPage(modelType: state.pathParameters['modelType']!),
    ),
  ),
  GoRoute(
    path: AppRoutes.settingsSearchApi,
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const SearchApiSettingsPage(),
    ),
  ),
  GoRoute(
    path: '/settings/appearance',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const AppearanceSettingsPage(),
    ),
  ),
  GoRoute(
    path: AppRoutes.settingsNotifications,
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const NotificationSettingsPage(),
    ),
  ),
  GoRoute(
    path: '/settings/runtime',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const RuntimePage(),
    ),
  ),
  GoRoute(
    path: '/settings/permissions',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const PermissionsPage(),
    ),
  ),
  GoRoute(
    path: '/settings/android-automation',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const AndroidAutomationPage(),
    ),
  ),
  GoRoute(
    path: '/settings/backup',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const BackupPage(),
    ),
  ),
  GoRoute(
    path: '/settings/asr',
    pageBuilder: (context, state) =>
        amitiaSlidePage(context: context, state: state, child: const AsrPage()),
  ),
  GoRoute(
    path: '/settings/deployment',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const DeploymentPage(),
    ),
  ),
  GoRoute(
    path: '/settings/system',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const SystemSettingsPage(),
    ),
  ),
  GoRoute(
    path: '/settings/temporal',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const TemporalSettingsPage(),
    ),
  ),
  GoRoute(
    path: '/settings/safety',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const SafetyPage(),
    ),
  ),
  GoRoute(
    path: '/settings/maintenance',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const MaintenancePage(),
    ),
  ),
  GoRoute(
    path: '/settings/user-feedback',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const UserFeedbackPage(),
    ),
  ),
  GoRoute(
    path: '/settings/long-running',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const LongRunningPage(),
    ),
  ),
  GoRoute(
    path: '/settings/advanced',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const AdvancedSystemPage(),
    ),
  ),
  GoRoute(
    path: '/settings/decision-viz',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const DecisionVizPage(),
    ),
  ),
  GoRoute(
    path: '/settings/theme',
    redirect: (_, __) => AppRoutes.settingsAppearance,
  ),
  GoRoute(
    path: '/settings/storage',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const StoragePage(),
    ),
  ),
  GoRoute(
    path: '/settings/user',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const UserSettingsPage(),
    ),
  ),
  GoRoute(
    path: '/settings/devices',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const DevicesPage()),
  ),
  GoRoute(
    path: '/settings/devices/add',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const DeviceAddPage(),
    ),
  ),
  GoRoute(
    path: '/settings/devices/settings',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const DeviceSettingsPage(),
    ),
  ),
  GoRoute(
    path: '/settings/privacy-scan',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const PrivacyScanPage(),
    ),
  ),
  GoRoute(
    path: '/settings/privacy-policy',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const PrivacyPolicyPage(),
    ),
  ),
  GoRoute(
    path: '/settings/user-agreement',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const UserAgreementPage(),
    ),
  ),
  GoRoute(
    path: '/settings/app-update',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const AppUpdatePage(),
    ),
  ),
  GoRoute(
    path: '/settings/about',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const AboutPageNew(),
    ),
  ),
  GoRoute(
    path: '/settings/toolbox',
    redirect: (context, state) => AppRoutes.settingsMaintenanceCategory,
  ),
  GoRoute(
    path: '/settings/toolbox/file-browser',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ToolboxFileBrowserPage(),
    ),
  ),
  GoRoute(
    path: '/settings/toolbox/workspace',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ToolboxWorkspacePage(),
    ),
  ),
  GoRoute(
    path: '/settings/toolbox/task-log',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ToolboxTaskLogPage(),
    ),
  ),
  GoRoute(
    path: '/settings/toolbox/log',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ToolboxLogPage(),
    ),
  ),
  GoRoute(
    path: '/settings/toolbox/prompt-trace',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ToolboxPromptTracePage(),
    ),
  ),
  GoRoute(
    path: '/settings/toolbox/runtime-status',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ToolboxRuntimeStatusPage(),
    ),
  ),
  GoRoute(
    path: '/settings/toolbox/database-status',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ToolboxDatabaseStatusPage(),
    ),
  ),
  GoRoute(
    path: '/settings/toolbox/device-status',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ToolboxDeviceStatusPage(),
    ),
  ),
  GoRoute(
    path: '/developer',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const DeveloperHomePage()),
  ),
  GoRoute(
    path: '/developer/kernel',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const KernelHomePage(),
    ),
  ),
  GoRoute(
    path: '/developer/kernel/wasm',
    pageBuilder: (context, state) =>
        amitiaSlidePage(context: context, state: state, child: const WasmPage()),
  ),
  GoRoute(
    path: '/developer/kernel/hooks',
    pageBuilder: (context, state) =>
        amitiaSlidePage(context: context, state: state, child: const HooksPage()),
  ),
  GoRoute(
    path: '/developer/kernel/trusted-services',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const TrustedServicesPage(),
    ),
  ),
  GoRoute(
    path: '/developer/kernel/tasks',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const KernelTasksPage(),
    ),
  ),
  GoRoute(
    path: '/developer/kernel/events',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const EventsPage(),
    ),
  ),
  GoRoute(
    path: '/developer/kernel/schedules',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const SchedulesPage(),
    ),
  ),
  GoRoute(
    path: '/developer/kernel/desktop',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const DesktopContributionsPage(),
    ),
  ),
  GoRoute(
    path: '/developer/kernel/updates',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const UpdatesPage(),
    ),
  ),
  GoRoute(
    path: '/developer/kernel/dev-console',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const DevConsolePage(),
    ),
  ),
  GoRoute(
    path: '/developer/kernel/migrations',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const MigrationsPage(),
    ),
  ),
  GoRoute(
    path: '/developer/kernel/dev-mode',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const DevModePage(),
    ),
  ),
];
