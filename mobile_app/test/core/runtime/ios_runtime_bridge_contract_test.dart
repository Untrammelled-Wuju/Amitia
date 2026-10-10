import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/providers/runtime_backend_connection_source.dart';
import 'package:amitia_app/core/runtime/method_channel_runtime_bridge.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  const methods = MethodChannel('com.amitia.runtime/bridge');
  const events = MethodChannel('com.amitia.runtime/events');

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(methods, null);
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(events, null);
  });

  Map<String, Object> connection(int generation) => {
    'schemaVersion': 1,
    'status': 'available',
    'generation': generation,
    'endpoint': {
      'host': '127.0.0.1',
      'port': 18899,
      'httpScheme': 'http',
      'webSocketScheme': 'ws',
      'livenessPath': '/livez',
      'readinessPath': '/readyz',
    },
    'authentication': {
      'type': 'local_token',
      'header': 'X-Amitia-Local-Token',
      'token': 'test-only-local-token-generation',
    },
  };

  test('真实连接解析器接收 iOS 同一份本机认证协议', () async {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(methods, (call) async => connection(8));
    final result = await const RuntimeBackendConnectionSource().resolve(
      expectedRuntimeGeneration: 8,
    );
    expect(result, isA<BackendConnectionAvailable>());
    final config = (result as BackendConnectionAvailable).config;
    expect(config.authStrategy, BackendAuthStrategy.localToken);
    expect(config.endpoint.port, 18899);
    expect(config.endpoint.host, '127.0.0.1');
  });

  test('后台中断后的旧 Runtime 代次不能重新取得凭据', () async {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(methods, (call) async => connection(9));
    expect(
      await const RuntimeBackendConnectionSource().resolve(
        expectedRuntimeGeneration: 8,
      ),
      isA<BackendConnectionUnavailable>(),
    );
  });

  test('Runtime 停止后拒绝提供本机认证与连接', () async {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
          methods,
          (call) async => {
            'schemaVersion': 1,
            'status': 'unavailable',
            'generation': 9,
            'error': {
              'code': 'RUNTIME_NOT_READY',
              'message': 'iOS Runtime 已停止',
            },
          },
        );
    expect(
      await const RuntimeBackendConnectionSource().resolve(),
      isA<BackendConnectionUnavailable>(),
    );
  });

  test('升级停机后宿主重启要求传到实际 Dart Bridge', () async {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(events, (call) async => null);
    const error = {
      'code': 'RESTART_REQUIRED',
      'message': '请关闭并重新打开应用',
      'retryable': false,
    };
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
          methods,
          (call) async => {
            'accepted': false,
            'error': error,
            'snapshot': {
              'schemaVersion': 1,
              'state': 'FAILED',
              'generation': 10,
              'runtimeInstalled': true,
              'runtimeAvailable': false,
              'lastError': error,
            },
          },
        );
    final bridge = MethodChannelRuntimeBridge();
    try {
      final result = await bridge.reconcileEmbedded();
      expect(result.accepted, isFalse);
      expect(result.snapshot.state, RuntimeBridgeState.failed);
      expect(result.error?.code, 'RESTART_REQUIRED');
      expect(result.error?.retryable, isFalse);
    } finally {
      await bridge.dispose();
    }
  });

  test('完整性校验拒绝通过实际 Dart Bridge 保留不可重试原因', () async {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(events, (call) async => null);
    const error = {
      'code': 'VERIFY_FAILED',
      'message': 'Runtime 校验失败，已停止 Core，请关闭重开后修复',
      'retryable': false,
    };
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
          methods,
          (call) async => {
            'accepted': false,
            'error': error,
            'snapshot': {
              'schemaVersion': 1,
              'state': 'FAILED',
              'generation': 11,
              'runtimeInstalled': true,
              'runtimeAvailable': false,
              'lastError': error,
            },
          },
        );
    final bridge = MethodChannelRuntimeBridge();
    try {
      final result = await bridge.verify();
      expect(result.accepted, isFalse);
      expect(result.snapshot.state, RuntimeBridgeState.failed);
      expect(result.snapshot.runtimeAvailable, isFalse);
      expect(result.error?.code, 'VERIFY_FAILED');
      expect(result.error?.retryable, isFalse);
      expect(result.snapshot.lastError?.retryable, isFalse);
    } finally {
      await bridge.dispose();
    }
  });
}
