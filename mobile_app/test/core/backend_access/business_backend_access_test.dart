import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_access/business_backend_access.dart';
import 'package:amitia_app/core/backend_access/business_backend_unavailable.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_phase.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_snapshot.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_error.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_projection.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';

class _FakeProjection implements RuntimeStatusProjection {
  RuntimeStatusSnapshot _current;

  _FakeProjection(this._current);

  void update(RuntimeStatusSnapshot snapshot) {
    _current = snapshot;
  }

  @override
  RuntimeStatusSnapshot get current => _current;

  @override
  Stream<RuntimeStatusSnapshot> get snapshots => const Stream.empty();

  @override
  Future<void> dispose() async {}
}

void main() {
  group('BusinessBackendAccess', () {
    RuntimeStatusSnapshot _makeSnapshot({
      RuntimeStatusPhase phase = RuntimeStatusPhase.ready,
      bool businessAvailable = true,
      int generation = 5,
      RuntimeStatusError? primaryError,
    }) {
      return RuntimeStatusSnapshot(
        phase: phase,
        runtimeState: phase == RuntimeStatusPhase.ready
            ? RuntimeBridgeState.ready
            : RuntimeBridgeState.stopped,
        runtimeReady: phase == RuntimeStatusPhase.ready,
        runtimeInstalled: true,
        backendConfigured: true,
        httpAvailable: true,
        webSocketConnected: true,
        businessAvailable: businessAvailable,
        generation: generation,
        primaryError: primaryError,
      );
    }

    test('businessAvailable=true allows access', () {
      final projection = _FakeProjection(
        _makeSnapshot(businessAvailable: true, generation: 5),
      );
      final gate = BusinessBackendAccess(projection);

      expect(gate.businessAvailable, true);
      expect(gate.businessGeneration, 5);
      expect(() => gate.requireBusinessAvailable(), returnsNormally);
    });

    test('businessAvailable=false throws typed error', () {
      final projection = _FakeProjection(
        _makeSnapshot(
          businessAvailable: false,
          generation: 5,
          phase: RuntimeStatusPhase.degraded,
          primaryError: const RuntimeStatusError(
            source: RuntimeStatusErrorSource.http,
            code: 'HTTP_UNAVAILABLE',
            message: 'HTTP transport unavailable',
          ),
        ),
      );
      final gate = BusinessBackendAccess(projection);

      expect(gate.businessAvailable, false);
      expect(
        () => gate.requireBusinessAvailable(),
        throwsA(isA<BusinessBackendUnavailable>()),
      );

      try {
        gate.requireBusinessAvailable();
        fail('should have thrown');
      } on BusinessBackendUnavailable catch (e) {
        expect(e.phase, RuntimeStatusPhase.degraded);
        expect(e.generation, 5);
        expect(e.primaryError?.code, 'HTTP_UNAVAILABLE');
      }
    });

    test('acquireApi returns api when available and generation matches', () {
      final projection = _FakeProjection(
        _makeSnapshot(businessAvailable: true, generation: 5),
      );
      final gate = BusinessBackendAccess(projection);
      final api = _FakeBackendServiceApi(5);

      final result = gate.acquireApi(api);
      expect(result, same(api));
    });

    test('acquireApi throws when business unavailable', () {
      final projection = _FakeProjection(
        _makeSnapshot(businessAvailable: false, generation: 5),
      );
      final gate = BusinessBackendAccess(projection);
      final api = _FakeBackendServiceApi(5);

      expect(
        () => gate.acquireApi(api),
        throwsA(isA<BusinessBackendUnavailable>()),
      );
    });

    test('acquireApi throws when rawApi is null', () {
      final projection = _FakeProjection(
        _makeSnapshot(businessAvailable: true, generation: 5),
      );
      final gate = BusinessBackendAccess(projection);

      expect(
        () => gate.acquireApi(null),
        throwsA(isA<BusinessBackendUnavailable>()),
      );
    });

    test('acquireApi throws when generation mismatch', () {
      final projection = _FakeProjection(
        _makeSnapshot(businessAvailable: true, generation: 8),
      );
      final gate = BusinessBackendAccess(projection);
      final api = _FakeBackendServiceApi(7);

      expect(
        () => gate.acquireApi(api),
        throwsA(isA<BusinessBackendUnavailable>()),
      );
    });

    test('snapshot reflects current projection state', () {
      final projection = _FakeProjection(
        _makeSnapshot(
          businessAvailable: false,
          generation: 0,
          phase: RuntimeStatusPhase.unavailable,
        ),
      );
      final gate = BusinessBackendAccess(projection);

      expect(gate.snapshot.phase, RuntimeStatusPhase.unavailable);
      expect(gate.snapshot.generation, 0);

      projection.update(_makeSnapshot(businessAvailable: true, generation: 10));

      expect(gate.snapshot.phase, RuntimeStatusPhase.ready);
      expect(gate.snapshot.generation, 10);
      expect(gate.businessAvailable, true);
      expect(gate.businessGeneration, 10);
    });
  });
}

class _FakeBackendServiceApi implements BackendServiceApi {
  @override
  final int _generation;

  _FakeBackendServiceApi(this._generation);

  @override
  int get generation => _generation;

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {}

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;

  @override
  Future<T?> postPayload<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;

  @override
  Future<T?> patch<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;
}
