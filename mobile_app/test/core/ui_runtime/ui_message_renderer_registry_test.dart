import 'package:amitia_app/core/ui_runtime/ui_message_renderer_registry.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:flutter_test/flutter_test.dart';

UIProviderDefinition _provider() {
  return const UIProviderDefinition(
    providerId: 'channel.renderer',
    extensionId: 'com.amitia/channel-test',
    capability: UICapability.conversationMessageRenderer,
    mode: UIProviderMode.replace,
    priority: 10,
    platforms: <String>[],
    entries: <String, UIProviderEntry>{
      '*': UIProviderEntry(type: UIProviderEntryType.declarative),
    },
    permissions: <String>[],
    placement: UIProviderPlacement.any,
    generation: 1,
    enabled: true,
    builtin: false,
    metadata: <String, dynamic>{
      'channelIds': <String>['wechat_personal'],
    },
  );
}

UIProviderSnapshot _snapshot(UIProviderDefinition provider) {
  return UIProviderSnapshot(
    providers: <UIProviderDefinition>[provider],
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
  test('message renderer channel selector only matches declared channel', () {
    final renderer = _provider();
    final snapshot = _snapshot(renderer);
    expect(
      UIMessageRendererRegistry.resolve(
        snapshot,
        messageType: 'text',
        role: 'user',
        channelId: 'wechat_personal',
      )?.providerId,
      renderer.providerId,
    );
    expect(
      UIMessageRendererRegistry.resolve(
        snapshot,
        messageType: 'text',
        role: 'user',
        channelId: 'qq',
      ),
      isNull,
    );
  });
}
