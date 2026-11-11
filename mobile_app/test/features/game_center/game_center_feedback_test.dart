import 'package:amitia_app/features/game_center/data/game_center_api.dart';
import 'package:amitia_app/features/game_center/domain/game_center_dto.dart';
import 'package:amitia_app/features/game_center/presentation/controllers/game_center_controller.dart';
import 'package:amitia_app/features/game_center/presentation/controllers/game_center_providers.dart';
import 'package:amitia_app/features/game_center/presentation/pages/game_center_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class TestGameApi extends Fake implements GameCenterApi {}

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
