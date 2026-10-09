import 'package:amitia_app/core/runtime/embedded/embedded_runtime_controller.dart';
import 'package:amitia_app/core/runtime/backend/backend_topology.dart';
import 'package:amitia_app/core/runtime/embedded/method_channel_embedded_runtime_controller.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  const channel = MethodChannel('com.amitia.runtime/bridge');

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
  });

  test('共用桥复用同 Profile 就绪代次及固定本机端口', () async {
    final calls = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
          calls.add(call.method);
          return {'state': 'READY', 'activeProfile': 'device-agent'};
        });
    final controller = MethodChannelEmbeddedRuntimeController();
    expect(
      await controller.ensureRunning(EmbeddedRuntimeProfile.deviceAgent),
      EmbeddedRuntimeStatus.ready,
    );
    expect(calls, ['runtime.snapshot']);
    expect(
      (await controller.getEndpoint()).httpBaseUri.toString(),
      'http://127.0.0.1:18899',
    );
  });

  test('本地切绑定 Profile 必须先停止原进程', () async {
    var state = 'READY';
    var profile = 'local';
    final calls = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
          calls.add(call.method);
          if (call.method == 'runtime.stop') state = 'STOPPED';
          if (call.method == 'runtime.startWithProfile') {
            expect(state, 'STOPPED');
            profile = (call.arguments as Map)['profile'] as String;
            state = 'STARTING';
            return {
              'accepted': true,
              'snapshot': {'state': state, 'activeProfile': profile},
            };
          }
          return {'state': state, 'activeProfile': profile};
        });
    final controller = MethodChannelEmbeddedRuntimeController();
    expect(
      await controller.ensureRunning(EmbeddedRuntimeProfile.deviceAgent),
      EmbeddedRuntimeStatus.starting,
    );
    expect(calls, [
      'runtime.snapshot',
      'runtime.stop',
      'runtime.snapshot',
      'runtime.startWithProfile',
    ]);
  });

  test('无 Profile 的旧就绪进程不能认领为绑定 Runtime', () async {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async => {'state': 'READY'});
    expect(
      await MethodChannelEmbeddedRuntimeController().ensureRunning(
        EmbeddedRuntimeProfile.deviceAgent,
      ),
      EmbeddedRuntimeStatus.failed,
    );
  });

  test('Runtime 尚未安装不能发起业务进程', () async {
    final calls = <String>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
          calls.add(call.method);
          return {'state': 'NOT_INSTALLED'};
        });
    expect(
      await MethodChannelEmbeddedRuntimeController().ensureRunning(
        EmbeddedRuntimeProfile.local,
      ),
      EmbeddedRuntimeStatus.notInstalled,
    );
    expect(calls, ['runtime.snapshot']);
  });
}
