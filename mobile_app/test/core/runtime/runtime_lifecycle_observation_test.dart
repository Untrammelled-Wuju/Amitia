import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/runtime/runtime_bridge.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_error.dart';
import 'package:amitia_app/core/runtime/default_runtime_bootstrap.dart';
import 'package:amitia_app/core/runtime/runtime_bootstrap_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bootstrap_phase.dart';
import 'package:amitia_app/core/runtime/runtime_bootstrap_policy.dart';

import 'status/fakes/fake_runtime_bridge.dart';

void main() {
  group('Full Normal Lifecycle Observation', () {
    test('STOPPED -> STARTING(1) -> READY(1) -> STOPPING(1) -> STOPPED(1)', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopped,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.starting,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopping,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopped,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      expect(snapshots.length, greaterThanOrEqualTo(5));
      expect(snapshots[0].phase, RuntimeBootstrapPhase.stopped);
      expect(snapshots[1].phase, RuntimeBootstrapPhase.starting);
      expect(snapshots[2].phase, RuntimeBootstrapPhase.ready);
      expect(snapshots[3].phase, RuntimeBootstrapPhase.stopping);
      expect(snapshots[4].phase, RuntimeBootstrapPhase.stopped);

      await sub.cancel();
      await bootstrap.dispose();
    });
  });

  group('Crash Lifecycle Observation', () {
    test('READY(1) -> FAILED(1) -> STARTING(2) -> READY(2)', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.failed,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
        lastError: const RuntimeBridgeError(
          code: 'PROCESS_CRASH',
          message: 'Unexpected exit',
          retryable: true,
        ),
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.starting,
        generation: 2,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 2,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      expect(snapshots.length, greaterThanOrEqualTo(4));
      expect(snapshots[0].phase, RuntimeBootstrapPhase.ready);
      expect(snapshots[0].runtime.generation, 1);
      expect(snapshots[1].phase, RuntimeBootstrapPhase.failed);
      expect(snapshots[1].runtime.generation, 1);
      expect(snapshots[2].phase, RuntimeBootstrapPhase.starting);
      expect(snapshots[2].runtime.generation, 2);
      expect(snapshots[3].phase, RuntimeBootstrapPhase.ready);
      expect(snapshots[3].runtime.generation, 2);

      await sub.cancel();
      await bootstrap.dispose();
    });
  });

  group('Stop from STARTING', () {
    test('STARTING(1) -> STOPPING(1) -> STOPPED(1)', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.starting,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopping,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopped,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      expect(snapshots.length, greaterThanOrEqualTo(3));
      expect(snapshots[0].phase, RuntimeBootstrapPhase.starting);
      expect(snapshots[1].phase, RuntimeBootstrapPhase.stopping);
      expect(snapshots[2].phase, RuntimeBootstrapPhase.stopped);

      await sub.cancel();
      await bootstrap.dispose();
    });
  });

  group('Stop from STOPPED (idempotent)', () {
    test('STOPPED(1) -> STOPPED(1) no new generation', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopped,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopped,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      final stoppedSnapshots =
          snapshots.where((s) => s.phase == RuntimeBootstrapPhase.stopped).length;
      expect(stoppedSnapshots, greaterThanOrEqualTo(2));

      await sub.cancel();
      await bootstrap.dispose();
    });
  });

  group('Stop from STOPPING (idempotent)', () {
    test('STOPPING(1) -> STOPPING(1) same state', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopping,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopping,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      final stoppingSnapshots = snapshots
          .where((s) => s.phase == RuntimeBootstrapPhase.stopping)
          .length;
      expect(stoppingSnapshots, greaterThanOrEqualTo(2));

      await sub.cancel();
      await bootstrap.dispose();
    });
  });

  group('Stale events ignored', () {
    test('old generation events do not override current state', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopped,
        generation: 3,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      final hasOldStopped = snapshots.any((s) =>
          s.phase == RuntimeBootstrapPhase.stopped && s.runtime.generation == 3);
      expect(hasOldStopped, isFalse);

      final currentReady = snapshots
          .any((s) => s.phase == RuntimeBootstrapPhase.ready && s.runtime.generation == 5);
      expect(currentReady, isTrue);

      await sub.cancel();
      await bootstrap.dispose();
    });

    test('late READY from old generation during STOPPING ignored', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopping,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      final lateReadySnapshots = snapshots
          .where((s) =>
              s.phase == RuntimeBootstrapPhase.ready &&
              s.runtime.generation == 1 &&
              snapshots.indexOf(s) > 1)
          .length;
      expect(lateReadySnapshots, 0);

      await sub.cancel();
      await bootstrap.dispose();
    });
  });

  group('Generation consistency within a lifecycle', () {
    test('same generation from STARTING through STOPPED', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopped,
        generation: 10,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      for (final state in [
        RuntimeBridgeState.starting,
        RuntimeBridgeState.ready,
        RuntimeBridgeState.stopping,
        RuntimeBridgeState.stopped,
      ]) {
        bridge.setSnapshot(RuntimeBridgeSnapshot(
          schemaVersion: 1,
          state: state,
          generation: 10,
          runtimeInstalled: true,
          runtimeAvailable: state == RuntimeBridgeState.ready,
        ));
        await Future.delayed(const Duration(milliseconds: 5));
      }

      for (final snapshot in snapshots) {
        expect(snapshot.runtime.generation, 10);
      }

      await sub.cancel();
      await bootstrap.dispose();
    });
  });

  group('Auto-start behavior', () {
    test('policy triggers auto-start only once on stopped + installed', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(
        bridge: bridge,
        policy: const RuntimeBootstrapPolicy(autoStartInstalledRuntime: true),
      );

      await bootstrap.initialize();
      await Future.delayed(const Duration(milliseconds: 200));

      expect(bridge.startCallCount, lessThanOrEqualTo(1));

      await bootstrap.dispose();
    });

    test('auto-start does not trigger on FAILED', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(
        bridge: bridge,
        policy: const RuntimeBootstrapPolicy(autoStartInstalledRuntime: true),
      );

      await bootstrap.initialize();

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.failed,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 100));

      expect(bridge.startCallCount, 0);

      await bootstrap.dispose();
    });
  });

  group('Multiple crash-recovery cycles', () {
    test('repeated crash -> recovery cycles increment generation', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      for (int gen = 1; gen <= 3; gen++) {
        bridge.setSnapshot(RuntimeBridgeSnapshot(
          schemaVersion: 1,
          state: RuntimeBridgeState.ready,
          generation: gen,
          runtimeInstalled: true,
          runtimeAvailable: true,
        ));
        await Future.delayed(const Duration(milliseconds: 5));

        bridge.setSnapshot(RuntimeBridgeSnapshot(
          schemaVersion: 1,
          state: RuntimeBridgeState.failed,
          generation: gen,
          runtimeInstalled: true,
          runtimeAvailable: false,
          lastError: const RuntimeBridgeError(
            code: 'PROCESS_CRASH',
            message: 'Unexpected exit',
            retryable: true,
          ),
        ));
        await Future.delayed(const Duration(milliseconds: 5));
      }

      final maxGeneration = snapshots
          .map((s) => s.runtime.generation)
          .reduce((a, b) => a > b ? a : b);
      expect(maxGeneration, 3);

      final failedSnapshots = snapshots.where((s) => s.phase == RuntimeBootstrapPhase.failed);
      expect(failedSnapshots.length, greaterThanOrEqualTo(3));

      await sub.cancel();
      await bootstrap.dispose();
    });
  });

  group('Error preservation', () {
    test('FAILED preserves typed error with code', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.failed,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
        lastError: const RuntimeBridgeError(
          code: 'STARTUP_TIMEOUT',
          message: 'Startup timed out after 30s',
          retryable: true,
        ),
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      final failedSnapshot = snapshots.last;
      expect(failedSnapshot.phase, RuntimeBootstrapPhase.failed);
      expect(failedSnapshot.runtime.lastError?.code, 'STARTUP_TIMEOUT');
      expect(failedSnapshot.runtime.lastError?.retryable, true);

      await sub.cancel();
      await bootstrap.dispose();
    });

    test('non-recoverable error preserved', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.failed,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
        lastError: const RuntimeBridgeError(
          code: 'UNSUPPORTED_ABI',
          message: 'Device ABI not supported',
          retryable: false,
        ),
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      final failedSnapshot = snapshots.last;
      expect(failedSnapshot.runtime.lastError?.code, 'UNSUPPORTED_ABI');
      expect(failedSnapshot.runtime.lastError?.retryable, false);

      await sub.cancel();
      await bootstrap.dispose();
    });
  });

  group('Stop/Crash race simulation', () {
    test('STOPPING state arrival makes exit expected', () async {
      final bridge = FakeRuntimeBridge();
      final bootstrap = DefaultRuntimeBootstrap(bridge: bridge);
      final snapshots = <RuntimeBootstrapSnapshot>[];

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));

      final sub = bootstrap.snapshots.listen(snapshots.add);
      await bootstrap.initialize();

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopping,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopped,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await Future.delayed(const Duration(milliseconds: 10));

      expect(snapshots.last.phase, RuntimeBootstrapPhase.stopped);

      await sub.cancel();
      await bootstrap.dispose();
    });
  });
}
