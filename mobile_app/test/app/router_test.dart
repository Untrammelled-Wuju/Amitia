import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/app/router.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/extension_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/ui_runtime/ui_runtime_controller.dart';
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

  Map<String, dynamic> response = <String, dynamic>{};

  @override
  Future<Map<String, dynamic>> getUISnapshot(
    String platform, {
    String deviceId = '',
  }) async {
    return response;
  }
}

Map<String, dynamic> _snapshotWithRoute() {
  return <String, dynamic>{
    'providers': <Map<String, dynamic>>[
      <String, dynamic>{
        'providerId': 'test-routes',
        'extensionId': 'com.amitia.test',
        'capability': 'route.registry',
        'mode': 'replace',
        'priority': 10,
        'platforms': <String>['*'],
        'entries': <String, dynamic>{
          '*': <String, dynamic>{'type': 'declarative'},
        },
        'trustLevel': 'trusted',
        'enabled': true,
        'builtin': false,
        'generation': 1,
        'metadata': <String, dynamic>{
          'routes': <Map<String, dynamic>>[
            <String, dynamic>{
              'id': 'test-page',
              'path': '/test-page',
              'providerId': 'test-page',
              'capability': 'page.provider',
              'priority': 10,
            },
          ],
        },
      },
      <String, dynamic>{
        'providerId': 'test-page',
        'extensionId': 'com.amitia.test',
        'capability': 'page.provider',
        'mode': 'replace',
        'priority': 10,
        'platforms': <String>['*'],
        'entries': <String, dynamic>{
          '*': <String, dynamic>{'type': 'declarative'},
        },
        'trustLevel': 'trusted',
        'enabled': true,
        'builtin': false,
        'generation': 1,
        'metadata': <String, dynamic>{},
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
    },
    'resolved': <String, dynamic>{},
    'providerVersion': 1,
  };
}

bool _containsRoute(List<RouteBase> routes, String path) {
  for (final route in routes) {
    if (route is GoRoute && route.path == path) return true;
    if (_containsRoute(route.routes, path)) return true;
  }
  return false;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('provider route updates preserve the GoRouter instance', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final service = _FakeExtensionService();
    final container = ProviderContainer(
      overrides: <Override>[
        uiRuntimeProvider.overrideWith(
          (ref) => UIRuntimeController(
            service,
            onLastKnownGood: (_) {},
            cacheNamespace: () async => 'test',
            meshDeviceId: () async => null,
          ),
        ),
      ],
    );
    addTearDown(container.dispose);

    final router = container.read(goRouterProvider);
    await container.read(uiRuntimeProvider.notifier).ensureLoaded();
    expect(identical(container.read(goRouterProvider), router), isTrue);

    service.response = _snapshotWithRoute();
    await container.read(uiRuntimeProvider.notifier).ensureLoaded(force: true);

    expect(identical(container.read(goRouterProvider), router), isTrue);
    expect(_containsRoute(router.configuration.routes, '/test-page'), isTrue);
  });

  group('resolveOnboardingRedirectLocation', () {
    const incomplete = OnboardingGateResult(completed: false);
    const completed = OnboardingGateResult(completed: true);

    test('引导未完成时拦截业务路由到 /onboarding', () {
      expect(
        resolveOnboardingRedirectLocation(AppRoutes.conversations, incomplete),
        AppRoutes.onboarding,
      );
      expect(
        resolveOnboardingRedirectLocation(AppRoutes.chat, incomplete),
        AppRoutes.onboarding,
      );
    });

    test('引导未完成时放行 /onboarding 与前置页面', () {
      expect(
        resolveOnboardingRedirectLocation(AppRoutes.onboarding, incomplete),
        isNull,
      );
      expect(
        resolveOnboardingRedirectLocation(AppRoutes.privacy, incomplete),
        isNull,
      );
      expect(
        resolveOnboardingRedirectLocation(
          AppRoutes.settingsUIProviders,
          incomplete,
        ),
        isNull,
      );
    });

    test('引导已完成时不再拦截业务路由', () {
      expect(
        resolveOnboardingRedirectLocation(AppRoutes.conversations, completed),
        isNull,
      );
      expect(
        resolveOnboardingRedirectLocation(AppRoutes.chat, completed),
        isNull,
      );
    });

    test('引导已完成时 /onboarding 跳回消息页', () {
      expect(
        resolveOnboardingRedirectLocation(AppRoutes.onboarding, completed),
        AppRoutes.conversations,
      );
    });
  });
}
