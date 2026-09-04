import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/app/app_routes.dart';

void main() {
  group('AppRoutes', () {
    test('all static route constants are non-empty strings starting with /', () {
      const routes = [
        AppRoutes.chat,
        AppRoutes.conversations,
        AppRoutes.dashboard,
        AppRoutes.channels,
        AppRoutes.channelsWechat,
        AppRoutes.channelsQq,
        AppRoutes.characters,
        AppRoutes.charactersCreate,
        AppRoutes.agent,
        AppRoutes.memory,
        AppRoutes.memoryTimeline,
        AppRoutes.memoryGraph,
        AppRoutes.memoryWorldBook,
        AppRoutes.memoryEpisodic,
        AppRoutes.memoryProfiles,
        AppRoutes.memoryManager,
        AppRoutes.reminders,
        AppRoutes.emotes,
        AppRoutes.chatLogs,
        AppRoutes.chatImport,
        AppRoutes.extensions,
        AppRoutes.extensionsPackages,
        AppRoutes.extensionsMcp,
        AppRoutes.extensionsMcpNew,
        AppRoutes.extensionsAgentSkills,
        AppRoutes.extensionsPlugins,
        AppRoutes.extensionsSkills,
        AppRoutes.extensionsRuns,
        AppRoutes.workshop,
        AppRoutes.workshopSkills,
        AppRoutes.workshopPet,
        AppRoutes.workshopPetCreate,
        AppRoutes.workshopPetTasks,
        AppRoutes.workshopPetInstallations,
        AppRoutes.settings,
        AppRoutes.settingsModels,
        AppRoutes.settingsAppearance,
        AppRoutes.settingsRuntime,
        AppRoutes.settingsPermissions,
        AppRoutes.settingsBackup,
        AppRoutes.settingsAi,
        AppRoutes.settingsSystem,
        AppRoutes.settingsTemporal,
        AppRoutes.settingsSafety,
        AppRoutes.settingsMaintenance,
        AppRoutes.settingsStorage,
        AppRoutes.settingsTheme,
        AppRoutes.settingsUser,
        AppRoutes.settingsPrivacyScan,
        AppRoutes.settingsDeployment,
        AppRoutes.settingsAbout,
        AppRoutes.settingsToolbox,
        AppRoutes.onboarding,
        AppRoutes.login,
        AppRoutes.privacy,
        AppRoutes.developer,
        AppRoutes.developerKernel,
        AppRoutes.gameCenter,
        AppRoutes.desktopPet,
      ];

      for (final route in routes) {
        expect(route.startsWith('/'), isTrue, reason: '$route should start with /');
        expect(route.length > 1, isTrue, reason: '$route should be longer than just /');
      }
    });

    test('dynamic route helpers produce correct paths', () {
      expect(AppRoutes.character('c1'), '/characters/c1');
      expect(AppRoutes.characterLifeRules('c1'), '/characters/c1/life-rules');
      expect(AppRoutes.characterVoice('c1'), '/characters/c1/voice');
      expect(AppRoutes.characterMemory('c1'), '/characters/c1/memory');
      expect(AppRoutes.characterTimeline('c1'), '/characters/c1/timeline');
      expect(AppRoutes.characterProactive('c1'), '/characters/c1/proactive');
      expect(AppRoutes.characterPsyche('c1'), '/characters/c1/psyche');
      expect(AppRoutes.characterDebug('c1'), '/characters/c1/debug');
      expect(AppRoutes.agentTask('t1'), '/agent/task/t1');
      expect(AppRoutes.mcpDetail('m1'), '/extensions/mcp/m1');
      expect(AppRoutes.mcpEdit('m1'), '/extensions/mcp/m1/edit');
      expect(AppRoutes.skillDetail('s1'), '/extensions/skills/s1');
      expect(AppRoutes.pluginDetail('p1'), '/extensions/plugins/p1');
      expect(AppRoutes.extensionPage('page1'), '/extension/page/page1');
      expect(AppRoutes.petProcessing('task1'), '/workshop/pet/processing/task1');
      expect(AppRoutes.petActionEditor('task1', 'wave'), '/workshop/pet/processing/task1/actions/wave/editor');
      expect(AppRoutes.skillDraftEditor('d1'), '/workshop/skills/d1/editor');
      expect(AppRoutes.modelConfig('llm'), '/settings/models/llm');
      expect(AppRoutes.kernelPage('wasm'), '/developer/kernel/wasm');
    });

    test('no duplicate route constants', () {
      const routes = [
        AppRoutes.chat,
        AppRoutes.conversations,
        AppRoutes.dashboard,
        AppRoutes.channels,
        AppRoutes.channelsWechat,
        AppRoutes.channelsQq,
        AppRoutes.characters,
        AppRoutes.charactersCreate,
        AppRoutes.agent,
        AppRoutes.memory,
        AppRoutes.memoryTimeline,
        AppRoutes.memoryGraph,
        AppRoutes.memoryWorldBook,
        AppRoutes.memoryEpisodic,
        AppRoutes.memoryProfiles,
        AppRoutes.memoryManager,
        AppRoutes.reminders,
        AppRoutes.emotes,
        AppRoutes.chatLogs,
        AppRoutes.chatImport,
        AppRoutes.extensions,
        AppRoutes.extensionsPackages,
        AppRoutes.extensionsMcp,
        AppRoutes.extensionsMcpNew,
        AppRoutes.extensionsAgentSkills,
        AppRoutes.extensionsPlugins,
        AppRoutes.extensionsSkills,
        AppRoutes.extensionsRuns,
        AppRoutes.workshop,
        AppRoutes.workshopSkills,
        AppRoutes.workshopPet,
        AppRoutes.workshopPetCreate,
        AppRoutes.workshopPetTasks,
        AppRoutes.workshopPetInstallations,
        AppRoutes.settings,
        AppRoutes.settingsModels,
        AppRoutes.settingsAppearance,
        AppRoutes.settingsRuntime,
        AppRoutes.settingsPermissions,
        AppRoutes.settingsBackup,
        AppRoutes.settingsAi,
        AppRoutes.settingsSystem,
        AppRoutes.settingsTemporal,
        AppRoutes.settingsSafety,
        AppRoutes.settingsMaintenance,
        AppRoutes.settingsStorage,
        AppRoutes.settingsTheme,
        AppRoutes.settingsUser,
        AppRoutes.settingsPrivacyScan,
        AppRoutes.settingsDeployment,
        AppRoutes.settingsAbout,
        AppRoutes.settingsToolbox,
        AppRoutes.onboarding,
        AppRoutes.login,
        AppRoutes.privacy,
        AppRoutes.developer,
        AppRoutes.developerKernel,
        AppRoutes.gameCenter,
        AppRoutes.desktopPet,
      ];

      final uniqueRoutes = routes.toSet();
      expect(uniqueRoutes.length, routes.length,
          reason: 'Duplicate route constants found');
    });

    test('settings toolbox route exists', () {
      expect(AppRoutes.settingsToolbox, '/settings/toolbox');
    });

    test('settings about route exists', () {
      expect(AppRoutes.settingsAbout, '/settings/about');
    });
  });
}
