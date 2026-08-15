import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/status/default_runtime_status_projection.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_snapshot.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_phase.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_projection.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';

import 'fakes/fake_runtime_bridge.dart';
import 'fakes/fake_backend_connection_source.dart';
import 'fakes/fake_transport_state_source.dart';

void main() {
  group('DefaultRuntimeStatusProjection', () {
    late FakeRuntimeBridge bridge;
    late FakeBackendConnectionSource connectionSource;
    late FakeTransportStateSource transportSource;
    late DefaultRuntimeStatusProjection projection;

    setUp(() {
      bridge = FakeRuntimeBridge();
      connectionSource = FakeBackendConnectionSource();
      transportSource = FakeTransportStateSource();
      projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );
    });

    tearDown(() async {
      await projection.dispose();
      await transportSource.close();
    });

    BackendConnectionConfig _makeConfig(int generation) {
      return BackendConnectionConfig(
        schemaVersion: 1,
        generation: generation,
        endpoint: BackendConnectionEndpoint(
          host: '127.0.0.1',
          port: 18899,
          httpScheme: 'http',
          webSocketScheme: 'ws',
          livenessPath: '/livez',
          readinessPath: '/readyz',
        ),
        authStrategy: BackendAuthStrategy.localToken,
        credential: BackendConnectionCredential.tryCreate(
              'a' * 32,
            ) ??
            (throw StateError('Failed to create credential')),
      );
    }

    Future<void> _pump() async {
      await Future.delayed(const Duration(milliseconds: 5));
    }

    test('initial snapshot is unavailable before initialize', () {
      expect(projection.current.phase, RuntimeStatusPhase.unavailable);
      expect(projection.current.runtimeReady, false);
      expect(projection.current.businessAvailable, false);
    });

    test('does not call runtime controller methods', () async {
      await projection.initialize();
      await _pump();
      expect(bridge.startCallCount, 0);
      expect(bridge.stopCallCount, 0);
      expect(bridge.installCallCount, 0);
      expect(bridge.verifyCallCount, 0);
      expect(bridge.repairCallCount, 0);
    });

    test('CASE 1: Fully ready projects ready', () async {
      const bridgeSnapshot = RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
      );
      bridge.setSnapshot(bridgeSnapshot);
      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(5)),
      );

      await projection.initialize();
      await _pump();
      transportSource.emit(const TransportStateSnapshot(
        generation: 5,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      expect(projection.current.phase, RuntimeStatusPhase.ready);
      expect(projection.current.runtimeReady, true);
      expect(projection.current.runtimeInstalled, true);
      expect(projection.current.backendConfigured, true);
      expect(projection.current.httpAvailable, true);
      expect(projection.current.webSocketConnected, true);
      expect(projection.current.businessAvailable, true);
      expect(projection.current.primaryError, isNull);
    });

    test('CASE 2: Starting with stale transport projects starting', () async {
      const bridgeSnapshot = RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.starting,
        generation: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
      );
      bridge.setSnapshot(bridgeSnapshot);
      connectionSource.setAvailability(BackendConnectionUnavailable());

      await projection.initialize();
      await _pump();
      transportSource.emit(const TransportStateSnapshot(
        generation: 4,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      expect(projection.current.phase, RuntimeStatusPhase.starting);
      expect(projection.current.runtimeReady, false);
      expect(projection.current.businessAvailable, false);
    });

    test('CASE 3: Runtime failed with connected transport projects failed',
        () async {
      const bridgeSnapshot = RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.failed,
        generation: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
      );
      bridge.setSnapshot(bridgeSnapshot);
      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(5)),
      );

      await projection.initialize();
      await _pump();
      transportSource.emit(const TransportStateSnapshot(
        generation: 5,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      expect(projection.current.phase, RuntimeStatusPhase.failed);
      expect(projection.current.runtimeReady, false);
      expect(projection.current.businessAvailable, false);
    });

    test('CASE 4: Runtime ready with HTTP unavailable projects degraded',
        () async {
      const bridgeSnapshot = RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
      );
      bridge.setSnapshot(bridgeSnapshot);
      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(5)),
      );

      await projection.initialize();
      await _pump();
      transportSource.emit(const TransportStateSnapshot(
        generation: 5,
        httpState: BackendHttpState.unavailable,
        webSocketState: BackendWebSocketState.disconnected,
      ));
      await _pump();

      expect(projection.current.phase, RuntimeStatusPhase.degraded);
      expect(projection.current.runtimeReady, true);
      expect(projection.current.httpAvailable, false);
      expect(projection.current.businessAvailable, false);
    });

    test('CASE 9: Generation mismatch does not project ready', () async {
      const bridgeSnapshot = RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 8,
        runtimeInstalled: true,
        runtimeAvailable: true,
      );
      bridge.setSnapshot(bridgeSnapshot);
      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(8)),
      );

      await projection.initialize();
      await _pump();
      transportSource.emit(const TransportStateSnapshot(
        generation: 7,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      expect(projection.current.phase, RuntimeStatusPhase.degraded);
      expect(projection.current.businessAvailable, false);
      expect(projection.current.primaryError?.code, 'GENERATION_MISMATCH');
    });

    test('dispose does not call runtime controller', () async {
      await projection.initialize();
      await _pump();
      await projection.dispose();
      expect(bridge.startCallCount, 0);
      expect(bridge.stopCallCount, 0);
      expect(bridge.disposeCallCount, 0);
    });

    test('snapshots stream emits distinct values only', () async {
      final emissions = <RuntimeStatusSnapshot>[];
      projection.snapshots.listen(emissions.add);

      const bridgeSnapshot = RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
      );
      bridge.setSnapshot(bridgeSnapshot);
      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(5)),
      );

      await projection.initialize();
      await _pump();

      transportSource.emit(const TransportStateSnapshot(
        generation: 5,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      transportSource.emit(const TransportStateSnapshot(
        generation: 5,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      await Future.delayed(const Duration(milliseconds: 10));

      final readyEmissions =
          emissions.where((e) => e.phase == RuntimeStatusPhase.ready).length;
      expect(readyEmissions, 1);
    });
  });
}
