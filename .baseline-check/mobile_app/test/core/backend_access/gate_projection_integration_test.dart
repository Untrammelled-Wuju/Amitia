import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_access/business_backend_access.dart';
import 'package:amitia_app/core/backend_access/business_backend_unavailable.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/status/default_runtime_status_projection.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_phase.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_projection.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_snapshot.dart';

import '../runtime/status/fakes/fake_runtime_bridge.dart';
import '../runtime/status/fakes/fake_backend_connection_source.dart';
import '../runtime/status/fakes/fake_transport_state_source.dart';

void main() {
  group('Gate + Projection Integration', () {
    test('businessAvailable flows from projection to gate', () async {
      final projection = _FakeProjection();
      projection.setBusinessAvailable(true);
      projection.setGeneration(5);
      final gate = BusinessBackendAccess(projection);

      expect(gate.businessAvailable, true);
      expect(gate.businessGeneration, 5);

      projection.setBusinessAvailable(false);
      projection.setPhase(RuntimeStatusPhase.degraded);

      expect(gate.businessAvailable, false);
      expect(() => gate.requireBusinessAvailable(),
          throwsA(isA<BusinessBackendUnavailable>()));

      await projection.dispose();
    });

    test('gate generation tracks projection generation', () async {
      final projection = _FakeProjection();
      final gate = BusinessBackendAccess(projection);

      expect(gate.businessGeneration, 1);

      projection.setGeneration(10);
      expect(gate.businessGeneration, 10);

      projection.setGeneration(20);
      expect(gate.businessGeneration, 20);

      await projection.dispose();
    });

    test('acquireApi rejects stale generation', () async {
      final projection = _FakeProjection();
      projection.setBusinessAvailable(true);
      projection.setGeneration(5);
      final gate = BusinessBackendAccess(projection);

      final api = _FakeApi(5);
      final result = gate.acquireApi(api);
      expect(result.generation, 5);

      projection.setGeneration(10);
      expect(() => gate.acquireApi(api),
          throwsA(isA<BusinessBackendUnavailable>()));

      await projection.dispose();
    });

    test('full projection to gate cycle', () async {
      final bridge = FakeRuntimeBridge();
      final connectionSource = FakeBackendConnectionSource();
      final transportSource = FakeTransportStateSource();
      final projection = DefaultRuntimeStatusProjection(
        bridge: bridge,
        connectionSource: connectionSource,
        transportStateSource: transportSource,
      );

      await projection.initialize();

      const bridgeSnapshot = RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 1,
        runtimeInstalled: true,
        runtimeAvailable: true,
      );
      bridge.setSnapshot(bridgeSnapshot);
      connectionSource.setAvailability(
        BackendConnectionAvailable(_makeConfig(1)),
      );
      transportSource.emit(const TransportStateSnapshot(
        generation: 1,
        httpState: BackendHttpState.available,
        webSocketState: BackendWebSocketState.connected,
      ));

      await Future.delayed(const Duration(milliseconds: 20));

      final gate = BusinessBackendAccess(projection);

      expect(projection.current.phase, RuntimeStatusPhase.ready);
      expect(gate.businessAvailable, true);

      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 2,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));

      await Future.delayed(const Duration(milliseconds: 20));

      connectionSource.setAvailability(
        BackendConnectionUnavailable(),
      );

      bridge.setSnapshot(const RuntimeBridgeSnapshot(
        schemaVersion: 1,
        state: RuntimeBridgeState.ready,
        generation: 3,
        runtimeInstalled: true,
        runtimeAvailable: true,
      ));

      await Future.delayed(const Duration(milliseconds: 20));

      expect(gate.businessAvailable, false);
      expect(() => gate.requireBusinessAvailable(),
          throwsA(isA<BusinessBackendUnavailable>()));

      await projection.dispose();
      await transportSource.close();
    });
  });
}

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
    credential: BackendConnectionCredential.tryCreate('a' * 32) ??
        (throw StateError('Failed to create credential')),
  );
}

class _FakeProjection implements RuntimeStatusProjection {
  RuntimeStatusSnapshot _snapshot = const RuntimeStatusSnapshot(
    generation: 1,
    phase: RuntimeStatusPhase.ready,
    httpAvailable: true,
    webSocketConnected: true,
    primaryError: null,
    businessAvailable: true,
    runtimeState: RuntimeBridgeState.ready,
    runtimeReady: true,
    runtimeInstalled: true,
    backendConfigured: true,
  );

  @override
  Stream<RuntimeStatusSnapshot> get snapshots => const Stream.empty();

  @override
  RuntimeStatusSnapshot get current => _snapshot;

  RuntimeStatusSnapshot _copyWith({
    int? generation,
    RuntimeStatusPhase? phase,
    bool? businessAvailable,
  }) {
    return RuntimeStatusSnapshot(
      generation: generation ?? _snapshot.generation,
      phase: phase ?? _snapshot.phase,
      httpAvailable: _snapshot.httpAvailable,
      webSocketConnected: _snapshot.webSocketConnected,
      primaryError: _snapshot.primaryError,
      businessAvailable: businessAvailable ?? _snapshot.businessAvailable,
      runtimeState: _snapshot.runtimeState,
      runtimeReady: _snapshot.runtimeReady,
      runtimeInstalled: _snapshot.runtimeInstalled,
      backendConfigured: _snapshot.backendConfigured,
    );
  }

  void setBusinessAvailable(bool available) {
    _snapshot = _copyWith(businessAvailable: available);
  }

  void setPhase(RuntimeStatusPhase phase) {
    _snapshot = _copyWith(phase: phase);
  }

  void setGeneration(int generation) {
    _snapshot = _copyWith(generation: generation);
  }

  @override
  Future<void> dispose() async {}
}

class _FakeApi implements BackendServiceApi {
  @override
  final int generation;

  _FakeApi(this.generation);

  @override
  Future<T?> get<T>(String path,
          {Map<String, dynamic>? queryParameters,
          T Function(dynamic)? fromJson}) async =>
      null;

  @override
  Future<T?> post<T>(String path,
          {Object? data, T Function(dynamic)? fromJson}) async =>
      null;

  @override
  Future<T?> put<T>(String path,
          {Object? data, T Function(dynamic)? fromJson}) async =>
      null;

  @override
  Future<void> delete(String path) async {}

  @override
  Future<T?> deleteWithResponse<T>(String path,
          {Object? data, T Function(dynamic)? fromJson}) async =>
      null;
}
