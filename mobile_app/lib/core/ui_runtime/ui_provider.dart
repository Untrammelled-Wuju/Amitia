import 'package:flutter/foundation.dart';

enum UIProviderMode { replace, compose, augment }
enum UIProviderEntryType { builtinNative, declarative, webModule, schemaRenderer, webRestricted, webIsolated, unknown }

abstract final class UICapability {
  static const appShell = 'app.shell';
  static const appNavigation = 'app.navigation';
  static const appWorkspace = 'app.workspace';
  static const routeRegistry = 'route.registry';
  static const pageProvider = 'page.provider';
  static const conversationShell = 'conversation.shell';
  static const conversationHeader = 'conversation.header';
  static const conversationMessages = 'conversation.messages';
  static const conversationMessageRenderer = 'conversation.message_renderer';
  static const conversationSidebar = 'conversation.sidebar';
  static const conversationComposer = 'conversation.composer';
  static const conversationOverlay = 'conversation.overlay';
  static const characterShell = 'character.shell';
  static const characterDetail = 'character.detail';
  static const memoryShell = 'memory.shell';
  static const memoryDetail = 'memory.detail';
  static const settingsShell = 'settings.shell';
  static const settingsSection = 'settings.section';
  static const extensionCenter = 'extension.center';
  static const extensionPage = 'extension.page';
  static const theme = 'ui.theme';
  static const tokens = 'ui.tokens';
  static const icons = 'ui.icons';
  static const components = 'ui.components';

  static const all = <String>[
    appShell, appNavigation, appWorkspace, routeRegistry, pageProvider,
    conversationShell, conversationHeader, conversationMessages, conversationMessageRenderer, conversationSidebar, conversationComposer, conversationOverlay,
    characterShell, characterDetail, memoryShell, memoryDetail, settingsShell, settingsSection, extensionCenter, extensionPage,
    theme, tokens, icons, components,
  ];
}

String currentUIPlatform() {
  if (kIsWeb) return 'web';
  switch (defaultTargetPlatform) {
    case TargetPlatform.android: return 'android';
    case TargetPlatform.iOS: return 'ios';
    case TargetPlatform.windows: return 'windows';
    case TargetPlatform.macOS: return 'macos';
    case TargetPlatform.linux: return 'linux';
    case TargetPlatform.fuchsia: return 'mobile';
  }
}

class UIProviderEntry {
  final String? contributionId;
  final UIProviderEntryType type;
  final String? path;
  final String? schemaPath;
  final String? exportName;
  final String? contentHash;

  const UIProviderEntry({this.contributionId, required this.type, this.path, this.schemaPath, this.exportName, this.contentHash});

  factory UIProviderEntry.fromJson(Map<String, dynamic> json) {
    final rawType = (json['type'] ?? '').toString();
    final type = switch (rawType) {
      'builtin_native' => UIProviderEntryType.builtinNative,
      'declarative' => UIProviderEntryType.declarative,
      'web_module' => UIProviderEntryType.webModule,
      'schema_renderer' => UIProviderEntryType.schemaRenderer,
      'web_restricted' => UIProviderEntryType.webRestricted,
      'web_isolated' => UIProviderEntryType.webIsolated,
      _ => UIProviderEntryType.unknown,
    };
    return UIProviderEntry(
      contributionId: json['contributionId']?.toString(),
      type: type,
      path: json['path']?.toString(),
      schemaPath: json['schemaPath']?.toString(),
      exportName: json['exportName']?.toString(),
      contentHash: json['contentHash']?.toString(),
    );
  }
}

class UIProviderDefinition {
  final String providerId;
  final String extensionId;
  final String? moduleId;
  final String capability;
  final UIProviderMode mode;
  final int priority;
  final List<String> platforms;
  final Map<String, UIProviderEntry> entries;
  final String? fallbackProviderId;
  final String? trustLevel;
  final List<String> permissions;
  final int generation;
  final bool enabled;
  final bool builtin;
  final Map<String, dynamic> metadata;

  const UIProviderDefinition({
    required this.providerId, required this.extensionId, this.moduleId, required this.capability,
    required this.mode, required this.priority, required this.platforms, required this.entries,
    this.fallbackProviderId, this.trustLevel, required this.permissions, required this.generation,
    required this.enabled, required this.builtin, required this.metadata,
  });

  factory UIProviderDefinition.fromJson(Map<String, dynamic> json) {
    final rawEntries = (json['entries'] as Map?)?.cast<String, dynamic>() ?? const <String, dynamic>{};
    final entries = <String, UIProviderEntry>{};
    for (final item in rawEntries.entries) {
      if (item.value is Map) entries[item.key] = UIProviderEntry.fromJson((item.value as Map).cast<String, dynamic>());
    }
    return UIProviderDefinition(
      providerId: (json['providerId'] ?? '').toString(),
      extensionId: (json['extensionId'] ?? '').toString(),
      moduleId: json['moduleId']?.toString(),
      capability: (json['capability'] ?? '').toString(),
      mode: switch ((json['mode'] ?? 'replace').toString()) { 'compose' => UIProviderMode.compose, 'augment' => UIProviderMode.augment, _ => UIProviderMode.replace },
      priority: (json['priority'] as num?)?.toInt() ?? 0,
      platforms: ((json['platforms'] as List?) ?? const []).map((e) => e.toString()).toList(),
      entries: entries,
      fallbackProviderId: json['fallbackProviderId']?.toString(),
      trustLevel: json['trustLevel']?.toString(),
      permissions: ((json['permissions'] as List?) ?? const []).map((e) => e.toString()).toList(),
      generation: (json['generation'] as num?)?.toInt() ?? 0,
      enabled: json['enabled'] != false,
      builtin: json['builtin'] == true,
      metadata: (json['metadata'] as Map?)?.cast<String, dynamic>() ?? const {},
    );
  }

  UIProviderEntry? entryFor(String platform) {
    if (entries[platform] case final direct?) return direct;
    if ((platform == 'android' || platform == 'ios') && entries['mobile'] case final mobile?) return mobile;
    if ((platform == 'windows' || platform == 'macos' || platform == 'linux') && entries['desktop'] case final desktop?) return desktop;
    return entries['*'];
  }
}

class UIProfile {
  final String profileId;
  final String name;
  final Map<String, String> selections;
  final int? updatedAt;
  const UIProfile({required this.profileId, required this.name, required this.selections, this.updatedAt});
  factory UIProfile.fromJson(Map<String, dynamic> json) => UIProfile(
    profileId: (json['profileId'] ?? 'default').toString(),
    name: (json['name'] ?? 'Default').toString(),
    selections: ((json['selections'] as Map?) ?? const {}).map((k, v) => MapEntry(k.toString(), v.toString())),
    updatedAt: (json['updatedAt'] as num?)?.toInt(),
  );
  Map<String, dynamic> toJson() => {'profileId': profileId, 'name': name, 'selections': selections, if (updatedAt != null) 'updatedAt': updatedAt};
}

class UIProviderSnapshot {
  final List<UIProviderDefinition> providers;
  final UIProfile profile;
  final Map<String, UIProviderDefinition> resolved;
  final int version;
  const UIProviderSnapshot({required this.providers, required this.profile, required this.resolved, required this.version});

  factory UIProviderSnapshot.fromJson(Map<String, dynamic> json) {
    final providers = ((json['providers'] as List?) ?? const [])
        .whereType<Map>()
        .map((e) => UIProviderDefinition.fromJson(e.cast<String, dynamic>()))
        .toList();
    final resolved = <String, UIProviderDefinition>{};
    final rawResolved = (json['resolved'] as Map?) ?? const {};
    for (final entry in rawResolved.entries) {
      if (entry.value is Map) resolved[entry.key.toString()] = UIProviderDefinition.fromJson((entry.value as Map).cast<String, dynamic>());
    }
    return UIProviderSnapshot(
      providers: providers,
      profile: UIProfile.fromJson(((json['profile'] as Map?) ?? const {}).cast<String, dynamic>()),
      resolved: resolved,
      version: (json['providerVersion'] as num?)?.toInt() ?? (json['version'] as num?)?.toInt() ?? 1,
    );
  }

  UIProviderDefinition? resolve(String capability, {String? providerId}) {
    if (providerId != null && providerId.isNotEmpty) {
      for (final provider in providers) {
        if (provider.providerId == providerId && provider.enabled && provider.capability == capability) return provider;
      }
    }
    return resolved[capability];
  }

  List<UIProviderDefinition> fallbackChain(
    String capability, {
    String? providerId,
    String? platform,
  }) {
    final activePlatform = platform ?? currentUIPlatform();
    final byId = <String, UIProviderDefinition>{
      for (final provider in providers) provider.providerId: provider,
    };
    final chain = <UIProviderDefinition>[];
    final seen = <String>{};
    UIProviderDefinition? current = resolve(capability, providerId: providerId);
    while (current != null && seen.add(current.providerId)) {
      if (current.enabled && current.capability == capability && current.entryFor(activePlatform) != null) {
        chain.add(current);
      }
      final nextId = current.fallbackProviderId?.trim() ?? '';
      current = nextId.isEmpty ? null : byId[nextId];
    }
    for (final provider in providers) {
      if (provider.builtin &&
          provider.enabled &&
          provider.capability == capability &&
          provider.entryFor(activePlatform) != null &&
          seen.add(provider.providerId)) {
        chain.add(provider);
        break;
      }
    }
    return chain;
  }
}
