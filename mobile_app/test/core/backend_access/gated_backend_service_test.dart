import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_access/business_backend_unavailable.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_phase.dart';

void main() {
  group('GatedBackendServiceApi Request Count', () {
    test('blocks all requests when gate throws', () {
      final gated = _AlwaysBlockedGatedApi();

      expect(() => gated.get('/api/test'),
          throwsA(isA<BusinessBackendUnavailable>()));
      expect(() => gated.post('/api/test', data: {'x': 1}),
          throwsA(isA<BusinessBackendUnavailable>()));
      expect(() => gated.put('/api/test', data: {'x': 1}),
          throwsA(isA<BusinessBackendUnavailable>()));
      expect(() => gated.delete('/api/test'),
          throwsA(isA<BusinessBackendUnavailable>()));
      expect(() => gated.deleteWithResponse('/api/test'),
          throwsA(isA<BusinessBackendUnavailable>()));

      expect(gated.sendCount, 0);
    });

    test('allows requests when gate passes', () async {
      final gated = _PassingGatedApi();

      await gated.get('/api/test');
      await gated.post('/api/test', data: {'x': 1});
      await gated.put('/api/test', data: {'x': 1});
      await gated.delete('/api/test');
      await gated.deleteWithResponse('/api/test');

      expect(gated.sendCount, 5);
    });

    test('gate false = zero network requests', () async {
      final gated = _AlwaysBlockedGatedApi();

      var requestCount = 0;
      try {
        await gated.get('/api/test');
      } on BusinessBackendUnavailable {
        requestCount++;
      }
      try {
        await gated.post('/api/test', data: {'a': 1});
      } on BusinessBackendUnavailable {
        requestCount++;
      }
      try {
        await gated.put('/api/test', data: {'a': 1});
      } on BusinessBackendUnavailable {
        requestCount++;
      }
      try {
        await gated.delete('/api/test');
      } on BusinessBackendUnavailable {
        requestCount++;
      }

      expect(requestCount, 4);
      expect(gated.sendCount, 0);
    });
  });
}

class _AlwaysBlockedGatedApi implements BackendServiceApi {
  int sendCount = 0;

  @override
  int get generation => 5;

  void _check() {
    throw const BusinessBackendUnavailable(
      phase: RuntimeStatusPhase.degraded,
      generation: 5,
      primaryError: null,
    );
  }

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {
    _check();
    sendCount++;
  }

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<Stream<List<int>>> getStream(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    CancelToken? cancelToken,
  }) async => const Stream<List<int>>.empty();

  @override
  Future<Stream<List<int>>> postStream(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    CancelToken? cancelToken,
  }) async => const Stream<List<int>>.empty();

  @override
  Future<T?> postMultipart<T>(
    String path, {
    Map<String, String> fields = const {},
    Map<String, List<String>> files = const {},
    Map<String, dynamic>? queryParameters,
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
  Future<T?> patch<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;
}

class _PassingGatedApi implements BackendServiceApi {
  int sendCount = 0;

  @override
  int get generation => 5;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    sendCount++;
    return null;
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    sendCount++;
    return null;
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    sendCount++;
    return null;
  }

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {
    sendCount++;
  }

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    sendCount++;
    return null;
  }

  @override
  Future<Stream<List<int>>> getStream(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    CancelToken? cancelToken,
  }) async => const Stream<List<int>>.empty();

  @override
  Future<Stream<List<int>>> postStream(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    CancelToken? cancelToken,
  }) async => const Stream<List<int>>.empty();

  @override
  Future<T?> postMultipart<T>(
    String path, {
    Map<String, String> fields = const {},
    Map<String, List<String>> files = const {},
    Map<String, dynamic>? queryParameters,
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
  Future<T?> patch<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;
}
