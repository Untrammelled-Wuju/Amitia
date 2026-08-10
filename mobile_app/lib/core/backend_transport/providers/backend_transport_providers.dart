import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:logger/logger.dart';

import '../../backend_connection/backend_connection_availability.dart';
import '../../backend_connection/backend_connection_config.dart';
import '../../backend_connection/providers/backend_connection_providers.dart';
import '../backend_service_api.dart';
import '../backend_transport.dart';
import '../default_backend_transport.dart';
import '../http/backend_http_method.dart';
import '../http/backend_http_request.dart';
import '../http/backend_http_response.dart';
import '../http/backend_http_transport.dart';
import '../state/backend_transport_state.dart';

final _transportLogger = Logger();

final backendTransportProvider = AsyncNotifierProvider<
    BackendTransportNotifier, BackendTransportState>(
  BackendTransportNotifier.new,
);

class BackendTransportNotifier
    extends AsyncNotifier<BackendTransportState> {
  BackendTransport? _current;
  int _currentGeneration = 0;

  @override
  Future<BackendTransportState> build() async {
    final connectionAsync = ref.watch(backendConnectionProvider);

    return connectionAsync.when(
      data: (connection) {
        if (connection is! BackendConnectionAvailable) {
          _closeCurrentIfNeeded();
          return const TransportUnavailable();
        }
        final config = connection.config;
        if (config.generation != _currentGeneration || _current == null) {
          _recreateTransport(config);
        }
        return const TransportAvailable();
      },
      loading: () => const TransportIdle(),
      error: (error, stack) {
        _closeCurrentIfNeeded();
        return const TransportUnavailable();
      },
    );
  }

  void _recreateTransport(BackendConnectionConfig config) {
    if (_currentGeneration == config.generation && _current != null) return;
    _closeCurrentIfNeeded();
    _currentGeneration = config.generation;
    _current = DefaultBackendTransport.create(config);
    _transportLogger.d(
      'BackendTransport created: generation=${config.generation}',
    );
  }

  void _closeCurrentIfNeeded() {
    if (_current != null) {
      _transportLogger.d(
        'BackendTransport closing: generation=$_currentGeneration',
      );
      _current!.close();
      _current = null;
      _currentGeneration = 0;
    }
  }

  BackendTransport? get currentTransport => _current;

  int get currentGeneration => _currentGeneration;

  void disposeTransport() {
    _closeCurrentIfNeeded();
  }
}

final backendCurrentTransportProvider =
    Provider<BackendTransport?>((ref) {
  final notifier = ref.watch(backendTransportProvider.notifier);
  return notifier.currentTransport;
});

final backendTransportGenerationProvider = Provider<int>((ref) {
  final notifier = ref.watch(backendTransportProvider.notifier);
  return notifier.currentGeneration;
});

final backendTransportApiProvider = Provider<BackendTransportApi?>((ref) {
  final transport = ref.watch(backendCurrentTransportProvider);
  if (transport == null) return null;
  return BackendTransportApi(transport.http, transport.generation);
});

final backendServiceProvider = Provider<BackendServiceApi?>((ref) {
  final transport = ref.watch(backendCurrentTransportProvider);
  if (transport == null) return null;
  return BackendServiceApi(transport.http, transport.generation);
});

final streamTimeoutProvider = Provider<Duration>((ref) {
  return const Duration(seconds: 120);
});

final connectTimeoutProvider = Provider<Duration>((ref) {
  return const Duration(seconds: 5);
});

class BackendTransportApi {
  final BackendHttpTransport _http;
  final int _generation;

  BackendTransportApi(this._http, this._generation);

  int get generation => _generation;

  Future<BackendHttpResponse> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    Duration? timeout,
  }) {
    return _http.send(BackendHttpRequest(
      method: BackendHttpMethod.get,
      path: path,
      queryParameters: queryParameters,
      timeout: timeout,
    ));
  }

  Future<BackendHttpResponse> post(
    String path, {
    Object? body,
    Map<String, dynamic>? queryParameters,
    Duration? timeout,
  }) {
    return _http.send(BackendHttpRequest(
      method: BackendHttpMethod.post,
      path: path,
      body: body,
      queryParameters: queryParameters,
      timeout: timeout,
    ));
  }

  Future<BackendHttpResponse> put(
    String path, {
    Object? body,
    Map<String, dynamic>? queryParameters,
    Duration? timeout,
  }) {
    return _http.send(BackendHttpRequest(
      method: BackendHttpMethod.put,
      path: path,
      body: body,
      queryParameters: queryParameters,
      timeout: timeout,
    ));
  }

  Future<BackendHttpResponse> head(
    String path, {
    Duration? timeout,
  }) {
    return _http.send(BackendHttpRequest(
      method: BackendHttpMethod.head,
      path: path,
      timeout: timeout,
    ));
  }

  Future<BackendHttpResponse> delete(
    String path, {
    Object? body,
    Duration? timeout,
  }) {
    return _http.send(BackendHttpRequest(
      method: BackendHttpMethod.delete,
      path: path,
      body: body,
      timeout: timeout,
    ));
  }
}
