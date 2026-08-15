import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../backend_connection_availability.dart';
import '../backend_connection_config.dart';
import '../backend_connection_endpoint.dart';
import '../backend_connection_repository.dart';
import '../backend_connection_source.dart';
import 'runtime_backend_connection_source.dart';
import '../../auth/auth_token_store.dart';
import '../../runtime/runtime_bridge_provider.dart';
import '../../runtime/runtime_bridge_state.dart';
import '../../runtime/backend/mobile_backend_providers.dart';
import '../../runtime/backend/mobile_deployment_mode.dart';

final authTokenStoreProvider = Provider<AuthTokenStore>((ref) {
  return const SharedPreferencesAuthTokenStore();
});

final backendConnectionRepositoryProvider = Provider<BackendConnectionRepository>((ref) {
  final source = ref.watch(backendConnectionSourceProvider);
  return _DefaultBackendConnectionRepository(source);
});

final backendConnectionSourceProvider = Provider<BackendConnectionSource>((ref) {
  final config = ref.watch(mobileDeploymentConfigProvider);
  if (config.mode == MobileDeploymentMode.local) {
    return const RuntimeBackendConnectionSource();
  }
  final tokenStore = ref.watch(authTokenStoreProvider);
  return _CloudBackendConnectionSource(config, tokenStore);
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
    return repo.resolve(expectedRuntimeGeneration: runtime.generation);
  }

  return repo.resolve(expectedRuntimeGeneration: null);
});

class _DefaultBackendConnectionRepository implements BackendConnectionRepository {
  final BackendConnectionSource _source;
  BackendConnectionConfig? _cached;
  int _resolutionEpoch = 0;
  int _businessGeneration = 0;
  String? _lastCacheKey;

  _DefaultBackendConnectionRepository(this._source);

  @override
  Future<BackendConnectionAvailability> resolve({
    int? expectedRuntimeGeneration,
  }) async {
    final epoch = ++_resolutionEpoch;

    final result = await _source.resolve(
      expectedRuntimeGeneration: expectedRuntimeGeneration,
    );

    if (epoch != _resolutionEpoch) {
      return BackendConnectionUnavailable();
    }

    if (result is BackendConnectionAvailable) {
      if (expectedRuntimeGeneration != null &&
          result.config.generation != expectedRuntimeGeneration) {
        _cached = null;
        return BackendConnectionUnavailable();
      }
      final cacheKey = _cacheKeyFor(result.config);
      if (cacheKey != _lastCacheKey) {
        _businessGeneration++;
        _lastCacheKey = cacheKey;
      }
      final stamped = BackendConnectionConfig(
        schemaVersion: result.config.schemaVersion,
        generation: _businessGeneration,
        endpoint: result.config.endpoint,
        authStrategy: result.config.authStrategy,
        credential: result.config.credential,
      );
      _cached = stamped;
      return BackendConnectionAvailable(stamped);
    }

    _cached = null;
    return BackendConnectionUnavailable();
  }

  String _cacheKeyFor(BackendConnectionConfig config) {
    return [
      config.endpoint.host,
      config.endpoint.port,
      config.endpoint.httpScheme,
      config.authStrategy.name,
      config.credential.revealForTransport(),
    ].join('|');
  }

  @override
  BackendConnectionConfig? get cached => _cached;

  @override
  void invalidate() {
    _resolutionEpoch++;
    _businessGeneration++;
    _cached = null;
  }
}

class _CloudBackendConnectionSource implements BackendConnectionSource {
  final MobileDeploymentConfig _config;
  final AuthTokenStore _tokenStore;

  _CloudBackendConnectionSource(this._config, this._tokenStore);

  @override
  Future<BackendConnectionAvailability> resolve({int? expectedRuntimeGeneration}) async {
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
      final token = await _tokenStore.getToken();
      if (token == null || token.trim().isEmpty) {
        return BackendConnectionUnavailable();
      }
      final credential = BackendConnectionCredential.tryCreate(token);
      if (credential == null) return BackendConnectionUnavailable();
      final config = BackendConnectionConfig(
        schemaVersion: 1,
        generation: 1,
        endpoint: endpoint,
        authStrategy: BackendAuthStrategy.bearer,
        credential: credential,
      );
      return BackendConnectionAvailable(config);
    } catch (_) {
      return BackendConnectionUnavailable();
    }
  }
}
