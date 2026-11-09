import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:flutter_test/flutter_test.dart';

UIProviderDefinition _provider({
  bool enabled = true,
  Map<String, UIProviderEntry> entries = const {
    'android': UIProviderEntry(type: UIProviderEntryType.declarative),
  },
  UIDeviceRequirements? requirements,
}) {
  return UIProviderDefinition(
    providerId: 'provider',
    extensionId: 'com.example/plugin',
    capability: UICapability.pageProvider,
    mode: UIProviderMode.replace,
    priority: 1,
    platforms: const <String>[],
    entries: entries,
    permissions: const <String>[],
    placement: UIProviderPlacement.any,
    deviceRequirements: requirements,
    generation: 1,
    enabled: enabled,
    builtin: false,
    metadata: const <String, dynamic>{},
  );
}

void main() {
  test('provider compatibility exposes the unavailable reason', () {
    const context = UIProviderResolveContext(localRuntime: true);
    expect(
      _provider(enabled: false).incompatibilityReason(context, 'android'),
      'provider-disabled',
    );
    expect(
      _provider().incompatibilityReason(context, 'ios'),
      'entry-unavailable:ios',
    );
    expect(
      _provider(
        requirements: const UIDeviceRequirements(platforms: ['macos']),
      ).incompatibilityReason(context, 'android'),
      'platform-unsupported:android',
    );
  });

  test('snapshot exposes compatibility reason for every provider', () {
    final snapshot = UIProviderSnapshot.fromJson(<String, dynamic>{
      'providers': <dynamic>[],
      'slots': <dynamic>[],
      'profile': <String, dynamic>{},
      'profileLayers': <dynamic>[],
      'providerContext': <String, dynamic>{'platform': 'android'},
      'resolved': <String, dynamic>{},
      'providerResolutions': <String, dynamic>{
        'provider': <String, dynamic>{
          'providerId': 'provider',
          'capability': UICapability.pageProvider,
          'compatible': false,
          'reason': 'platform-unsupported:android',
        },
      },
      'providerVersion': 2,
    });
    final resolution = snapshot.resolutionFor('provider');
    expect(resolution?.compatible, isFalse);
    expect(resolution?.reason, 'platform-unsupported:android');
  });
}
