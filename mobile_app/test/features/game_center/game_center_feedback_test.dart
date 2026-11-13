import 'package:amitia_app/features/game_center/data/game_center_api.dart';
import 'package:amitia_app/features/game_center/domain/game_center_dto.dart';
import 'package:amitia_app/features/game_center/presentation/controllers/game_center_controller.dart';
import 'package:amitia_app/features/game_center/presentation/controllers/game_center_providers.dart';
import 'package:amitia_app/features/game_center/presentation/pages/game_center_page.dart';
import 'package:amitia_app/features/game_center/presentation/pages/runtime_detail_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class TestGameApi extends Fake implements GameCenterApi {
  int accessReads = 0;

  @override
  Future<bool> developerAccess() async {
    accessReads++;
    return false;
  }
}

class TestRuntimeController extends GameCenterController {
  TestRuntimeController(TestGameApi api) : super(api: api) {
    state = const GameCenterState(runtimeOperationByRuntimeId: {'runtime'});
  }

  @override
  Future<void> selectRuntime(String runtimeId, {String? pluginId}) async {}
  @override
  Future<void> loadRuntimeServices(String runtimeId) async {}
  @override
  Future<void> loadRuntimeHealth(String runtimeId) async {}
  @override
  Future<void> performHandshake(String runtimeId) async {}
}

class TestGameController extends GameCenterController {
  TestGameController({bool inactive = false}) : super(api: TestGameApi()) {
    state = GameCenterState(
      gameModeStatus: GameModeStatus.fromJson({
        'session': {'state': inactive ? 'inactive' : 'active'},
        'currentDeviceId': 'device',
        'owned': !inactive,
      }),
      gameModeOperating: true,
      plugins: [
        GamePluginSummary.fromJson({
          'extensionId': 'example',
          'pluginId': 'game',
          'name': '测试游戏',
        }),
      ],
      packageOperationByExtensionId: const {'example'},
    );
  }
  int releases = 0;
  int acquisitions = 0;
  @override
  Future<void> loadGameMode() async {}
  @override
  Future<void> loadPlugins() async {}
  @override
  Future<void> loadCenterHealth() async {}
  @override
  Future<bool> releaseGameMode() async {
    releases++;
    return true;
  }

  @override
  Future<bool> acquireGameMode() async {
    acquisitions++;
    return true;
  }
}

void main() {
  testWidgets('运行实例操作期间点击刷新显示原因且不重新加载', (tester) async {
    final api = TestGameApi();
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) =>
              const RuntimeDetailPage(runtimeId: 'runtime'),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          gameCenterApiProvider.overrideWithValue(api),
          gameCenterControllerProvider.overrideWith(
            (ref) => TestRuntimeController(api),
          ),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pump();
    expect(api.accessReads, 1);
    await tester.tap(find.byTooltip('刷新运行时详情'));
    await tester.pump();
    expect(find.text('正在处理该运行实例，请稍后刷新。'), findsOneWidget);
    expect(api.accessReads, 1);
  });

  testWidgets('游戏模式和插件处理中点击提示且不重复操作', (tester) async {
    final controller = TestGameController();
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (context, state) => const GameCenterPage()),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          gameCenterControllerProvider.overrideWith((ref) => controller),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));
    await tester.tap(find.text('测试游戏'));
    await tester.pump();
    expect(find.text('正在处理该游戏插件，请稍后重试。'), findsOneWidget);
    ScaffoldMessenger.of(
      tester.element(find.text('测试游戏')),
    ).hideCurrentSnackBar();
    await tester.pump(const Duration(seconds: 1));
    await tester.tap(find.byTooltip('退出游戏模式'));
    await tester.pump();
    expect(find.text('正在处理游戏模式，请稍后重试。'), findsOneWidget);
    expect(controller.releases, 0);
  });
  testWidgets('进入游戏模式过程中点击显示原因', (tester) async {
    final controller = TestGameController(inactive: true);
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (context, state) => const GameCenterPage()),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          gameCenterControllerProvider.overrideWith((ref) => controller),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('正在进入...'));
    await tester.pump();
    expect(find.text('正在处理游戏模式，请稍后重试。'), findsOneWidget);
    expect(controller.acquisitions, 0);
  });
}
