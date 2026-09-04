import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:logger/logger.dart';

import '../../backend_connection/backend_connection_availability.dart';
import '../../backend_connection/backend_connection_config.dart';
import '../../backend_connection/backend_connection_error.dart';
import '../../backend_connection/providers/backend_connection_providers.dart';
import '../../backend_connection/providers/runtime_backend_connection_source.dart';
import '../../runtime/status/runtime_status_provider.dart';
import '../../runtime/runtime_bridge_provider.dart';
import '../../runtime/runtime_bridge_state.dart';
import '../../runtime/backend/mobile_backend_providers.dart';
import '../../runtime/backend/mobile_deployment_mode.dart';
import '../../debug/debug_log_service.dart';
import '../backend_service_api.dart';
import '../backend_transport.dart';
import '../dynamic_backend_service_api.dart';
import '../routed_backend_service_api.dart';
import '../default_backend_transport.dart';
import '../http/backend_http_method.dart';
import '../http/backend_http_request.dart';
import '../http/backend_http_response.dart';
import '../http/backend_http_transport.dart';
import '../state/backend_transport_state.dart';

final _transportLogger = Logger();

final backendTransportProvider =
    AsyncNotifierProvider<BackendTransportNotifier, BackendTransportState>(
      BackendTransportNotifier.new,
    );

class BackendTransportNotifier extends AsyncNotifier<BackendTransportState> {
  BackendTransport? _current;
  int _currentGeneration = 0;
  bool _disposeRegistered = false;

  @override
  Future<BackendTransportState> build() async {
    _ensureDisposeRegistered();

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
        return TransportAvailable(generation: config.generation);
      },
      loading: () {
        _closeCurrentIfNeeded();
        return const TransportIdle();
      },
      error: (error, stack) {
        _closeCurrentIfNeeded();
        return const TransportUnavailable();
      },
    );
  }

  void _ensureDisposeRegistered() {
    if (_disposeRegistered) return;
    _disposeRegistered = true;
    ref.onDispose(_closeCurrentIfNeeded);
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

final backendCurrentTransportProvider = Provider<BackendTransport?>((ref) {
  final transportAsync = ref.watch(backendTransportProvider);

  final state = transportAsync.asData?.value;

  if (state is! TransportAvailable) {
    return null;
  }

  final notifier = ref.read(backendTransportProvider.notifier);

  final transport = notifier.currentTransport;

  if (transport == null || notifier.currentGeneration != state.generation) {
    return null;
  }

  return transport;
});

final backendTransportGenerationProvider = Provider<int>((ref) {
  final transportAsync = ref.watch(backendTransportProvider);

  final state = transportAsync.asData?.value;

  if (state is TransportAvailable) {
    return state.generation;
  }

  return 0;
});

final backendTransportApiProvider = Provider<BackendTransportApi?>((ref) {
  final transport = ref.watch(backendCurrentTransportProvider);
  if (transport == null) return null;
  return BackendTransportApi(transport.http, transport.generation);
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
    return _http.send(
      BackendHttpRequest(
        method: BackendHttpMethod.get,
        path: path,
        queryParameters: queryParameters,
        timeout: timeout,
      ),
    );
  }

  Future<BackendHttpResponse> post(
    String path, {
    Object? body,
    Map<String, dynamic>? queryParameters,
    Duration? timeout,
  }) {
    return _http.send(
      BackendHttpRequest(
        method: BackendHttpMethod.post,
        path: path,
        body: body,
        queryParameters: queryParameters,
        timeout: timeout,
      ),
    );
  }

  Future<BackendHttpResponse> put(
    String path, {
    Object? body,
    Map<String, dynamic>? queryParameters,
    Duration? timeout,
  }) {
    return _http.send(
      BackendHttpRequest(
        method: BackendHttpMethod.put,
        path: path,
        body: body,
        queryParameters: queryParameters,
        timeout: timeout,
      ),
    );
  }

  Future<BackendHttpResponse> head(String path, {Duration? timeout}) {
    return _http.send(
      BackendHttpRequest(
        method: BackendHttpMethod.head,
        path: path,
        timeout: timeout,
      ),
    );
  }

  Future<BackendHttpResponse> delete(
    String path, {
    Object? body,
    Duration? timeout,
  }) {
    return _http.send(
      BackendHttpRequest(
        method: BackendHttpMethod.delete,
        path: path,
        body: body,
        timeout: timeout,
      ),
    );
  }
}


final deviceLocalBackendConnectionProvider =
    FutureProvider<BackendConnectionAvailability>((ref) async {
      final runtimeAsync = ref.watch(runtimeSnapshotProvider);
      final runtime = runtimeAsync.valueOrNull;
      if (runtime == null ||
          runtime.state != RuntimeBridgeState.ready ||
          runtime.generation <= 0) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.RUNTIME_NOT_READY,
            'embedded device runtime is not ready',
          ),
        );
      }
      return const RuntimeBackendConnectionSource().resolve(
        expectedRuntimeGeneration: runtime.generation,
      );
    });

final deviceLocalBackendTransportProvider = AsyncNotifierProvider<
  DeviceLocalBackendTransportNotifier,
  BackendTransportState
>(DeviceLocalBackendTransportNotifier.new);

class DeviceLocalBackendTransportNotifier
    extends AsyncNotifier<BackendTransportState> {
  BackendTransport? _current;
  int _currentGeneration = 0;
  bool _disposeRegistered = false;

  @override
  Future<BackendTransportState> build() async {
    if (!_disposeRegistered) {
      _disposeRegistered = true;
      ref.onDispose(_closeCurrentIfNeeded);
    }

    final connectionAsync = ref.watch(deviceLocalBackendConnectionProvider);
    return connectionAsync.when(
      data: (connection) {
        if (connection is! BackendConnectionAvailable) {
          _closeCurrentIfNeeded();
          return const TransportUnavailable();
        }
        final config = connection.config;
        if (_current == null || _currentGeneration != config.generation) {
          _closeCurrentIfNeeded();
          _currentGeneration = config.generation;
          _current = DefaultBackendTransport.create(config);
          _transportLogger.d(
            'DeviceLocalBackendTransport created: generation=${config.generation}',
          );
        }
        return TransportAvailable(generation: config.generation);
      },
      loading: () {
        _closeCurrentIfNeeded();
        return const TransportIdle();
      },
      error: (_, __) {
        _closeCurrentIfNeeded();
        return const TransportUnavailable();
      },
    );
  }

  BackendTransport? get currentTransport => _current;

  int get currentGeneration => _currentGeneration;

  void _closeCurrentIfNeeded() {
    final current = _current;
    _current = null;
    _currentGeneration = 0;
    if (current != null) {
      current.close();
    }
  }
}

final deviceLocalBackendCurrentTransportProvider = Provider<BackendTransport?>((ref) {
  final state = ref.watch(deviceLocalBackendTransportProvider).asData?.value;
  if (state is! TransportAvailable) return null;
  final notifier = ref.read(deviceLocalBackendTransportProvider.notifier);
  final transport = notifier.currentTransport;
  if (transport == null || notifier.currentGeneration != state.generation) {
    return null;
  }
  return transport;
});

final rawDeviceLocalBackendServiceApiProvider = Provider<BackendServiceApi?>((ref) {
  final transport = ref.watch(deviceLocalBackendCurrentTransportProvider);
  if (transport == null) return null;
  return BackendServiceApi(transport.http, transport.generation);
});

final rawBackendServiceApiProvider = Provider<BackendServiceApi?>((ref) {
  final transport = ref.watch(backendCurrentTransportProvider);
  if (transport == null) return null;
  return BackendServiceApi(transport.http, transport.generation);
});

final backendServiceProvider = Provider<BackendServiceApi>((ref) {
  final logService = ref.read(debugLogServiceProvider);
  DateTime? lastUnavailableAt;
  String? lastUnavailableKey;

  final businessApi = DynamicBackendServiceApiProxy(
    currentApi: () => ref.read(rawBackendServiceApiProvider),
    currentStatus: () => ref.read(runtimeStatusCurrentProvider),
    canUseApi: (status, api) {
      final mode = ref.read(mobileDeploymentConfigProvider).mode;
      if (mode != MobileDeploymentMode.local) {
        return api != null;
      }
      return status.businessAvailable &&
          api != null &&
          api.generation == status.generation;
    },
    onUnavailable: (error) {
      final now = DateTime.now();
      final key = '${error.phase.name}:${error.generation}:${error.primaryError?.code ?? 'BUSINESS_UNAVAILABLE'}';
      if (lastUnavailableKey == key &&
          lastUnavailableAt != null &&
          now.difference(lastUnavailableAt!) < const Duration(seconds: 5)) {
        return;
      }
      lastUnavailableKey = key;
      lastUnavailableAt = now;
      logService.addBackendLog(
        'Business backend unavailable: $error',
        DebugLogLevel.error,
      );
    },
  );

  final deviceLocalApi = DynamicBackendServiceApiProxy(
    currentApi: () => ref.read(rawDeviceLocalBackendServiceApiProvider),
    currentStatus: () => ref.read(runtimeStatusCurrentProvider),
    canUseApi: (_, api) => api != null,
    onUnavailable: (error) {
      logService.addBackendLog(
        'Device-local backend unavailable: $error',
        DebugLogLevel.error,
      );
    },
  );

  return RoutedBackendServiceApiProxy(
    businessApi: businessApi,
    deviceLocalApi: deviceLocalApi,
  );
});
