import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/status/default_runtime_status_projection.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_phase.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_error.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_snapshot.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';

import 'fakes/fake_runtime_bridge.dart';
import 'fakes/fake_backend_connection_source.dart';
import 'fakes/fake_transport_state_source.dart';

void main() {
  group('RuntimeStatus Truth Table', () {
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

    Future<RuntimeStatusSnapshot> _buildAndSnapshot({
      required RuntimeBridgeState runtimeState,
      required int runtimeGeneration,
      required bool runtimeInstalled,
      required bool runtimeAvailable,
      required BackendConnectionAvailability connection,
      required int transportGeneration,
      required BackendHttpState httpState,
      required BackendWebSocketState webSocketState,
    }) async {
      bridge.setSnapshot(RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: runtimeState,
        generation: runtimeGeneration,
        runtimeInstalled: runtimeInstalled,
        runtimeAvailable: runtimeAvailable,
      ));
      connectionSource.setAvailability(connection);

      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();
      await _pump();

      transportSource.emit(TransportStateSnapshot(
        generation: transportGeneration,
        httpState: httpState,
        webSocketState: webSocketState,
      ));
      await _pump();

      final snapshot = projection.current;
      await projection.dispose();
      return snapshot;
    }

    test('Case 5: Runtime ready with WS disconnected = degraded, business true',
        () async {
      final snapshot = await _buildAndSnapshot(
        runtimeState: RuntimeBridgeState.ready,
        runtimeGeneration: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
        connection: BackendConnectionAvailable(_makeConfig(5)),
        transportGeneration: 5,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.disconnected,
      );

      expect(snapshot.phase, RuntimeStatusPhase.degraded);
      expect(snapshot.runtimeReady, true);
      expect(snapshot.httpAvailable, true);
      expect(snapshot.webSocketConnected, false);
      expect(snapshot.businessAvailable, true);
    });

    test('Case 5b: Runtime ready with WS connecting = degraded', () async {
      final snapshot = await _buildAndSnapshot(
        runtimeState: RuntimeBridgeState.ready,
        runtimeGeneration: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
        connection: BackendConnectionAvailable(_makeConfig(5)),
        transportGeneration: 5,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connecting,
      );

      expect(snapshot.phase, RuntimeStatusPhase.degraded);
      expect(snapshot.runtimeReady, true);
      expect(snapshot.businessAvailable, true);
    });

    test('Case 6: Not installed returns installRequired', () async {
      final snapshot = await _buildAndSnapshot(
        runtimeState: RuntimeBridgeState.notInstalled,
        runtimeGeneration: 0,
        runtimeInstalled: false,
        runtimeAvailable: false,
        connection: BackendConnectionUnavailable(),
        transportGeneration: 5,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      );

      expect(snapshot.phase, RuntimeStatusPhase.installRequired);
      expect(snapshot.runtimeInstalled, false);
      expect(snapshot.businessAvailable, false);
    });

    test('Case 7: Runtime unavailable returns unavailable', () async {
      final snapshot = await _buildAndSnapshot(
        runtimeState: RuntimeBridgeState.unavailable,
        runtimeGeneration: 0,
        runtimeInstalled: false,
        runtimeAvailable: false,
        connection: BackendConnectionUnavailable(),
        transportGeneration: 0,
        httpState: BackendHttpState.closed,
        webSocketState: BackendWebSocketState.closed,
      );

      expect(snapshot.phase, RuntimeStatusPhase.unavailable);
      expect(snapshot.runtimeReady, false);
      expect(snapshot.businessAvailable, false);
    });

    test('Case 8: Runtime ready with invalid config returns degraded',
        () async {
      final snapshot = await _buildAndSnapshot(
        runtimeState: RuntimeBridgeState.ready,
        runtimeGeneration: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
        connection: BackendConnectionUnavailable(),
        transportGeneration: 0,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      );

      expect(snapshot.phase, RuntimeStatusPhase.degraded);
      expect(snapshot.runtimeReady, true);
      expect(snapshot.backendConfigured, false);
      expect(snapshot.businessAvailable, false);
    });

    test('Case 10: HTTP new generation ready, WS old generation', () async {
      final snapshot = await _buildAndSnapshot(
        runtimeState: RuntimeBridgeState.ready,
        runtimeGeneration: 8,
        runtimeInstalled: true,
        runtimeAvailable: true,
        connection: BackendConnectionAvailable(_makeConfig(8)),
        transportGeneration: 7,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      );

      expect(snapshot.phase, RuntimeStatusPhase.degraded);
      expect(snapshot.businessAvailable, false);
      expect(snapshot.primaryError?.code, 'GENERATION_MISMATCH');
    });

    test('Case 11: Runtime stopping returns stopping', () async {
      final snapshot = await _buildAndSnapshot(
        runtimeState: RuntimeBridgeState.stopping,
        runtimeGeneration: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
        connection: BackendConnectionAvailable(_makeConfig(5)),
        transportGeneration: 5,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      );

      expect(snapshot.phase, RuntimeStatusPhase.stopping);
      expect(snapshot.businessAvailable, false);
    });

    test('Case 12: Runtime stopped with stale transport', () async {
      final snapshot = await _buildAndSnapshot(
        runtimeState: RuntimeBridgeState.stopped,
        runtimeGeneration: 5,
        runtimeInstalled: true,
        runtimeAvailable: false,
        connection: BackendConnectionAvailable(_makeConfig(5)),
        transportGeneration: 4,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      );

      expect(snapshot.phase, RuntimeStatusPhase.initializing);
      expect(snapshot.businessAvailable, false);
      expect(snapshot.runtimeReady, false);
    });

    test('Case 15: WS only failure does not make Runtime failed', () async {
      final snapshot = await _buildAndSnapshot(
        runtimeState: RuntimeBridgeState.ready,
        runtimeGeneration: 5,
        runtimeInstalled: true,
        runtimeAvailable: true,
        connection: BackendConnectionAvailable(_makeConfig(5)),
        transportGeneration: 5,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.disconnected,
      );

      expect(snapshot.phase, RuntimeStatusPhase.degraded);
      expect(snapshot.runtimeReady, true);
      expect(snapshot.primaryError?.source,
          RuntimeStatusErrorSource.webSocket);
    });
  });
}
