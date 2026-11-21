import 'package:amitia_app/core/ui_runtime/ui_page_provider_registry.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:flutter_test/flutter_test.dart';

UIProviderDefinition _provider({
  required String providerId,
  required String extensionId,
  String contributionId = 'dsh-compatible-mobile-page',
  bool enabled = true,
  bool builtin = false,
  Set<String> platforms = const <String>{'android', 'ios'},
}) {
  final entries = <String, UIProviderEntry>{
    for (final platform in platforms)
      platform: UIProviderEntry(
        contributionId: contributionId,
        type: UIProviderEntryType.mobileWebRuntime,
      ),
  };
  return UIProviderDefinition(
    providerId: providerId,
    extensionId: extensionId,
    capability: UICapability.pageProvider,
    mode: UIProviderMode.replace,
    priority: 100,
    platforms: const <String>[],
    entries: entries,
    permissions: const <String>[],
    placement: UIProviderPlacement.any,
    generation: 1,
    enabled: enabled,
    builtin: builtin,
    metadata: const <String, dynamic>{},
  );
}

UIProviderSnapshot _snapshot(List<UIProviderDefinition> providers) {
  return UIProviderSnapshot(
    providers: providers,
    slots: const <UISlotSnapshotEntry>[],
    profile: const UIProfile(
      profileId: 'default',
      name: 'Default',
      selections: <String, String>{},
    ),
    profileLayers: const <UIProfile>[],
    context: const UIProviderResolveContext(localRuntime: true),
    resolved: const <String, UIProviderDefinition>{},
    version: 1,
  );
}

void main() {
  test('resolves the owner extension from a page id without a query', () {
    final snapshot = _snapshot([
      _provider(
        providerId: 'dsh-compatible-page-provider',
        extensionId: 'com.amitia/dsh-compatible-host',
      ),
    ]);
    expect(
      UIPageProviderRegistry.extensionIdForPageId(
        snapshot,
        'dsh-compatible-mobile-page',
        platform: 'android',
      ),
      'com.amitia/dsh-compatible-host',
    );
  });

  test('returns null when several extensions declare the same page id', () {
    final snapshot = _snapshot([
      _provider(
        providerId: 'provider-a',
        extensionId: 'com.example/a',
        contributionId: 'shared-page',
      ),
      _provider(
        providerId: 'provider-b',
        extensionId: 'com.example/b',
        contributionId: 'shared-page',
      ),
    ]);
    expect(
      UIPageProviderRegistry.extensionIdForPageId(
        snapshot,
        'shared-page',
        platform: 'android',
      ),
      isNull,
    );
  });

  test('ignores disabled and builtin providers', () {
    final snapshot = _snapshot([
      _provider(
        providerId: 'disabled-provider',
        extensionId: 'com.example/disabled',
        enabled: false,
      ),
      _provider(
        providerId: 'builtin-provider',
        extensionId: 'com.example/builtin',
        builtin: true,
      ),
    ]);
    expect(
      UIPageProviderRegistry.extensionIdForPageId(
        snapshot,
        'dsh-compatible-mobile-page',
        platform: 'android',
      ),
      isNull,
    );
  });

  test('only matches the requested platform entry', () {
    final snapshot = _snapshot([
      _provider(
        providerId: 'desktop-provider',
        extensionId: 'com.example/desktop',
        platforms: const <String>{'macos', 'windows'},
      ),
    ]);
    expect(
      UIPageProviderRegistry.extensionIdForPageId(
        snapshot,
        'dsh-compatible-mobile-page',
        platform: 'android',
      ),
      isNull,
    );
    expect(
      UIPageProviderRegistry.extensionIdForPageId(
        snapshot,
        'dsh-compatible-mobile-page',
        platform: 'macos',
      ),
      'com.example/desktop',
    );
  });

  test('returns null for blank page ids and missing snapshots', () {
    final snapshot = _snapshot(const <UIProviderDefinition>[]);
    expect(
      UIPageProviderRegistry.extensionIdForPageId(
        snapshot,
        '  ',
        platform: 'android',
      ),
      isNull,
    );
    expect(
      UIPageProviderRegistry.extensionIdForPageId(
        null,
        'dsh-compatible-mobile-page',
        platform: 'android',
      ),
      isNull,
    );
  });
}
