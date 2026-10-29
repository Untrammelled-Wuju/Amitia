import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/extension_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/ui_runtime/ui_runtime_controller.dart';
import 'package:amitia_app/features/profile/presentation/pages/profile_hub_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _FakeBackendServiceApi extends Fake implements BackendServiceApi {
  @override
  int get generation => 1;
}

class _FakeExtensionService extends ExtensionService {
  _FakeExtensionService() : super(_FakeBackendServiceApi());

  @override
  Future<Map<String, dynamic>> getUISnapshot(
    String platform, {
    String deviceId = '',
  }) async {
    return <String, dynamic>{
      'providers': <Map<String, dynamic>>[
        <String, dynamic>{
          'providerId': 'channel-presentation',
          'extensionId': 'com.amitia/channel-test',
          'capability': 'channel.presentation',
          'mode': 'augment',
          'priority': 100,
          'entries': <String, dynamic>{
            'android': <String, dynamic>{'type': 'declarative'},
          },
          'placement': 'device',
          'enabled': true,
          'builtin': false,
          'generation': 1,
          'metadata': <String, dynamic>{
            'channelId': 'test_channel',
            'displayName': '测试渠道',
          },
        },
      ],
      'slots': <dynamic>[],
      'profile': <String, dynamic>{
        'profileId': 'default',
        'name': 'Default',
        'selections': <String, dynamic>{},
      },
      'profileLayers': <dynamic>[],
      'providerContext': <String, dynamic>{
        'platform': 'android',
        'localRuntime': true,
        'deviceOnline': true,
        'deviceId': 'device',
      },
      'resolved': <String, dynamic>{},
      'providerVersion': 1,
    };
  }
}

void main() {
  testWidgets('profile channel entry follows installed channel plugins', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final container = ProviderContainer(
      overrides: <Override>[
        currentSpaceProfileProvider.overrideWith((ref) async => null),
        uiRuntimeProvider.overrideWith(
          (ref) => UIRuntimeController(
            _FakeExtensionService(),
            onLastKnownGood: (_) {},
            cacheNamespace: () async => 'test',
            meshDeviceId: () async => null,
          ),
        ),
      ],
    );
    await container.read(uiRuntimeProvider.notifier).ensureLoaded();

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(
          routerConfig: GoRouter(
            initialLocation: '/me',
            routes: [
              GoRoute(
                path: '/me',
                builder: (context, state) => const ProfileHubPage(),
              ),
            ],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('渠道消息'), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
    container.dispose();
  });
}
