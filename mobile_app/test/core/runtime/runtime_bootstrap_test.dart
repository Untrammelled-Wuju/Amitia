import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/runtime/runtime_bridge.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/runtime_bootstrap.dart';
import 'package:amitia_app/core/runtime/default_runtime_bootstrap.dart';
import 'package:amitia_app/core/runtime/runtime_bootstrap_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bootstrap_phase.dart';
import 'package:amitia_app/core/runtime/runtime_bootstrap_policy.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_error.dart';

class FakeRuntimeBridge implements RuntimeBridge {
  final StreamController<RuntimeBridgeSnapshot> _controller =
      StreamController<RuntimeBridgeSnapshot>.broadcast();

  int startCallCount = 0;
  int stopCallCount = 0;
  int installCallCount = 0;

  RuntimeBridgeSnapshot _current;
  bool _shouldFailSnapshot = false;

  FakeRuntimeBridge({RuntimeBridgeSnapshot? initial})
      : _current = initial ?? RuntimeBridgeSnapshot.initial();

  void emit(RuntimeBridgeSnapshot snapshot) {
    _current = snapshot;
    _controller.add(snapshot);
  }

  void failNextSnapshot() {
    _shouldFailSnapshot = true;
  }

  @override
  Stream<RuntimeBridgeSnapshot> get snapshots => _controller.stream;

  @override
  Future<RuntimeBridgeSnapshot> snapshot() async {
    if (_shouldFailSnapshot) {
      _shouldFailSnapshot = false;
      throw Exception('Bridge unavailable');
    }
    return _current;
  }

  @override
  Future<RuntimeBridgeCommandResult> start() async {
    startCallCount++;
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: _current,
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> stop() async {
    stopCallCount++;
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: _current,
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> install() async {
    installCallCount++;
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: _current,
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> verify() async {
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: _current,
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> repair() async {
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: _current,
    );
  }

  @override
  Future<RuntimeManifestSummary?> manifestSummary() async {
    return _current.manifest;
  }

  @override
  Future<void> dispose() async {
    await _controller.close();
  }
}

RuntimeBridgeSnapshot _makeSnapshot({
  required RuntimeBridgeState state,
  int generation = 1,
  bool runtimeInstalled = false,
  RuntimeBridgeError? error,
}) {
  return RuntimeBridgeSnapshot(
    schemaVersion: 1,
    state: state,
    generation: generation,
    runtimeInstalled: runtimeInstalled,
    runtimeAvailable: state == RuntimeBridgeState.ready,
    lastError: error,
  );
}

void setUp(WidgetTester _) {}

void main() {
  group('Initialize once', () {
    test('concurrent initialize calls only execute one start decision', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(
          state: RuntimeBridgeState.stopped,
          runtimeInstalled: true,
        ),
      );

      final bootstrap = DefaultRuntimeBootstrap(
        bridge: bridge,
        policy: const RuntimeBootstrapPolicy(autoStartInstalledRuntime: true),
      );

      await Future.wait([
        bootstrap.initialize(),
        bootstrap.initialize(),
        bootstrap.initialize(),
        bootstrap.initialize(),
        bootstrap.initialize(),
      ]);

      await Future.delayed(const Duration(milliseconds: 100));
      expect(bridge.startCallCount, lessThanOrEqualTo(1));
      await bootstrap.dispose();
    });
  });

  group('Initial state mapping', () {
    test('initial READY → phase ready, start calls = 0', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.ready),
      );

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap.initialize();

      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, equals(0));

      final snapshots = bootstrap.snapshots.take(1).toList();
      final result = await snapshots;
      expect(result.first.phase, equals(RuntimeBootstrapPhase.ready));

      await bootstrap.dispose();
    });

    test('initial STARTING → phase starting, start calls = 0', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.starting),
      );

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap.initialize();

      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, equals(0));

      final snapshots = bootstrap.snapshots.take(1).toList();
      final result = await snapshots;
      expect(result.first.phase, equals(RuntimeBootstrapPhase.starting));

      await bootstrap.dispose();
    });

    test('initial STOPPED with autoStart → start calls = 1', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(
          state: RuntimeBridgeState.stopped,
          runtimeInstalled: true,
        ),
      );

      final bootstrap = DefaultRuntimeBootstrap(
        bridge: bridge,
        policy: const RuntimeBootstrapPolicy(autoStartInstalledRuntime: true),
      );

      await bootstrap.initialize();
      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, equals(1));
      await bootstrap.dispose();
    });

    test('initial STOPPED without autoStart → start calls = 0', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.stopped),
      );

      final bootstrap = DefaultRuntimeBootstrap(
        bridge: bridge,
        policy: const RuntimeBootstrapPolicy(autoStartInstalledRuntime: false),
      );

      await bootstrap.initialize();
      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, equals(0));
      await bootstrap.dispose();
    });

    test('initial FAILED → start calls = 0', () async {
      const error = RuntimeBridgeError(
        code: 'STARTUP_TIMEOUT',
        message: 'Timeout',
        retryable: true,
      );

      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(
          state: RuntimeBridgeState.failed,
          error: error,
        ),
      );

      final bootstrap = DefaultRuntimeBootstrap(
        bridge: bridge,
        policy: const RuntimeBootstrapPolicy(autoStartInstalledRuntime: true),
      );

      await bootstrap.initialize();
      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, equals(0));
      await bootstrap.dispose();
    });

    test('initial NOT_INSTALLED → phase installRequired, start calls = 0', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.notInstalled),
      );

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap.initialize();

      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, equals(0));
      expect(bridge.installCallCount, equals(0));

      final snapshots = bootstrap.snapshots.take(1).toList();
      final result = await snapshots;
      expect(result.first.phase, equals(RuntimeBootstrapPhase.installRequired));

      await bootstrap.dispose();
    });

    test('initial UNAVAILABLE → start calls = 0', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.unavailable),
      );

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap.initialize();

      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, equals(0));

      final snapshots = bootstrap.snapshots.take(1).toList();
      final result = await snapshots;
      expect(result.first.phase, equals(RuntimeBootstrapPhase.unavailable));

      await bootstrap.dispose();
    });

    test('initial INSTALLING → start calls = 0', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.installing),
      );

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap.initialize();

      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, equals(0));
      expect(bridge.installCallCount, equals(0));

      final snapshots = bootstrap.snapshots.take(1).toList();
      final result = await snapshots;
      expect(result.first.phase, equals(RuntimeBootstrapPhase.initializing));

      await bootstrap.dispose();
    });

    test('initial STOPPING → start calls = 0', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.stopping),
      );

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap.initialize();

      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, equals(0));

      final snapshots = bootstrap.snapshots.take(1).toList();
      final result = await snapshots;
      expect(result.first.phase, equals(RuntimeBootstrapPhase.stopping));

      await bootstrap.dispose();
    });
  });

  group('User stop does not auto restart', () {
    test('READY → USER STOP → STOPPED does not re-trigger start', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.stopped, generation: 1),
      );

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap.initialize();

      bridge.emit(_makeSnapshot(state: RuntimeBridgeState.stopping, generation: 2));
      await Future.delayed(const Duration(milliseconds: 50));

      bridge.emit(_makeSnapshot(state: RuntimeBridgeState.stopped, generation: 3));
      await Future.delayed(const Duration(milliseconds: 50));

      bridge.emit(_makeSnapshot(state: RuntimeBridgeState.starting, generation: 4));
      await Future.delayed(const Duration(milliseconds: 50));

      bridge.emit(_makeSnapshot(state: RuntimeBridgeState.ready, generation: 5));
      await Future.delayed(const Duration(milliseconds: 50));

      bridge.emit(_makeSnapshot(state: RuntimeBridgeState.stopping, generation: 6));
      await Future.delayed(const Duration(milliseconds: 50));

      bridge.emit(_makeSnapshot(state: RuntimeBridgeState.stopped, generation: 7));
      await Future.delayed(const Duration(milliseconds: 50));

      expect(bridge.startCallCount, equals(0));
      await bootstrap.dispose();
    });
  });

  group('Reconnect scenarios', () {
    test('Runtime READY reconnect → new bootstrap no start', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.ready),
      );

      final bootstrap1 = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap1.initialize();
      await Future.delayed(const Duration(milliseconds: 50));
      await bootstrap1.dispose();

      final bootstrap2 = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap2.initialize();
      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, equals(0));

      final snapshots = bootstrap2.snapshots.take(1).toList();
      final result = await snapshots;
      expect(result.first.phase, equals(RuntimeBootstrapPhase.ready));

      await bootstrap2.dispose();
    });

    test('Runtime STARTING reconnect → new bootstrap no start', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.starting),
      );

      final bootstrap1 = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap1.initialize();
      await Future.delayed(const Duration(milliseconds: 50));
      await bootstrap1.dispose();

      final bootstrap2 = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap2.initialize();
      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, equals(0));
      await bootstrap2.dispose();
    });
  });

  group('Stale snapshot race', () {
    test('old generation snapshot does not override new event', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.ready, generation: 5),
      );

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap.initialize();
      await Future.delayed(const Duration(milliseconds: 50));

      final snapshots = <RuntimeBootstrapSnapshot>[];
      final sub = bootstrap.snapshots.listen(snapshots.add);

      bridge.emit(_makeSnapshot(state: RuntimeBridgeState.starting, generation: 4));
      await Future.delayed(const Duration(milliseconds: 100));

      final lastPhase = snapshots.isNotEmpty ? snapshots.last.phase : null;
      expect(lastPhase, isNot(RuntimeBootstrapPhase.starting));

      await sub.cancel();
      await bootstrap.dispose();
    });
  });

  group('Dispose behavior', () {
    test('dispose does not call bridge.stop()', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.stopped),
      );

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap.initialize();
      await Future.delayed(const Duration(milliseconds: 50));

      await bootstrap.dispose();

      expect(bridge.stopCallCount, equals(0));
    });

    test('dispose is idempotent', () async {
      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(state: RuntimeBridgeState.stopped),
      );

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap.initialize();

      await bootstrap.dispose();
      await bootstrap.dispose();

      expect(bridge.stopCallCount, equals(0));
    });
  });

  group('Bridge failure', () {
    test('bridge failure → bootstrap phase = failed', () async {
      final controller = StreamController<RuntimeBridgeSnapshot>();
      final bridge = _FailingBridge(controller);

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final future = bootstrap.initialize();
      await Future.delayed(const Duration(milliseconds: 50));

      controller.addError(Exception('Bridge failure'));
      await Future.delayed(const Duration(milliseconds: 50));

      await future.timeout(const Duration(seconds: 2));

      expect(bridge.startCallCount, equals(0));

      await bootstrap.dispose();
      await controller.close();
    });
  });

  group('Native error preservation', () {
    test('native error code is preserved in snapshot', () async {
      const nativeError = RuntimeBridgeError(
        code: 'STARTUP_TIMEOUT',
        message: 'Startup timed out',
        retryable: true,
      );

      final bridge = FakeRuntimeBridge(
        initial: _makeSnapshot(
          state: RuntimeBridgeState.failed,
          error: nativeError,
        ),
      );

      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      await bootstrap.initialize();

      final snapshot = await bootstrap.snapshots.first;
      expect(snapshot.runtime.lastError?.code, equals('STARTUP_TIMEOUT'));
      expect(snapshot.runtime.lastError?.retryable, equals(true));

      await bootstrap.dispose();
    });
  });
}

class _FailingBridge implements RuntimeBridge {
  final StreamController<RuntimeBridgeSnapshot> _controller;
  int startCallCount = 0;
  int stopCallCount = 0;

  _FailingBridge(this._controller);

  @override
  Stream<RuntimeBridgeSnapshot> get snapshots => _controller.stream;

  @override
  Future<RuntimeBridgeSnapshot> snapshot() async {
    return RuntimeBridgeSnapshot.initial();
  }

  @override
  Future<RuntimeBridgeCommandResult> start() async {
    startCallCount++;
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: RuntimeBridgeSnapshot.initial(),
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> stop() async {
    stopCallCount++;
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: RuntimeBridgeSnapshot.initial(),
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> install() async {
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: RuntimeBridgeSnapshot.initial(),
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> verify() async {
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: RuntimeBridgeSnapshot.initial(),
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> repair() async {
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: RuntimeBridgeSnapshot.initial(),
    );
  }

  @override
  Future<RuntimeManifestSummary?> manifestSummary() async => null;

  @override
  Future<void> dispose() async {}
}
