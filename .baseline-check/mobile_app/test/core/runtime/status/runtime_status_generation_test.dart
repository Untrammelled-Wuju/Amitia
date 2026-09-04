import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/status/default_runtime_status_projection.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_phase.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';

import 'fakes/fake_runtime_bridge.dart';
import 'fakes/fake_backend_connection_source.dart';
import 'fakes/fake_transport_state_source.dart';

void main() {
  group('RuntimeStatus Generation Consistency', () {
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

    test('Same generation across all sources yields ready', () async {
      const bridgeSnapshot = RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 10,
        runtimeInstalled: true,
        runtimeAvailable: true,
      );
      bridge.setSnapshot(bridgeSnapshot);
      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(10)),
      );

      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();
      await _pump();

      transportSource.emit(const TransportStateSnapshot(
        generation: 10,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      expect(projection.current.generation, 10);
      expect(projection.current.phase, RuntimeStatusPhase.ready);

      await projection.dispose();
    });

    test('Old HTTP generation does not yield ready', () async {
      const bridgeSnapshot = RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 10,
        runtimeInstalled: true,
        runtimeAvailable: true,
      );
      bridge.setSnapshot(bridgeSnapshot);
      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(10)),
      );

      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();
      await _pump();

      transportSource.emit(const TransportStateSnapshot(
        generation: 9,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));
      await _pump();

      expect(projection.current.phase, RuntimeStatusPhase.degraded);
      expect(projection.current.businessAvailable, false);

      await projection.dispose();
    });

    test('Projection does not create its own generation', () async {
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

      expect(projection.current.generation, 5);

      await projection.dispose();
    });
  });
}
