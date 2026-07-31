import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/widgets/amitia_drawer.dart';
import '../features/chat/presentation/pages/chat_page.dart';
import '../features/conversations/presentation/pages/conversation_list_page.dart';
import '../features/agent/presentation/pages/agent_page.dart';
import '../features/agent/presentation/pages/agent_task_detail_page.dart';
import '../features/characters/presentation/pages/character_list_page.dart';
import '../features/characters/presentation/pages/character_detail_page.dart';
import '../features/characters/presentation/pages/character_life_rules_page.dart';
import '../features/characters/presentation/pages/character_voice_page.dart';
import '../features/characters/presentation/pages/character_memory_page.dart';
import '../features/characters/presentation/pages/character_timeline_page.dart';
import '../features/characters/presentation/pages/character_proactive_page.dart';
import '../features/characters/presentation/pages/character_psyche_page.dart';
import '../features/characters/presentation/pages/character_debug_page.dart';
import '../features/memory/presentation/pages/memory_page.dart';
import '../features/memory/presentation/pages/memory_manager_page.dart';
import '../features/memory/presentation/pages/episodic_memory_page.dart';
import '../features/memory/presentation/pages/memory_graph_page.dart';
import '../features/memory/presentation/pages/memory_timeline_page.dart';
import '../features/memory/presentation/pages/user_profiles_page.dart';
import '../features/memory/presentation/pages/world_book_page.dart';
import '../features/reminders/presentation/pages/reminders_page.dart';
import '../features/emotes/presentation/pages/emotes_page.dart';
import '../features/chat_logs/presentation/pages/chat_logs_page.dart';
import '../features/chat_import/presentation/pages/chat_import_page.dart';
import '../features/extensions/presentation/pages/extension_center_page.dart';
import '../features/extensions/presentation/pages/extension_packages_page.dart';
import '../features/extensions/presentation/pages/mcp_list_page.dart';
import '../features/extensions/presentation/pages/mcp_detail_page.dart';
import '../features/extensions/presentation/pages/mcp_edit_page.dart';
import '../features/extensions/presentation/pages/agent_skills_page.dart';
import '../features/extensions/presentation/pages/system_plugins_page.dart';
import '../features/extensions/presentation/pages/compatible_skills_page.dart';
import '../features/extensions/presentation/pages/execution_runs_page.dart';
import '../features/extensions/presentation/pages/extension_run_detail_page.dart';
import '../features/extensions/presentation/pages/extension_page_host_page.dart';
import '../features/extensions/presentation/pages/skill_detail_page.dart';
import '../features/extensions/presentation/pages/plugin_detail_page.dart';
import '../features/game_center/presentation/pages/game_center_page.dart';
import '../features/desktop_pet/presentation/pages/desktop_pet_page.dart';
import '../features/workshop/presentation/pages/workshop_home_page.dart';
import '../features/workshop/presentation/pages/skill_workshop_page.dart';
import '../features/workshop/presentation/pages/skill_draft_editor_page.dart';
import '../features/workshop/presentation/pages/pet_center_page.dart';
import '../features/workshop/presentation/pages/pet_create_page.dart';
import '../features/workshop/presentation/pages/pet_tasks_page.dart';
import '../features/workshop/presentation/pages/pet_processing_page.dart';
import '../features/workshop/presentation/pages/pet_action_editor_page.dart';
import '../features/workshop/presentation/pages/pet_installations_page.dart';
import '../features/dashboard/presentation/pages/dashboard_page.dart';
import '../features/channels/presentation/pages/wechat_page.dart';
import '../features/channels/presentation/pages/qq_page.dart';
import '../features/channels/presentation/pages/channel_center_page.dart';
import '../features/characters/presentation/pages/character_create_page.dart';
import '../features/settings/presentation/pages/settings_page.dart';
import '../features/settings/presentation/pages/model_settings_page.dart';
import '../features/settings/presentation/pages/appearance_settings_page.dart';
import '../features/runtime/presentation/pages/runtime_page.dart';
import '../features/permissions/presentation/pages/permissions_page.dart';
import '../features/settings/presentation/pages/backup_page.dart';
import '../features/settings/presentation/pages/ai_config_page.dart';
import '../features/settings/presentation/pages/deployment_page.dart';
import '../features/settings/presentation/pages/system_settings_page.dart';
import '../features/settings/presentation/pages/temporal_settings_page.dart';
import '../features/settings/presentation/pages/model_config_page.dart';
import '../features/settings/presentation/pages/safety_page.dart';
import '../features/settings/presentation/pages/maintenance_page.dart';
import '../features/settings/presentation/pages/theme_settings_page.dart';
import '../features/settings/presentation/pages/storage_page.dart';
import '../features/settings/presentation/pages/user_settings_page.dart';
import '../features/settings/presentation/pages/privacy_scan_page.dart';
import '../features/settings/presentation/pages/about_page_new.dart';
import '../features/toolbox/presentation/pages/toolbox_page.dart';
import '../features/onboarding/presentation/pages/onboarding_page.dart';
import '../features/auth/presentation/pages/login_page.dart';
import '../features/privacy/presentation/pages/privacy_page.dart';
import '../features/error/presentation/pages/not_found_page.dart';
import '../features/developer/presentation/pages/developer_home_page.dart';
import '../features/developer/presentation/pages/kernel_home_page.dart';
import '../features/developer/presentation/pages/wasm_page.dart';
import '../features/developer/presentation/pages/hooks_page.dart';
import '../features/developer/presentation/pages/trusted_services_page.dart';
import '../features/developer/presentation/pages/kernel_tasks_page.dart';
import '../features/developer/presentation/pages/events_page.dart';
import '../features/developer/presentation/pages/schedules_page.dart';
import '../features/developer/presentation/pages/desktop_contributions_page.dart';
import '../features/developer/presentation/pages/updates_page.dart';
import '../features/developer/presentation/pages/dev_console_page.dart';
import '../features/developer/presentation/pages/migrations_page.dart';
import '../features/developer/presentation/pages/dev_mode_page.dart';

final _shellNavigatorKey = GlobalKey<NavigatorState>(debugLabel: 'shell');

class AppShell extends ConsumerStatefulWidget {
  final Widget child;
  final String currentRoute;

  const AppShell({super.key, required this.child, required this.currentRoute});

  @override
  ConsumerState<AppShell> createState() => _AppShellState();
}

class _AppShellState extends ConsumerState<AppShell> {
  @override
  Widget build(BuildContext context) {
    return Scaffold(
      drawerEnableOpenDragGesture: true,
      drawer: AmitiaDrawer(currentRoute: widget.currentRoute),
      body: widget.child,
    );
  }
}

final goRouterProvider = Provider<GoRouter>((ref) {
  return GoRouter(
    initialLocation: '/chat',
    navigatorKey: _shellNavigatorKey,
    redirect: (context, state) {
      final stage = ref.read(mockStartupStageProvider);
      final location = state.matchedLocation;

      if (location == '/about') return '/settings/about';
      if (location == '/toolbox') return '/settings/toolbox';

      switch (stage) {
        case MockStartupStage.firstLaunch:
          if (location != '/onboarding') return '/onboarding';
        case MockStartupStage.needsLogin:
          if (location != '/login') return '/login';
        case MockStartupStage.privacyRequired:
          if (location != '/privacy') return '/privacy';
        case MockStartupStage.ready:
          if (location == '/onboarding' || location == '/login' || location == '/privacy') return '/chat';
      }

      return null;
    },
    errorBuilder: (context, state) => NotFoundPage(attemptedPath: state.uri.toString()),
    routes: [
      GoRoute(path: '/onboarding', builder: (context, state) => const OnboardingPage()),
      GoRoute(path: '/login', builder: (context, state) => const LoginPage()),
      GoRoute(path: '/privacy', builder: (context, state) => const PrivacyPage()),
      ShellRoute(
        builder: (context, state, child) {
          return AppShell(currentRoute: state.matchedLocation, child: child);
        },
        routes: [
          GoRoute(path: '/chat', builder: (context, state) => const ChatPage()),
          GoRoute(path: '/conversations', builder: (context, state) => const ConversationListPage()),
          GoRoute(path: '/dashboard', builder: (context, state) => const DashboardPage()),
          GoRoute(path: '/channels', builder: (context, state) => const ChannelCenterPage()),
          GoRoute(path: '/channels/wechat', builder: (context, state) => const WechatPage()),
          GoRoute(path: '/channels/qq', builder: (context, state) => const QqPage()),
          GoRoute(path: '/agent', builder: (context, state) => const AgentPage()),
          GoRoute(path: '/agent/task/:id', builder: (context, state) => AgentTaskDetailPage(taskId: state.pathParameters['id']!)),
          GoRoute(path: '/characters', builder: (context, state) => const CharacterListPage()),
          GoRoute(path: '/characters/create', builder: (context, state) => const CharacterCreatePage()),
          GoRoute(path: '/characters/:id', builder: (context, state) => CharacterDetailPage(characterId: state.pathParameters['id']!)),
          GoRoute(path: '/characters/:id/life-rules', builder: (context, state) => CharacterLifeRulesPage(characterId: state.pathParameters['id']!)),
          GoRoute(path: '/characters/:id/voice', builder: (context, state) => CharacterVoicePage(characterId: state.pathParameters['id']!)),
          GoRoute(path: '/characters/:id/memory', builder: (context, state) => CharacterMemoryPage(characterId: state.pathParameters['id']!)),
          GoRoute(path: '/characters/:id/timeline', builder: (context, state) => CharacterTimelinePage(characterId: state.pathParameters['id']!)),
          GoRoute(path: '/characters/:id/proactive', builder: (context, state) => CharacterProactivePage(characterId: state.pathParameters['id']!)),
          GoRoute(path: '/characters/:id/psyche', builder: (context, state) => CharacterPsychePage(characterId: state.pathParameters['id']!)),
          GoRoute(path: '/characters/:id/debug', builder: (context, state) => CharacterDebugPage(characterId: state.pathParameters['id']!)),
          GoRoute(path: '/memory', builder: (context, state) => const MemoryPage()),
          GoRoute(path: '/memory/manager', builder: (context, state) => const MemoryManagerPage()),
          GoRoute(path: '/memory/episodic', builder: (context, state) => const EpisodicMemoryPage()),
          GoRoute(path: '/memory/graph', builder: (context, state) => const MemoryGraphPage()),
          GoRoute(path: '/memory/timeline', builder: (context, state) => const MemoryTimelinePage()),
          GoRoute(path: '/memory/profiles', builder: (context, state) => const UserProfilesPage()),
          GoRoute(path: '/memory/world-book', builder: (context, state) => const WorldBookPage()),
          GoRoute(path: '/reminders', builder: (context, state) => const RemindersPage()),
          GoRoute(path: '/emotes', builder: (context, state) => const EmotesPage()),
          GoRoute(path: '/chat-logs', builder: (context, state) => const ChatLogsPage()),
          GoRoute(path: '/chat-import', builder: (context, state) => const ChatImportPage()),
          GoRoute(path: '/extensions', builder: (context, state) => const ExtensionCenterPage()),
          GoRoute(path: '/extensions/packages', builder: (context, state) => const ExtensionPackagesPage()),
          GoRoute(path: '/extensions/mcp', builder: (context, state) => const McpListPage()),
          GoRoute(path: '/extensions/mcp/new', builder: (context, state) => const McpEditPage(mcpId: 'new')),
          GoRoute(path: '/extensions/mcp/:id', builder: (context, state) => McpDetailPage(mcpId: state.pathParameters['id']!)),
          GoRoute(path: '/extensions/mcp/:id/edit', builder: (context, state) => McpEditPage(mcpId: state.pathParameters['id']!)),
          GoRoute(path: '/extensions/agent-skills', builder: (context, state) => const AgentSkillsPage()),
          GoRoute(path: '/extensions/skills', builder: (context, state) => const CompatibleSkillsPage()),
          GoRoute(path: '/extensions/skills/:id', builder: (context, state) => SkillDetailPage(skillId: state.pathParameters['id']!)),
          GoRoute(path: '/extensions/plugins', builder: (context, state) => const SystemPluginsPage()),
          GoRoute(path: '/extensions/plugins/:id', builder: (context, state) => PluginDetailPage(pluginId: state.pathParameters['id']!)),
          GoRoute(path: '/extensions/runs', builder: (context, state) => const ExecutionRunsPage()),
          GoRoute(path: '/extensions/runs/:id', builder: (context, state) => ExtensionRunDetailPage(runId: state.pathParameters['id']!)),
          GoRoute(path: '/extension/page/:pageId', builder: (context, state) => ExtensionPageHostPage(pageId: state.pathParameters['pageId']!)),
          GoRoute(path: '/game-center', builder: (context, state) => const GameCenterPage()),
          GoRoute(path: '/desktop-pet', builder: (context, state) => const DesktopPetPage()),
          GoRoute(path: '/workshop', builder: (context, state) => const WorkshopHomePage()),
          GoRoute(path: '/workshop/skills', builder: (context, state) => const SkillWorkshopPage()),
          GoRoute(path: '/workshop/skills/:id/editor', builder: (context, state) => SkillDraftEditorPage(draftId: state.pathParameters['id']!)),
          GoRoute(path: '/workshop/pet', builder: (context, state) => const PetCenterPage()),
          GoRoute(path: '/workshop/pet/create', builder: (context, state) => const PetCreatePage()),
          GoRoute(path: '/workshop/pet/tasks', builder: (context, state) => const PetTasksPage()),
          GoRoute(path: '/workshop/pet/processing/:taskId', builder: (context, state) => PetProcessingPage(taskId: state.pathParameters['taskId']!)),
          GoRoute(path: '/workshop/pet/processing/:taskId/actions/:actionKey/editor', builder: (context, state) => PetActionEditorPage(taskId: state.pathParameters['taskId']!, actionKey: state.pathParameters['actionKey']!)),
          GoRoute(path: '/workshop/pet/installations', builder: (context, state) => const PetInstallationsPage()),
          GoRoute(path: '/settings', builder: (context, state) => const SettingsPage()),
          GoRoute(path: '/settings/models', builder: (context, state) => const ModelSettingsPage()),
          GoRoute(path: '/settings/models/:modelType', builder: (context, state) => ModelConfigPage(modelType: state.pathParameters['modelType']!)),
          GoRoute(path: '/settings/appearance', builder: (context, state) => const AppearanceSettingsPage()),
          GoRoute(path: '/settings/runtime', builder: (context, state) => const RuntimePage()),
          GoRoute(path: '/settings/permissions', builder: (context, state) => const PermissionsPage()),
          GoRoute(path: '/settings/backup', builder: (context, state) => const BackupPage()),
          GoRoute(path: '/settings/ai', builder: (context, state) => const AiConfigPage()),
          GoRoute(path: '/settings/deployment', builder: (context, state) => const DeploymentPage()),
          GoRoute(path: '/settings/system', builder: (context, state) => const SystemSettingsPage()),
          GoRoute(path: '/settings/temporal', builder: (context, state) => const TemporalSettingsPage()),
          GoRoute(path: '/settings/safety', builder: (context, state) => const SafetyPage()),
          GoRoute(path: '/settings/maintenance', builder: (context, state) => const MaintenancePage()),
          GoRoute(path: '/settings/theme', builder: (context, state) => const ThemeSettingsPage()),
          GoRoute(path: '/settings/storage', builder: (context, state) => const StoragePage()),
          GoRoute(path: '/settings/user', builder: (context, state) => const UserSettingsPage()),
          GoRoute(path: '/settings/privacy-scan', builder: (context, state) => const PrivacyScanPage()),
          GoRoute(path: '/settings/about', builder: (context, state) => const AboutPageNew()),
          GoRoute(path: '/settings/toolbox', builder: (context, state) => const ToolboxPage()),
          GoRoute(path: '/developer', builder: (context, state) => const DeveloperHomePage()),
          GoRoute(path: '/developer/kernel', builder: (context, state) => const KernelHomePage()),
          GoRoute(path: '/developer/kernel/wasm', builder: (context, state) => const WasmPage()),
          GoRoute(path: '/developer/kernel/hooks', builder: (context, state) => const HooksPage()),
          GoRoute(path: '/developer/kernel/trusted-services', builder: (context, state) => const TrustedServicesPage()),
          GoRoute(path: '/developer/kernel/tasks', builder: (context, state) => const KernelTasksPage()),
          GoRoute(path: '/developer/kernel/events', builder: (context, state) => const EventsPage()),
          GoRoute(path: '/developer/kernel/schedules', builder: (context, state) => const SchedulesPage()),
          GoRoute(path: '/developer/kernel/desktop', builder: (context, state) => const DesktopContributionsPage()),
          GoRoute(path: '/developer/kernel/updates', builder: (context, state) => const UpdatesPage()),
          GoRoute(path: '/developer/kernel/dev-console', builder: (context, state) => const DevConsolePage()),
          GoRoute(path: '/developer/kernel/migrations', builder: (context, state) => const MigrationsPage()),
          GoRoute(path: '/developer/kernel/dev-mode', builder: (context, state) => const DevModePage()),
        ],
      ),
    ],
  );
});
