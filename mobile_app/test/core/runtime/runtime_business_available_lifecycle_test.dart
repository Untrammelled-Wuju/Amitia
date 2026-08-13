import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/status/default_runtime_status_projection.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_phase.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';

import 'status/fakes/fake_runtime_bridge.dart';
import 'status/fakes/fake_backend_connection_source.dart';
import 'status/fakes/fake_transport_state_source.dart';

void main() {
  group('businessAvailable Lifecycle Transitions', () {
    late FakeRuntimeBridge bridge;
    late FakeBackendConnectionSource connectionSource;
    late FakeTransportStateSource transportSource;

    setUp(() {
      bridge = FakeRuntimeBridge();
      connectionSource = FakeBackendConnectionSource();
      transportSource = FakeTransportStateSource();
    });

    tearDown(() async {
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
        credential: BackendConnectionCredential.tryCreate('a' * 32) ??
            (throw StateError('Failed to create credential')),
      );
    }

    Future<void> _pump() async {
      await Future.delayed(const Duration(milliseconds: 5));
    }

    test('businessAvailable=false during STOPPED', () async {
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopped,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));

      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();
      await _pump();

      expect(projection.current.businessAvailable, false);
      expect(projection.current.phase, RuntimeStatusPhase.unavailable);

      await projection.dispose();
    });

    test('businessAvailable=false during STARTING', () async {
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.starting,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));

      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();
      await _pump();

      expect(projection.current.businessAvailable, false);
      expect(projection.current.phase, RuntimeStatusPhase.starting);

      await projection.dispose();
    });

    test('businessAvailable=true during READY with full consistency', () async {
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));

      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(5)),
      );

      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();
      await _pump();

      transportSource.emit(const TransportStateSnapshot(
        generation: 5,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      expect(projection.current.businessAvailable, true);
      expect(projection.current.phase, RuntimeStatusPhase.ready);
      expect(projection.current.generation, 5);

      await projection.dispose();
    });

    test('businessAvailable=false during STOPPING', () async {
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopping,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));

      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();
      await _pump();

      expect(projection.current.businessAvailable, false);

      await projection.dispose();
    });

    test('businessAvailable=false during FAILED', () async {
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.failed,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));

      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();
      await _pump();

      expect(projection.current.businessAvailable, false);
      expect(projection.current.phase, RuntimeStatusPhase.failed);

      await projection.dispose();
    });

    test('businessAvailable transition: true -> false on crash', () async {
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));

      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(1)),
      );

      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();
      await _pump();

      transportSource.emit(const TransportStateSnapshot(
        generation: 1,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      expect(projection.current.businessAvailable, true);

      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.failed,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await _pump();

      expect(projection.current.businessAvailable, false);
      expect(projection.current.phase, RuntimeStatusPhase.failed);

      await projection.dispose();
    });

    test('businessAvailable transition: true -> false on stop', () async {
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));

      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(1)),
      );

      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();
      await _pump();

      transportSource.emit(const TransportStateSnapshot(
        generation: 1,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      expect(projection.current.businessAvailable, true);

      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.stopping,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await _pump();

      expect(projection.current.businessAvailable, false);

      await projection.dispose();
    });

    test('businessAvailable=false during recovery until new READY', () async {
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));

      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(1)),
      );

      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();
      await _pump();

      transportSource.emit(const TransportStateSnapshot(
        generation: 1,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      expect(projection.current.businessAvailable, true);

      // Crash
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.failed,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await _pump();
      expect(projection.current.businessAvailable, false);

      // Recovery starts
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.starting,
        generation: 2,
        runtimeInstalled: true,
        runtimeAvailable: false,
      ));
      await _pump();
      expect(projection.current.businessAvailable, false);

      // Recovery complete
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 2,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));
      await _pump();

      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(2)),
      );
      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 2,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));
      await _pump();

      transportSource.emit(const TransportStateSnapshot(
        generation: 2,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      expect(projection.current.businessAvailable, true);
      expect(projection.current.generation, 2);

      await projection.dispose();
    });
  });
}
