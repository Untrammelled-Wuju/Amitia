import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../backend_connection_availability.dart';
import '../backend_connection_config.dart';
import '../backend_connection_endpoint.dart';
import '../backend_connection_repository.dart';
import '../backend_connection_source.dart';
import 'runtime_backend_connection_source.dart';
import '../../runtime/runtime_bridge_provider.dart';
import '../../runtime/runtime_bridge_state.dart';
import '../../runtime/backend/mobile_backend_providers.dart';
import '../../runtime/backend/mobile_deployment_mode.dart';

final backendConnectionRepositoryProvider = Provider<BackendConnectionRepository>((ref) {
  final source = ref.watch(backendConnectionSourceProvider);
  return _DefaultBackendConnectionRepository(source);
});

final backendConnectionSourceProvider = Provider<BackendConnectionSource>((ref) {
  final config = ref.watch(mobileDeploymentConfigProvider);
  if (config.mode == MobileDeploymentMode.local) {
    return const RuntimeBackendConnectionSource();
  }
  return _CloudBackendConnectionSource(config);
});

final backendConnectionProvider = FutureProvider<BackendConnectionAvailability>((ref) async {
  final config = ref.watch(mobileDeploymentConfigProvider);
  final repo = ref.watch(backendConnectionRepositoryProvider);

  if (config.mode == MobileDeploymentMode.local) {
    final runtimeAsync = ref.watch(runtimeSnapshotProvider);
    final runtime = runtimeAsync.valueOrNull;
    if (runtime == null ||
        runtime.state != RuntimeBridgeState.ready ||
        runtime.generation <= 0) {
      repo.invalidate();
      return BackendConnectionUnavailable();
    }
    return repo.resolve(expectedGeneration: runtime.generation);
  }

  return repo.resolve(expectedGeneration: 0);
});

class _DefaultBackendConnectionRepository implements BackendConnectionRepository {
  final BackendConnectionSource _source;
  BackendConnectionConfig? _cached;
  int _resolutionEpoch = 0;

  _DefaultBackendConnectionRepository(this._source);

  @override
  Future<BackendConnectionAvailability> resolve({
    required int expectedGeneration,
  }) async {
    if (expectedGeneration <= 0) {
      invalidate();
      return BackendConnectionUnavailable();
    }

    final epoch = ++_resolutionEpoch;

    final result = await _source.resolve(
      expectedGeneration: expectedGeneration,
    );

    if (epoch != _resolutionEpoch) {
      return BackendConnectionUnavailable();
    }

    if (result is BackendConnectionAvailable &&
        result.config.generation == expectedGeneration) {
      _cached = result.config;
      return result;
    }

    _cached = null;
    return BackendConnectionUnavailable();
  }

  @override
  BackendConnectionConfig? get cached => _cached;

  @override
  void invalidate() {
    _resolutionEpoch++;
    _cached = null;
  }
}

class _CloudBackendConnectionSource implements BackendConnectionSource {
  final MobileDeploymentConfig _config;

  _CloudBackendConnectionSource(this._config);

  @override
  Future<BackendConnectionAvailability> resolve({int expectedGeneration = 0}) async {
    final uri = _config.remoteCoreUri;
    if (uri == null || uri.trim().isEmpty) {
      return BackendConnectionUnavailable();
    }
    try {
      final parsed = Uri.parse(uri.trim());
      final scheme = parsed.scheme.toLowerCase();
      if (scheme != 'http' && scheme != 'https') {
        return BackendConnectionUnavailable();
      }
      final host = parsed.host;
      if (host.isEmpty) return BackendConnectionUnavailable();
      var port = parsed.port;
      if (port == 0) port = scheme == 'https' ? 443 : 80;
      final wsScheme = scheme == 'https' ? 'wss' : 'ws';
      final endpoint = BackendConnectionEndpoint(
        host: host,
        port: port,
        httpScheme: scheme,
        webSocketScheme: wsScheme,
        livenessPath: '/livez',
        readinessPath: '/readyz',
      );
      final credential = BackendConnectionCredential.tryCreate('');
      if (credential == null) return BackendConnectionUnavailable();
      final config = BackendConnectionConfig(
        schemaVersion: 1,
        generation: 1,
        endpoint: endpoint,
        credential: credential,
      );
      return BackendConnectionAvailable(config);
    } catch (_) {
      return BackendConnectionUnavailable();
    }
  }
}
