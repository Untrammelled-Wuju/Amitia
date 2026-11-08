import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/features/game_center/data/game_center_api.dart';
import 'package:flutter_test/flutter_test.dart';
import '../../core/services/owned_task_submission_test.dart' show TaskTransport;

Map<String, dynamic> session({
  String device = 'device-a',
  String state = 'active',
  int revision = 2,
  String runtimeId = '',
}) => {
  'spaceId': 'space',
  'sessionId': 'session-a',
  'deviceId': device,
  'state': state,
  'gameExtensionId': '',
  'gamePluginId': '',
  'gameTitle': 'Game',
  'runtimeId': runtimeId,
  'revision': revision,
  'startedAt': '2026-11-08T00:00:00Z',
  'updatedAt': '2026-11-08T00:00:01Z',
};

void main() {
  test('读取其他设备占用游戏模式', () async {
    final transport = TaskTransport([
      {
        'session': session(),
        'currentDeviceId': 'device-b',
        'owned': false,
        'canAcquire': false,
      },
    ]);
    final status = await GameCenterApi(
      BackendServiceApi(transport, 1),
    ).getGameModeStatus();
    expect(status.session.deviceId, 'device-a');
    expect(status.session.active, isTrue);
    expect(status.owned, isFalse);
    expect(status.currentDeviceId, 'device-b');
    expect(transport.requests.single.path, '/api/game-mode');
  });

  test('动态行为目录和调用使用插件返回的 revision', () async {
    final transport = TaskTransport([
      {
        'pluginId': 'com.example/game',
        'extensionId': 'com.example',
        'catalogRevision': 'r7',
        'behaviors': [
          {
            'id': 'pause',
            'name': '暂停',
            'serviceId': 'game',
            'method': 'game.pause',
            'inputSchema': {'type': 'object'},
          },
        ],
      },
      {
        'pluginId': 'com.example/game',
        'behaviorId': 'pause',
        'catalogRevision': 'r7',
        'result': {'ok': true},
      },
    ]);
    final api = GameCenterApi(BackendServiceApi(transport, 1));
    final catalog = await api.listBehaviors('com.example/game');
    expect(catalog['catalogRevision'], 'r7');
    final result = await api.invokeBehavior(
      'com.example/game',
      'pause',
      expectedCatalogRevision: 'r7',
    );
    expect(result['result'], {'ok': true});
    expect(transport.requests[0].path, '/api/game-center/behaviors');
    expect(
      transport.requests[0].queryParameters?['pluginId'],
      'com.example/game',
    );
    expect(transport.requests[1].path, '/api/game-center/behaviors/invoke');
  });

  test('含斜杠的扩展 ID 通过请求体启停，不进入路径参数', () async {
    final transport = TaskTransport([
      {'state': 'enabled'},
      {'state': 'disabled'},
    ]);
    final api = GameCenterApi(BackendServiceApi(transport, 1));

    expect(await api.enablePlugin('com.amitia/world-game-plugin'), isTrue);
    expect(await api.disablePlugin('com.amitia/world-game-plugin'), isTrue);

    expect(transport.requests[0].path, '/api/game-center/extensions/enable');
    expect(transport.requests[0].body, {
      'extensionId': 'com.amitia/world-game-plugin',
    });
    expect(transport.requests[1].path, '/api/game-center/extensions/disable');
    expect(transport.requests[1].body, {
      'extensionId': 'com.amitia/world-game-plugin',
    });
  });

  test('退出游戏模式先停止本机运行实例再释放租约', () async {
    final transport = TaskTransport([
      {'runtimeId': 'rt_game', 'operation': 'stop'},
      session(state: 'inactive', revision: 3),
    ]);
    final api = GameCenterApi(BackendServiceApi(transport, 1));

    await api.releaseGameMode(
      sessionId: 'session-a',
      expectedRevision: 2,
      runtimeId: 'rt_game',
    );

    expect(
      transport.requests[0].path,
      '/api/game-center/runtimes/rt_game/stop',
    );
    expect(transport.requests[1].path, '/api/game-mode/release');
    expect(transport.requests[1].body, {
      'sessionId': 'session-a',
      'expectedRevision': 2,
      'force': false,
    });
  });
}
