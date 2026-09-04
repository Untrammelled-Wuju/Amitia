import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/runtime/method_channel_runtime_bridge.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('MethodChannelRuntimeBridge', () {
    late MethodChannelRuntimeBridge bridge;

    setUp(() {
      bridge = MethodChannelRuntimeBridge();
    });

    tearDown(() async {
      await bridge.dispose();
    });

    test('initial state is available', () {
      expect(bridge.snapshots, isNotNull);
    });

    test('contract has correct channel names', () {
      expect(RuntimeBridgeContract.methodChannelName, 'com.amitia.runtime/bridge');
      expect(RuntimeBridgeContract.eventChannelName, 'com.amitia.runtime/events');
    });

    test('contract has correct method names', () {
      expect(RuntimeBridgeContract.methodSnapshot, 'runtime.snapshot');
      expect(RuntimeBridgeContract.methodStart, 'runtime.start');
      expect(RuntimeBridgeContract.methodStop, 'runtime.stop');
      expect(RuntimeBridgeContract.methodInstall, 'runtime.install');
      expect(RuntimeBridgeContract.methodVerify, 'runtime.verify');
      expect(RuntimeBridgeContract.methodRepair, 'runtime.repair');
      expect(RuntimeBridgeContract.methodManifestSummary, 'runtime.manifestSummary');
    });

    testWidgets('snapshot handles MissingPluginException', (tester) async {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel(RuntimeBridgeContract.methodChannelName),
        (call) async => null,
      );

      final result = await bridge.snapshot();
      expect(result.state, RuntimeBridgeState.unavailable);
    });

    testWidgets('start returns bridge unavailable on null', (tester) async {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel(RuntimeBridgeContract.methodChannelName),
        (call) async => null,
      );

      final result = await bridge.start();
      expect(result.accepted, false);
      expect(result.error, isNotNull);
      expect(result.error!.code, 'BRIDGE_UNAVAILABLE');
    });

    testWidgets('stop returns bridge unavailable on null', (tester) async {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel(RuntimeBridgeContract.methodChannelName),
        (call) async => null,
      );

      final result = await bridge.stop();
      expect(result.accepted, false);
      expect(result.error, isNotNull);
      expect(result.error!.code, 'BRIDGE_UNAVAILABLE');
    });

    testWidgets('install returns bridge unavailable on null', (tester) async {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel(RuntimeBridgeContract.methodChannelName),
        (call) async => null,
      );

      final result = await bridge.install();
      expect(result.accepted, false);
      expect(result.error, isNotNull);
      expect(result.error!.code, 'BRIDGE_UNAVAILABLE');
    });

    testWidgets('verify returns bridge unavailable on null', (tester) async {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel(RuntimeBridgeContract.methodChannelName),
        (call) async => null,
      );

      final result = await bridge.verify();
      expect(result.accepted, false);
      expect(result.error, isNotNull);
      expect(result.error!.code, 'BRIDGE_UNAVAILABLE');
    });

    testWidgets('repair returns bridge unavailable on null', (tester) async {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel(RuntimeBridgeContract.methodChannelName),
        (call) async => null,
      );

      final result = await bridge.repair();
      expect(result.accepted, false);
      expect(result.error, isNotNull);
      expect(result.error!.code, 'BRIDGE_UNAVAILABLE');
    });

    testWidgets('snapshot returns data on success', (tester) async {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel(RuntimeBridgeContract.methodChannelName),
        (call) async => {
          'schemaVersion': 1,
          'state': 'READY',
          'generation': 5,
          'runtimeInstalled': true,
          'runtimeAvailable': true,
        },
      );

      final result = await bridge.snapshot();
      expect(result.state, RuntimeBridgeState.ready);
      expect(result.generation, 5);
      expect(result.runtimeInstalled, true);
      expect(result.runtimeAvailable, true);
    });

    test('dispose cleans up resources', () async {
      await bridge.dispose();
    });
  });
}
