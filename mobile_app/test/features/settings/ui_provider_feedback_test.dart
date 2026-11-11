import 'package:amitia_app/core/services/extension_service.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:amitia_app/core/ui_runtime/ui_runtime_controller.dart';
import 'package:amitia_app/features/settings/presentation/pages/ui_provider_settings_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class TestExtensionService extends Fake implements ExtensionService {}

class TestRuntime extends UIRuntimeController {
  TestRuntime()
    : super(
        TestExtensionService(),
        onLastKnownGood: (_) {},
        cacheNamespace: () async => 'feedback',
        meshDeviceId: () async => null,
      ) {
    state = AsyncValue.data(
      UIProviderSnapshot.fromJson({
        'providers': [
          {
            'providerId': 'unavailable-provider',
            'extensionId': 'test',
            'capability': UICapability.appShell,
            'enabled': false,
          },
        ],
        'slots': [],
        'profile': {
          'profileId': 'default',
          'name': 'Default',
          'selections': {},
        },
        'profileLayers': [],
        'providerContext': {'platform': 'android', 'localRuntime': true},
        'resolved': {},
        'providerVersion': 1,
      }),
    );
  }
  int writes = 0;
  @override
  Future<void> ensureLoaded({bool force = false}) async {}
  @override
  Future<UIProfileEnvelope> loadProfileScope(UIProfileScopeKind scope) async =>
      UIProfileEnvelope.fromJson({
        'scope': 'space',
        'profile': {},
        'scopeProfile': {},
        'context': {},
      });
  @override
  Future<UIProfile> updateSelection(
    String capability,
    String? providerId, {
    UIProfileScopeKind scope = UIProfileScopeKind.space,
  }) async {
    writes++;
    return UIProfile.fromJson({});
  }
}

void main() {
  testWidgets('未启用的界面提供者显示原因且不会写入配置', (tester) async {
    final runtime = TestRuntime();
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => const UIProviderSettingsPage(),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [uiRuntimeProvider.overrideWith((ref) => runtime)],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButton<String>).first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('unavailable-provider · cloud').last);
    await tester.pump();
    expect(find.text('该界面提供者未启用，无法选择。'), findsOneWidget);
    expect(runtime.writes, 0);
  });
}
