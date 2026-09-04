import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/errors/backend_transport_error.dart';
import 'package:amitia_app/core/backend_transport/errors/backend_transport_error_code.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_method.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_request.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_response.dart';
import 'package:amitia_app/core/backend_transport/routed_backend_service_api.dart';

class _RecordingApi implements BackendServiceApi {
  final List<_RoutedCall> calls = [];
  final int generationValue;

  _RecordingApi({this.generationValue = 1});

  @override
  int get generation => generationValue;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(_RoutedCall('GET', path, headers, null));
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
    calls.add(_RoutedCall('POST', path, headers, data));
    return null;
  }

  @override
  Future<T?> postPayload<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(_RoutedCall('POST', path, headers, data));
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
    calls.add(_RoutedCall('PUT', path, headers, data));
    return null;
  }

  @override
  Future<T?> patch<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(_RoutedCall('PATCH', path, headers, data));
    return null;
  }

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {
    calls.add(_RoutedCall('DELETE', path, headers, null));
  }

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(_RoutedCall('DELETE', path, headers, data));
    return null;
  }
}

class _RoutedCall {
  final String method;
  final String path;
  final Map<String, String>? headers;
  final Object? body;
  _RoutedCall(this.method, this.path, this.headers, this.body);
}

void main() {
  group('RoutedBackendServiceApiProxy routing', () {
    late _RecordingApi businessApi;
    late _RecordingApi deviceLocalApi;
    late RoutedBackendServiceApiProxy proxy;

    setUp(() {
      businessApi = _RecordingApi();
      deviceLocalApi = _RecordingApi();
      proxy = RoutedBackendServiceApiProxy(
        businessApi: businessApi,
        deviceLocalApi: deviceLocalApi,
      );
    });

    test('regular cloud API does not route to device agent', () async {
      await proxy.get('/api/characters');
      expect(businessApi.calls.length, 1);
      expect(deviceLocalApi.calls.length, 0);
      expect(businessApi.calls.first.path, '/api/characters');
    });

    test('desktop pet device API routes to device agent', () async {
      await proxy.get('/api/desktop-pets/installations');
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.first.path, '/api/desktop-pets/installations');
    });

    test('desktop pet singular prefix routes to device agent', () async {
      await proxy.get('/api/desktop-pet/status');
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
    });

    test('device mesh internal API routes to device agent', () async {
      await proxy.get('/internal/device-mesh/status');
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.first.path, '/internal/device-mesh/status');
    });

    test('similar prefix does not mis-route', () async {
      await proxy.get('/api/desktop-petfoo');
      expect(businessApi.calls.length, 1);
      expect(deviceLocalApi.calls.length, 0);
    });

    test('exact prefix match without slash stays on business', () async {
      await proxy.get('/api/desktop-pets');
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
    });

    test('path with query string classifies correctly', () async {
      await proxy.get('/api/desktop-pets/installations', queryParameters: {'active': 'true'});
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
    });
  });

  group('RoutedBackendServiceApiProxy headers', () {
    late _RecordingApi businessApi;
    late _RecordingApi deviceLocalApi;
    late RoutedBackendServiceApiProxy proxy;

    setUp(() {
      businessApi = _RecordingApi();
      deviceLocalApi = _RecordingApi();
      proxy = RoutedBackendServiceApiProxy(
        businessApi: businessApi,
        deviceLocalApi: deviceLocalApi,
      );
    });

    test('GET to device local sets X-Amitia-Client-Type mobile', () async {
      await proxy.get('/api/desktop-pets/installations');
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers, isNotNull);
      expect(headers!['X-Amitia-Client-Type'], 'mobile');
    });

    test('cloud GET does not set X-Amitia-Client-Type', () async {
      await proxy.get('/api/characters');
      final headers = businessApi.calls.first.headers;
      expect(headers?['X-Amitia-Client-Type'], isNull);
    });

    test('POST to device local auto-generates Idempotency-Key', () async {
      await proxy.post('/api/desktop-pets/installations', data: {'action': 'enable'});
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers, isNotNull);
      expect(headers!['Idempotency-Key'], isNotNull);
      expect(headers['Idempotency-Key']!, startsWith('mobile-'));
    });

    test('PUT to device local auto-generates Idempotency-Key', () async {
      await proxy.put('/api/desktop-pets/installations/123', data: {'enabled': true});
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers, isNotNull);
      expect(headers!['Idempotency-Key'], isNotNull);
    });

    test('PATCH to device local auto-generates Idempotency-Key', () async {
      await proxy.patch('/api/desktop-pets/installations/123', data: {'alpha': 0.5});
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers, isNotNull);
      expect(headers!['Idempotency-Key'], isNotNull);
    });

    test('DELETE to device local auto-generates Idempotency-Key', () async {
      await proxy.delete('/api/desktop-pets/installations/123');
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers, isNotNull);
      expect(headers!['Idempotency-Key'], isNotNull);
    });

    test('GET to device local does not generate Idempotency-Key', () async {
      await proxy.get('/api/desktop-pets/installations');
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers?['Idempotency-Key'], isNull);
    });

    test('caller-supplied Idempotency-Key is not overwritten', () async {
      await proxy.post(
        '/api/desktop-pets/installations',
        data: {'action': 'enable'},
        headers: {'Idempotency-Key': 'caller-supplied-key-123'},
      );
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers!['Idempotency-Key'], 'caller-supplied-key-123');
    });

    test('cloud POST does not get Idempotency-Key', () async {
      await proxy.post('/api/characters', data: {'name': 'test'});
      final headers = businessApi.calls.first.headers;
      expect(headers?['Idempotency-Key'], isNull);
    });

    test('Idempotency-Key is unique per mutation call', () async {
      await proxy.post('/api/desktop-pets/a', data: {});
      await proxy.post('/api/desktop-pets/b', data: {});
      final key1 = deviceLocalApi.calls[0].headers!['Idempotency-Key']!;
      final key2 = deviceLocalApi.calls[1].headers!['Idempotency-Key']!;
      expect(key1, isNot(equals(key2)));
    });
  });
}
