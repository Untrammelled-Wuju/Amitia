import 'package:flutter/material.dart';

import '../../features/characters/presentation/pages/character_card_workshop_page.dart';
import '../../features/characters/presentation/pages/character_detail_page.dart';
import '../../features/characters/presentation/pages/character_teams_page.dart';
import '../../features/continuity/presentation/pages/continuity_page.dart';
import '../../features/continuity/presentation/pages/continuity_detail_page.dart';
import '../../features/extensions/presentation/pages/agent_skills_page.dart';
import '../../features/extensions/presentation/pages/extension_packages_page.dart';
import '../../features/extensions/presentation/pages/mcp_list_page.dart';
import '../../features/game_center/presentation/pages/plugin_detail_page.dart'
    as game_center;
import '../../features/knowledge/presentation/pages/knowledge_page.dart';
import '../../features/memory/presentation/pages/episodic_memory_page.dart';
import '../../features/memory/presentation/pages/memory_graph_page.dart';
import '../../features/memory/presentation/pages/memory_overview_page.dart';
import '../../features/memory/presentation/pages/memory_timeline_page.dart';
import '../../features/memory/presentation/pages/user_profiles_page.dart';
import '../../features/memory/presentation/pages/world_book_page.dart';
import '../../features/settings/presentation/pages/mobile_user_space_host.dart';
import '../../features/workshop/presentation/pages/pet_installations_page.dart';
import '../../features/workshop/presentation/pages/workflow_list_page.dart';
import 'mobile_native_page_host.dart';

typedef MobileHostRuntimeBuilder =
    Widget Function(Map<String, dynamic> context);

final Map<String, MobileHostRuntimeBuilder> _hostRuntimes =
    <String, MobileHostRuntimeBuilder>{};
bool _builtinMobileHostRuntimesRegistered = false;

void registerMobileHostRuntime(
  String runtimeId,
  MobileHostRuntimeBuilder builder,
) {
  final key = runtimeId.trim();
  if (key.isEmpty) return;
  _hostRuntimes[key] = builder;
}

MobileHostRuntimeBuilder? resolveMobileHostRuntime(String runtimeId) {
  _ensureBuiltinMobileHostRuntimesRegistered();
  return _hostRuntimes[runtimeId.trim()];
}

String _parameter(Map<String, dynamic> context, String name) {
  final parameters = context['pathParameters'];
  if (parameters is Map) {
    return (parameters[name] ?? '').toString();
  }
  return '';
}

String _query(Map<String, dynamic> context, String name) {
  final parameters = context['queryParameters'];
  if (parameters is Map) {
    return (parameters[name] ?? '').toString();
  }
  return '';
}

void _registerBuiltinMobileHostRuntimes() {
  registerMobileHostRuntime(
    'host.character.detail',
    (context) => CharacterDetailPage(characterId: _parameter(context, 'id')),
  );
  registerMobileHostRuntime(
    'host.character.teams',
    (_) => const CharacterTeamsPage(),
  );
  registerMobileHostRuntime(
    'host.character.worldbook',
    (_) => const WorldBookPage(),
  );
  registerMobileHostRuntime(
    'host.creative.character-cards',
    (_) => const CharacterCardWorkshopPage(),
  );
  registerMobileHostRuntime(
    'host.creative.pet-installations',
    (context) =>
        PetInstallationsPage(initialReleaseId: _query(context, 'releaseId')),
  );
  registerMobileHostRuntime(
    'host.game.plugin',
    (context) => game_center.PluginDetailPage(
      pluginId: _query(context, 'pluginId'),
      extensionId: _query(context, 'extensionId'),
    ),
  );
  registerMobileHostRuntime(
    'host.extensions.packages',
    (_) => const ExtensionPackagesPage(),
  );
  registerMobileHostRuntime('host.extensions.mcp', (_) => const McpListPage());
  registerMobileHostRuntime(
    'host.extensions.agent-skills',
    (_) => const AgentSkillsPage(),
  );
  registerMobileHostRuntime(
    'host.extensions.workflows',
    (_) => const WorkflowListPage(),
  );
  registerMobileHostRuntime(
    'host.memory.overview',
    (_) => const MemoryOverviewPage(),
  );
  registerMobileHostRuntime(
    'host.memory.timeline',
    (_) => const MemoryTimelinePage(),
  );
  registerMobileHostRuntime(
    'host.memory.episodic',
    (_) => const EpisodicMemoryPage(),
  );
  registerMobileHostRuntime(
    'host.memory.graph',
    (_) => const MemoryGraphPage(),
  );
  registerMobileHostRuntime(
    'host.memory.knowledge',
    (_) => const KnowledgePage(),
  );
  registerMobileHostRuntime(
    'host.profile.summary',
    (_) => const UserProfilesPage(),
  );
  registerMobileHostRuntime('host.tasks.continuity', (context) {
    final threadId = _parameter(context, 'id');
    if (threadId.isNotEmpty) {
      return ContinuityDetailPage(threadId: threadId);
    }
    return const ContinuityPage();
  });
  registerMobileHostRuntime(
    'host.user-space',
    (context) => MobileUserSpaceHost(context: context),
  );
  registerMobileHostRuntime(
    'host.mobile.native-page',
    (context) => MobileNativePageHost(context: context),
  );
}

void _ensureBuiltinMobileHostRuntimesRegistered() {
  if (_builtinMobileHostRuntimesRegistered) return;
  _builtinMobileHostRuntimesRegistered = true;
  _registerBuiltinMobileHostRuntimes();
}
