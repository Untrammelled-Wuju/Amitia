import 'package:flutter/foundation.dart';

enum UIProviderMode { replace, compose, augment }
enum UIProviderPlacement { any, cloud, device, hybrid }
enum UIProfileScopeKind { global, user, platform, device, devicePlatform, runtime }
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

class UIDeviceRequirements {
  final List<String> platforms;
  final List<String> architectures;
  final String? minAppVersion;
  final String? minRuntimeVersion;
  final List<String> requiredFeatures;
  const UIDeviceRequirements({this.platforms = const [], this.architectures = const [], this.minAppVersion, this.minRuntimeVersion, this.requiredFeatures = const []});
  factory UIDeviceRequirements.fromJson(Map<String, dynamic> json) => UIDeviceRequirements(
    platforms: ((json['platforms'] as List?) ?? const []).map((e) => e.toString()).toList(),
    architectures: ((json['architectures'] as List?) ?? const []).map((e) => e.toString()).toList(),
    minAppVersion: json['minAppVersion']?.toString(),
    minRuntimeVersion: json['minRuntimeVersion']?.toString(),
    requiredFeatures: ((json['requiredFeatures'] as List?) ?? const []).map((e) => e.toString()).toList(),
  );
}

class UIProfileScope {
  final String? userId;
  final String? deviceId;
  final String? platform;
  final String? runtimeProfile;
  const UIProfileScope({this.userId, this.deviceId, this.platform, this.runtimeProfile});
  factory UIProfileScope.fromJson(Map<String, dynamic> json) => UIProfileScope(
    userId: json['userId']?.toString(), deviceId: json['deviceId']?.toString(),
    platform: json['platform']?.toString(), runtimeProfile: json['runtimeProfile']?.toString(),
  );
  Map<String, dynamic> toJson() => {
    if (userId?.isNotEmpty == true) 'userId': userId,
    if (deviceId?.isNotEmpty == true) 'deviceId': deviceId,
    if (platform?.isNotEmpty == true) 'platform': platform,
    if (runtimeProfile?.isNotEmpty == true) 'runtimeProfile': runtimeProfile,
  };
}

class UIProviderResolveContext extends UIProfileScope {
  final String? architecture;
  final String? appVersion;
  final String? runtimeVersion;
  final bool deviceOnline;
  final bool localRuntime;
  final List<String> deviceCapabilities;
  const UIProviderResolveContext({super.userId, super.deviceId, super.platform, super.runtimeProfile, this.architecture, this.appVersion, this.runtimeVersion, this.deviceOnline = false, this.localRuntime = false, this.deviceCapabilities = const []});
  factory UIProviderResolveContext.fromJson(Map<String, dynamic> json) => UIProviderResolveContext(
    userId: json['userId']?.toString(), deviceId: json['deviceId']?.toString(), platform: json['platform']?.toString(), runtimeProfile: json['runtimeProfile']?.toString(),
    architecture: json['architecture']?.toString(), appVersion: json['appVersion']?.toString(), runtimeVersion: json['runtimeVersion']?.toString(),
    deviceOnline: json['deviceOnline'] == true, localRuntime: json['localRuntime'] == true,
    deviceCapabilities: ((json['deviceCapabilities'] as List?) ?? const []).map((e) => e.toString()).toList(),
  );
}

String uiProfileScopeKindValue(UIProfileScopeKind kind) => switch (kind) {
  UIProfileScopeKind.global => 'global', UIProfileScopeKind.user => 'user', UIProfileScopeKind.platform => 'platform',
  UIProfileScopeKind.device => 'device', UIProfileScopeKind.devicePlatform => 'device_platform', UIProfileScopeKind.runtime => 'runtime',
};

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
  final UIProviderPlacement placement;
  final UIDeviceRequirements? deviceRequirements;
  final int generation;
  final bool enabled;
  final bool builtin;
  final Map<String, dynamic> metadata;

  const UIProviderDefinition({
    required this.providerId, required this.extensionId, this.moduleId, required this.capability,
    required this.mode, required this.priority, required this.platforms, required this.entries,
    this.fallbackProviderId, this.trustLevel, required this.permissions, required this.placement, this.deviceRequirements, required this.generation,
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
      placement: switch ((json['placement'] ?? (json['builtin'] == true ? 'any' : 'cloud')).toString()) { 'device' => UIProviderPlacement.device, 'hybrid' => UIProviderPlacement.hybrid, 'any' => UIProviderPlacement.any, _ => UIProviderPlacement.cloud },
      deviceRequirements: json['deviceRequirements'] is Map ? UIDeviceRequirements.fromJson((json['deviceRequirements'] as Map).cast<String, dynamic>()) : null,
      generation: (json['generation'] as num?)?.toInt() ?? 0,
      enabled: json['enabled'] != false,
      builtin: json['builtin'] == true,
      metadata: (json['metadata'] as Map?)?.cast<String, dynamic>() ?? const {},
    );
  }

  bool compatibleWith(UIProviderResolveContext context, String platform) {
    if (!enabled || entryFor(platform) == null) return false;
    if (placement == UIProviderPlacement.device && !context.localRuntime) {
      if ((context.deviceId ?? '').isEmpty || !context.deviceOnline) return false;
    }
    final req = deviceRequirements;
    if (req == null) return true;
    bool contains(List<String> values, String value) => values.any((item) => item.toLowerCase() == value.toLowerCase());
    if (req.platforms.isNotEmpty && !contains(req.platforms, platform)) return false;
    if (req.architectures.isNotEmpty) {
      if ((context.architecture ?? '').isEmpty || !contains(req.architectures, context.architecture!)) return false;
    }
    if ((req.minAppVersion ?? '').isNotEmpty) {
      if ((context.appVersion ?? '').isEmpty || _compareVersion(context.appVersion!, req.minAppVersion!) < 0) return false;
    }
    if ((req.minRuntimeVersion ?? '').isNotEmpty) {
      if ((context.runtimeVersion ?? '').isEmpty || _compareVersion(context.runtimeVersion!, req.minRuntimeVersion!) < 0) return false;
    }
    for (final feature in req.requiredFeatures) {
      if (!contains(context.deviceCapabilities, feature)) return false;
    }
    return true;
  }

  UIProviderEntry? entryFor(String platform) {
    if (entries[platform] case final direct?) return direct;
    if (platform == 'android' || platform == 'ios') {
      if (entries['mobile'] case final mobile?) return mobile;
    }
    if (platform == 'windows' || platform == 'macos' || platform == 'linux') {
      if (entries['desktop'] case final desktop?) return desktop;
    }
    return entries['*'];
  }
}

int _compareVersion(String a, String b) {
  List<int> parse(String raw) {
    var normalized = raw.trim();
    if (normalized.startsWith('v') || normalized.startsWith('V')) {
      normalized = normalized.substring(1);
    }
    normalized = normalized.split('-').first;
    final source = normalized.split('.');
    return List<int>.generate(4, (index) => index < source.length ? int.tryParse(source[index]) ?? 0 : 0);
  }

  final left = parse(a);
  final right = parse(b);
  for (var i = 0; i < left.length; i++) {
    if (left[i] < right[i]) return -1;
    if (left[i] > right[i]) return 1;
  }
  return 0;
}

class UIProfile {
  final String profileId;
  final String name;
  final Map<String, String> selections;
  final UIProfileScope? scope;
  final int revision;
  final int? updatedAt;
  const UIProfile({required this.profileId, required this.name, required this.selections, this.scope, this.revision = 0, this.updatedAt});
  factory UIProfile.fromJson(Map<String, dynamic> json) => UIProfile(
    profileId: (json['profileId'] ?? 'default').toString(),
    name: (json['name'] ?? 'Default').toString(),
    selections: ((json['selections'] as Map?) ?? const {}).map((k, v) => MapEntry(k.toString(), v.toString())),
    scope: json['scope'] is Map ? UIProfileScope.fromJson((json['scope'] as Map).cast<String, dynamic>()) : null,
    revision: (json['revision'] as num?)?.toInt() ?? 0,
    updatedAt: (json['updatedAt'] as num?)?.toInt(),
  );
  Map<String, dynamic> toJson() => {'profileId': profileId, 'name': name, 'selections': selections, if (scope != null) 'scope': scope!.toJson(), 'revision': revision, if (updatedAt != null) 'updatedAt': updatedAt};
}

class UIProfileEnvelope {
  final UIProfile profile;
  final List<UIProfile> layers;
  final UIProviderResolveContext context;
  final UIProfileScopeKind scope;
  final UIProfile scopeProfile;
  final bool scopeExists;
  const UIProfileEnvelope({required this.profile, required this.layers, required this.context, required this.scope, required this.scopeProfile, required this.scopeExists});
  factory UIProfileEnvelope.fromJson(Map<String, dynamic> json) {
    final rawScope = (json['scope'] ?? 'user').toString();
    final scope = switch (rawScope) { 'global' => UIProfileScopeKind.global, 'platform' => UIProfileScopeKind.platform, 'device' => UIProfileScopeKind.device, 'device_platform' => UIProfileScopeKind.devicePlatform, 'runtime' => UIProfileScopeKind.runtime, _ => UIProfileScopeKind.user };
    return UIProfileEnvelope(
      profile: UIProfile.fromJson(((json['profile'] as Map?) ?? const {}).cast<String, dynamic>()),
      layers: ((json['layers'] as List?) ?? const []).whereType<Map>().map((e) => UIProfile.fromJson(e.cast<String, dynamic>())).toList(),
      context: UIProviderResolveContext.fromJson(((json['context'] as Map?) ?? const {}).cast<String, dynamic>()),
      scope: scope,
      scopeProfile: UIProfile.fromJson(((json['scopeProfile'] as Map?) ?? const {}).cast<String, dynamic>()),
      scopeExists: json['scopeExists'] == true,
    );
  }
}

class UIProviderSnapshot {
  final List<UIProviderDefinition> providers;
  final UIProfile profile;
  final List<UIProfile> profileLayers;
  final UIProviderResolveContext context;
  final Map<String, UIProviderDefinition> resolved;
  final int version;
  const UIProviderSnapshot({required this.providers, required this.profile, required this.profileLayers, required this.context, required this.resolved, required this.version});

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
      profileLayers: ((json['profileLayers'] as List?) ?? const []).whereType<Map>().map((e) => UIProfile.fromJson(e.cast<String, dynamic>())).toList(),
      context: UIProviderResolveContext.fromJson(((json['providerContext'] as Map?) ?? const {}).cast<String, dynamic>()),
      resolved: resolved,
      version: (json['providerVersion'] as num?)?.toInt() ?? (json['version'] as num?)?.toInt() ?? 1,
    );
  }

  UIProviderDefinition? resolve(String capability, {String? providerId}) {
    final platform = currentUIPlatform();
    final byId = <String, UIProviderDefinition>{
      for (final provider in providers) provider.providerId: provider,
    };

    UIProviderDefinition? resolveFrom(String? initialId) {
      var id = initialId?.trim() ?? '';
      final seen = <String>{};
      while (id.isNotEmpty && seen.add(id)) {
        final provider = byId[id];
        if (provider == null || provider.capability != capability) return null;
        if (provider.compatibleWith(context, platform)) return provider;
        id = provider.fallbackProviderId?.trim() ?? '';
      }
      return null;
    }

    if (providerId != null && providerId.isNotEmpty) {
      final explicit = resolveFrom(providerId);
      if (explicit != null) return explicit;
    }

    final serverResolved = resolved[capability];
    if (serverResolved != null) {
      final active = resolveFrom(serverResolved.providerId);
      if (active != null) return active;
    }

    final selected = profile.selections[capability];
    if (selected != null && selected.isNotEmpty) {
      final active = resolveFrom(selected);
      if (active != null) return active;
    }

    for (final provider in providers) {
      if (provider.builtin && provider.capability == capability && provider.compatibleWith(context, platform)) return provider;
    }
    return null;
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
      if (current.capability == capability && current.compatibleWith(context, activePlatform)) {
        chain.add(current);
      }
      final nextId = current.fallbackProviderId?.trim() ?? '';
      current = nextId.isEmpty ? null : byId[nextId];
    }
    for (final provider in providers) {
      if (provider.builtin &&
          provider.capability == capability &&
          provider.compatibleWith(context, activePlatform) &&
          seen.add(provider.providerId)) {
        chain.add(provider);
        break;
      }
    }
    return chain;
  }
}
