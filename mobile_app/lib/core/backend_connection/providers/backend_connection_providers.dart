import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend_connection_availability.dart';
import '../backend_connection_config.dart';
import '../backend_connection_credential.dart';
import '../backend_connection_endpoint.dart';
import '../backend_connection_error.dart';
import '../backend_connection_repository.dart';
import '../backend_connection_source.dart';
import 'runtime_backend_connection_source.dart';
import '../../auth/account_session_store.dart';
import '../../runtime/runtime_bridge_provider.dart';
import '../../runtime/runtime_bridge_state.dart';
import '../../runtime/backend/mobile_backend_providers.dart';
import '../../runtime/backend/mobile_deployment_mode.dart';
import '../../runtime/backend/backend_topology_resolver.dart';

final accountSessionProvider = Provider<AccountSessionStore>((ref) {
  return FlutterSecureAccountSessionStore();
});

final backendConnectionRepositoryProvider = Provider<BackendConnectionRepository>((ref) {
  final source = ref.watch(backendConnectionSourceProvider);
  return DefaultBackendConnectionRepository(source);
});

final backendConnectionSourceProvider = Provider<BackendConnectionSource>((ref) {
  final config = ref.watch(mobileDeploymentConfigProvider);
  if (config.mode == MobileDeploymentMode.local) {
    return const RuntimeBackendConnectionSource();
  }
  final sessionStore = ref.watch(accountSessionProvider);
  return _CloudBackendConnectionSource(config, sessionStore);
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
      return const BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.RUNTIME_NOT_READY,
          'embedded runtime is not ready',
        ),
      );
    }
    return repo.resolve(expectedRuntimeGeneration: runtime.generation);
  }

  return repo.resolve(expectedRuntimeGeneration: null);
});

/// Repository-level generation semantics:
/// - local mode: Native Runtime generation is the single source of truth and
///   must never be rewritten by Flutter;
/// - cloud mode: the repository owns a monotonic business-generation counter
///   because there is no embedded runtime generation to bind against.
class DefaultBackendConnectionRepository implements BackendConnectionRepository {
  final BackendConnectionSource _source;
  BackendConnectionConfig? _cached;
  int _resolutionEpoch = 0;
  int _cloudBusinessGeneration = 0;
  String? _lastCloudCacheKey;

  DefaultBackendConnectionRepository(this._source);

  @override
  Future<BackendConnectionAvailability> resolve({
    int? expectedRuntimeGeneration,
  }) async {
    final epoch = ++_resolutionEpoch;
    final result = await _source.resolve(
      expectedRuntimeGeneration: expectedRuntimeGeneration,
    );

    if (epoch != _resolutionEpoch) {
      return const BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.GENERATION_INVALID,
          'backend connection resolution was superseded by a newer runtime state',
        ),
      );
    }

    if (result is! BackendConnectionAvailable) {
      _cached = null;
      return result;
    }

    if (expectedRuntimeGeneration != null) {
      if (expectedRuntimeGeneration <= 0 ||
          result.config.generation != expectedRuntimeGeneration) {
        _cached = null;
        return BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.GENERATION_INVALID,
            'backend connection generation ${result.config.generation} does not match runtime generation $expectedRuntimeGeneration',
          ),
        );
      }
      // Local mode must preserve the Native Runtime generation exactly.
      _cached = result.config;
      return BackendConnectionAvailable(result.config);
    }

    final cacheKey = _cacheKeyFor(result.config);
    if (_cloudBusinessGeneration <= 0 || cacheKey != _lastCloudCacheKey) {
      _cloudBusinessGeneration++;
      _lastCloudCacheKey = cacheKey;
    }
    final stamped = BackendConnectionConfig(
      schemaVersion: result.config.schemaVersion,
      generation: _cloudBusinessGeneration,
      endpoint: result.config.endpoint,
      authStrategy: result.config.authStrategy,
      credential: result.config.credential,
    );
    _cached = stamped;
    return BackendConnectionAvailable(stamped);
  }

  String _cacheKeyFor(BackendConnectionConfig config) {
    return [
      config.endpoint.host,
      config.endpoint.port,
      config.endpoint.httpScheme,
      config.endpoint.webSocketScheme,
      config.endpoint.livenessPath,
      config.endpoint.readinessPath,
      config.authStrategy.name,
      config.credential.revealForTransport(),
    ].join('|');
  }

  @override
  BackendConnectionConfig? get cached => _cached;

  @override
  void invalidate() {
    // Invalidation only revokes the current resolution/cache authority. It must
    // never mutate a generation, otherwise STARTING-state rebuilds can drift
    // away from the Native Runtime generation before READY is observed.
    _resolutionEpoch++;
    _cached = null;
  }
}

class _CloudBackendConnectionSource implements BackendConnectionSource {
  final MobileDeploymentConfig _config;
  final AccountSessionStore _sessionStore;

  _CloudBackendConnectionSource(this._config, this._sessionStore);

  @override
  Future<BackendConnectionAvailability> resolve({int? expectedRuntimeGeneration}) async {
    final uri = _config.remoteCoreUri;
    if (uri == null || uri.trim().isEmpty) {
      return const BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.ENDPOINT_UNAVAILABLE,
          'remote core URI is not configured',
        ),
      );
    }
    try {
      final parsed = normalizeRemoteCoreUri(uri);
      final scheme = parsed.scheme.toLowerCase();
      final host = parsed.host;
      if (host.isEmpty) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.ENDPOINT_INVALID,
            'remote core URI does not contain a host',
          ),
        );
      }
      var port = parsed.port;
      if (port == 0) port = scheme == 'https' ? 443 : 80;
      final wsScheme = scheme == 'https' ? 'wss' : 'ws';
      final endpoint = BackendConnectionEndpoint(
        host: host,
        port: port,
        httpScheme: scheme,
        webSocketScheme: wsScheme,
        livenessPath: '/readyz',
        readinessPath: '/readyz',
      );
      final token = await _sessionStore.getAccessToken();
      if (token == null || token.trim().isEmpty) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE,
            'remote core access token is unavailable',
          ),
        );
      }
      final credential = BackendConnectionCredential.tryCreate(token);
      if (credential == null) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_INVALID,
            'remote core access token is invalid',
          ),
        );
      }
      final config = BackendConnectionConfig(
        schemaVersion: 1,
        generation: 1,
        endpoint: endpoint,
        authStrategy: BackendAuthStrategy.bearer,
        credential: credential,
      );
      return BackendConnectionAvailable(config);
    } catch (error) {
      return BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.ENDPOINT_INVALID,
          'failed to parse remote core URI: $error',
        ),
      );
    }
  }
}
