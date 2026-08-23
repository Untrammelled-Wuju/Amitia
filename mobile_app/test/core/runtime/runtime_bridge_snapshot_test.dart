import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_error.dart';
import 'package:amitia_app/core/runtime/runtime_manifest_summary.dart';

void main() {
  group('RuntimeBridgeSnapshot', () {
    test('fromMap parses valid map', () {
      final map = <String, dynamic>{
        'schemaVersion': 1,
        'state': 'READY',
        'generation': 12,
        'runtimeInstalled': true,
        'runtimeAvailable': true,
        'lastError': null,
        'manifest': null,
      };
      final snapshot = RuntimeBridgeSnapshot.fromMap(map);
      expect(snapshot.schemaVersion, 1);
      expect(snapshot.state, RuntimeBridgeState.ready);
      expect(snapshot.generation, 12);
      expect(snapshot.runtimeInstalled, true);
      expect(snapshot.runtimeAvailable, true);
      expect(snapshot.lastError, isNull);
    });

    test('fromMap with error', () {
      final map = <String, dynamic>{
        'schemaVersion': 1,
        'state': 'FAILED',
        'generation': 5,
        'runtimeInstalled': true,
        'runtimeAvailable': false,
        'lastError': {
          'code': 'START_FAILED',
          'message': 'Failed to start runtime',
          'retryable': true,
        },
        'manifest': null,
      };
      final snapshot = RuntimeBridgeSnapshot.fromMap(map);
      expect(snapshot.lastError, isNotNull);
      expect(snapshot.lastError!.code, 'START_FAILED');
      expect(snapshot.lastError!.message, 'Failed to start runtime');
      expect(snapshot.lastError!.retryable, true);
    });

    test('fromMap with manifest', () {
      final map = <String, dynamic>{
        'schemaVersion': 1,
        'state': 'READY',
        'generation': 8,
        'runtimeInstalled': true,
        'runtimeAvailable': true,
        'lastError': null,
        'manifest': {
          'schemaVersion': 1,
          'runtimeVersion': '1.0.0',
          'packageId': 'test-package',
          'targetPlatform': 'android',
          'targetArch': 'arm64-v8a',
          'verified': true,
        },
      };
      final snapshot = RuntimeBridgeSnapshot.fromMap(map);
      expect(snapshot.manifest, isNotNull);
      expect(snapshot.manifest!.runtimeVersion, '1.0.0');
      expect(snapshot.manifest!.packageId, 'test-package');
      expect(snapshot.manifest!.verified, true);
    });

    test('initial creates default snapshot', () {
      final snapshot = RuntimeBridgeSnapshot.initial();
      expect(snapshot.schemaVersion, 1);
      expect(snapshot.state, RuntimeBridgeState.unavailable);
      expect(snapshot.generation, 0);
      expect(snapshot.runtimeInstalled, false);
      expect(snapshot.runtimeAvailable, false);
      expect(snapshot.lastError, isNull);
      expect(snapshot.manifest, isNull);
    });

    test('equality works correctly', () {
      final snapshot1 = RuntimeBridgeSnapshot.initial();
      final snapshot2 = RuntimeBridgeSnapshot.initial();
      expect(snapshot1, equals(snapshot2));
    });
  });

  group('RuntimeBridgeState', () {
    test('fromNative maps NOT_INSTALLED', () {
      expect(
        RuntimeBridgeState.fromNative('NOT_INSTALLED'),
        RuntimeBridgeState.notInstalled,
      );
    });

    test('fromNative maps READY', () {
      expect(RuntimeBridgeState.fromNative('READY'), RuntimeBridgeState.ready);
    });

    test('fromNative maps STARTING', () {
      expect(
        RuntimeBridgeState.fromNative('STARTING'),
        RuntimeBridgeState.starting,
      );
    });

    test('fromNative maps INSTALLING', () {
      expect(
        RuntimeBridgeState.fromNative('INSTALLING'),
        RuntimeBridgeState.installing,
      );
    });

    test('fromNative maps STOPPING', () {
      expect(
        RuntimeBridgeState.fromNative('STOPPING'),
        RuntimeBridgeState.stopping,
      );
    });

    test('fromNative maps FAILED', () {
      expect(
        RuntimeBridgeState.fromNative('FAILED'),
        RuntimeBridgeState.failed,
      );
    });

    test('fromNative maps CORRUPTED to failed', () {
      expect(
        RuntimeBridgeState.fromNative('CORRUPTED'),
        RuntimeBridgeState.failed,
      );
    });

    test('fromNative maps null to unavailable', () {
      expect(
        RuntimeBridgeState.fromNative(null),
        RuntimeBridgeState.unavailable,
      );
    });

    test('fromNative maps unknown value to unavailable', () {
      expect(
        RuntimeBridgeState.fromNative('SOME_NEW_STATE'),
        RuntimeBridgeState.unavailable,
      );
    });
  });

  group('RuntimeBridgeError', () {
    test('fromMap parses valid map', () {
      final map = <String, dynamic>{
        'code': 'START_FAILED',
        'message': 'Start failed',
        'retryable': true,
      };
      final error = RuntimeBridgeError.fromMap(map);
      expect(error.code, 'START_FAILED');
      expect(error.message, 'Start failed');
      expect(error.retryable, true);
    });

    test('fromMap handles null', () {
      final error = RuntimeBridgeError.fromMap(null);
      expect(error.code, 'UNKNOWN');
      expect(error.retryable, false);
    });

    test('tryFromMap returns null for null', () {
      final error = RuntimeBridgeError.tryFromMap(null);
      expect(error, isNull);
    });

    test('equality works correctly', () {
      final error1 = RuntimeBridgeError(
        code: 'TEST',
        message: 'test',
        retryable: true,
      );
      final error2 = RuntimeBridgeError(
        code: 'TEST',
        message: 'test',
        retryable: true,
      );
      expect(error1, equals(error2));
    });
  });

  group('RuntimeManifestSummary', () {
    test('fromMap parses valid map', () {
      final map = <String, dynamic>{
        'schemaVersion': 1,
        'runtimeVersion': '1.0.0',
        'packageId': 'test-package',
        'targetPlatform': 'android',
        'targetArch': 'arm64-v8a',
        'verified': true,
      };
      final summary = RuntimeManifestSummary.fromMap(map);
      expect(summary.schemaVersion, 1);
      expect(summary.runtimeVersion, '1.0.0');
      expect(summary.packageId, 'test-package');
      expect(summary.targetPlatform, 'android');
      expect(summary.targetArch, 'arm64-v8a');
      expect(summary.verified, true);
    });

    test('fromMap handles null', () {
      final summary = RuntimeManifestSummary.fromMap(null);
      expect(summary.schemaVersion, 0);
      expect(summary.verified, false);
    });
  });
}
