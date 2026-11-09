import 'package:flutter/material.dart';

import '../../features/characters/presentation/pages/character_create_page.dart';
import '../../features/characters/presentation/pages/character_detail_page.dart';
import '../../features/characters/presentation/pages/character_list_page.dart';
import '../../features/characters/presentation/pages/character_psyche_page.dart';
import '../../features/characters/presentation/pages/character_teams_page.dart';
import '../../features/characters/presentation/pages/character_voice_page.dart';
import '../../features/memory/presentation/pages/world_book_page.dart';
import '../../features/memory/presentation/pages/episodic_memory_page.dart';
import '../../features/memory/presentation/pages/memory_graph_page.dart';
import '../../features/memory/presentation/pages/memory_overview_page.dart';
import '../../features/memory/presentation/pages/memory_timeline_page.dart';
import '../../features/memory/presentation/pages/user_profiles_page.dart';
import '../../features/knowledge/presentation/pages/knowledge_page.dart';
import '../../features/profile/presentation/pages/character_world_book_hub_page.dart';
import '../../features/profile/presentation/pages/profile_hub_page.dart';
import '../../features/tasks/presentation/pages/task_hub_page.dart';
import '../../features/memory/presentation/pages/memory_hub_page.dart';
import '../../features/extensions/presentation/pages/extension_center_page.dart';
import '../../features/characters/presentation/pages/character_card_workshop_page.dart';
import '../../features/workshop/presentation/pages/pet_center_page.dart';
import '../../features/workshop/presentation/pages/pet_installations_page.dart';
import '../../features/workshop/presentation/pages/workshop_home_page.dart';
import '../../features/game_center/presentation/pages/game_center_page.dart';
import '../../features/game_center/presentation/pages/plugin_detail_page.dart'
    as game_center;
import '../../features/extensions/presentation/pages/extension_packages_page.dart';
import '../../features/extensions/presentation/pages/mcp_list_page.dart';
import '../../features/extensions/presentation/pages/mcp_detail_page.dart';
import '../../features/extensions/presentation/pages/mcp_edit_page.dart';
import '../../features/extensions/presentation/pages/agent_skills_page.dart';
import '../../features/workshop/presentation/pages/workflow_editor_page.dart';
import '../../features/workshop/presentation/pages/workflow_list_page.dart';
import '../../features/chat_logs/presentation/pages/chat_logs_page.dart';
import '../../features/chat_import/presentation/pages/chat_import_page.dart';
import '../../features/developer/presentation/pages/developer_home_page.dart';
import '../../features/developer/presentation/pages/kernel_home_page.dart';
import '../../features/developer/presentation/pages/wasm_page.dart';
import '../../features/developer/presentation/pages/hooks_page.dart';
import '../../features/developer/presentation/pages/trusted_services_page.dart';
import '../../features/developer/presentation/pages/kernel_tasks_page.dart';
import '../../features/developer/presentation/pages/events_page.dart';
import '../../features/developer/presentation/pages/schedules_page.dart';
import '../../features/developer/presentation/pages/desktop_contributions_page.dart';
import '../../features/developer/presentation/pages/updates_page.dart';
import '../../features/developer/presentation/pages/dev_console_page.dart';
import '../../features/developer/presentation/pages/dev_mode_page.dart';
import '../../features/desktop_pet/presentation/pages/desktop_pet_page.dart';
import '../../features/channels/presentation/pages/channel_messages_page.dart';

class MobileNativePageHost extends StatelessWidget {
  const MobileNativePageHost({super.key, required this.context});

  final Map<String, dynamic> context;

  String get _path {
    final route = (context['route'] ?? '').toString();
    return Uri.tryParse(route)?.path ?? route;
  }

  String _parameter(String name) {
    final parameters = context['pathParameters'];
    if (parameters is Map) return (parameters[name] ?? '').toString();
    final segments = _path.split('/').where((segment) => segment.isNotEmpty);
    if (name == 'id' && segments.length >= 2) return segments.elementAt(1);
    return '';
  }

  Widget _view(String path) {
    if (path == '/tasks') {
      return const TaskHubPage();
    }
    if (path == '/memory-hub') {
      return const MemoryHubPage();
    }
    if (path == '/extensions') {
      return const ExtensionCenterPage();
    }
    if (path == '/me') {
      return const ProfileHubPage();
    }
    if (path == '/characters') {
      return const CharacterListPage();
    }
    if (path == '/characters/create') {
      return const CharacterCreatePage();
    }
    if (path == '/characters/teams') {
      return const CharacterTeamsPage();
    }
    if (path == '/memory/world-book') {
      return const WorldBookPage();
    }
    if (path == '/memory' || path == '/memory-manager') {
      return const MemoryOverviewPage();
    }
    if (path == '/memory/episodic' || path == '/episodic') {
      return const EpisodicMemoryPage();
    }
    if (path == '/memory/graph' || path == '/graph') {
      return const MemoryGraphPage();
    }
    if (path == '/memory/timeline' || path == '/memory-timeline') {
      return const MemoryTimelinePage();
    }
    if (path == '/memory/profiles' || path == '/profiles') {
      return const UserProfilesPage();
    }
    if (path == '/memory/knowledge' || path == '/knowledge') {
      return const KnowledgePage();
    }
    if (path == '/me/characters') {
      return const CharacterWorldBookHubPage();
    }
    if (path == '/workshop') {
      return const WorkshopHomePage();
    }
    if (path == '/workshop/character-cards') {
      return const CharacterCardWorkshopPage();
    }
    if (path == '/workshop/pet') {
      return const PetCenterPage();
    }
    if (path == '/workshop/pet/installations') {
      final query = context['queryParameters'];
      final releaseId = query is Map
          ? (query['releaseId'] ?? '').toString()
          : '';
      return PetInstallationsPage(initialReleaseId: releaseId);
    }
    if (path == '/desktop-pet') {
      return const DesktopPetPage();
    }
    if (path == '/channel-center') {
      return const ChannelMessagesPage();
    }
    if (path == '/game-center') {
      return const GameCenterPage();
    }
    if (path == '/game-center/plugin') {
      final query = context['queryParameters'];
      return game_center.PluginDetailPage(
        pluginId: query is Map ? (query['pluginId'] ?? '').toString() : '',
        extensionId: query is Map
            ? (query['extensionId'] ?? '').toString()
            : '',
      );
    }
    if (path == '/extensions/packages') {
      return const ExtensionPackagesPage();
    }
    if (path == '/extensions/mcp') {
      return const McpListPage();
    }
    if (path == '/extensions/mcp/new') {
      return const McpEditPage(mcpId: 'new');
    }
    if (path.endsWith('/edit') && path.startsWith('/extensions/mcp/')) {
      final id = path.split('/')[3];
      return McpEditPage(mcpId: id);
    }
    if (path.startsWith('/extensions/mcp/')) {
      final id = path.split('/')[3];
      return McpDetailPage(mcpId: id);
    }
    if (path == '/extensions/agent-skills') {
      return const AgentSkillsPage();
    }
    if (path == '/workshop/workflows') {
      return const WorkflowListPage();
    }
    if (path.startsWith('/workshop/workflows/')) {
      final query = context['queryParameters'];
      return WorkflowEditorPage(
        workflowId: path.split('/').last,
        location: query is Map
            ? (query['location'] ?? 'cloud').toString()
            : 'cloud',
        deviceId: query is Map ? (query['deviceId'] ?? '').toString() : '',
      );
    }
    if (path == '/chat-logs') {
      return const ChatLogsPage();
    }
    if (path == '/chat-import') {
      return const ChatImportPage();
    }
    if (path == '/developer') {
      return const DeveloperHomePage();
    }
    if (path == '/developer/kernel') {
      return const KernelHomePage();
    }
    if (path == '/developer/kernel/wasm') {
      return const WasmPage();
    }
    if (path == '/developer/kernel/hooks') {
      return const HooksPage();
    }
    if (path == '/developer/kernel/trusted-services') {
      return const TrustedServicesPage();
    }
    if (path == '/developer/kernel/tasks') {
      return const KernelTasksPage();
    }
    if (path == '/developer/kernel/events') {
      return const EventsPage();
    }
    if (path == '/developer/kernel/schedules') {
      return const SchedulesPage();
    }
    if (path == '/developer/kernel/desktop') {
      return const DesktopContributionsPage();
    }
    if (path == '/developer/kernel/updates') {
      return const UpdatesPage();
    }
    if (path == '/developer/kernel/dev-console') {
      return const DevConsolePage();
    }
    if (path == '/developer/kernel/dev-mode') {
      return const DevModePage();
    }
    if (path.endsWith('/voice')) {
      return CharacterVoicePage(characterId: _parameter('id'));
    }
    if (path.endsWith('/psyche')) {
      return CharacterPsychePage(characterId: _parameter('id'));
    }
    if (path.startsWith('/characters/')) {
      return CharacterDetailPage(characterId: _parameter('id'));
    }
    return const CharacterDetailPage(characterId: '');
  }

  @override
  Widget build(BuildContext context) => _view(_path);
}
