import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../backend_connection_availability.dart';
import '../backend_connection_config.dart';
import '../backend_connection_repository.dart';
import '../backend_connection_source.dart';
import 'runtime_backend_connection_source.dart';
import '../../runtime/runtime_bridge_provider.dart';
import '../../runtime/runtime_bridge_state.dart';

final backendConnectionRepositoryProvider = Provider<BackendConnectionRepository>((ref) {
  final source = ref.watch(backendConnectionSourceProvider);
  return _DefaultBackendConnectionRepository(source);
});

final backendConnectionSourceProvider = Provider<BackendConnectionSource>((ref) {
  return const RuntimeBackendConnectionSource();
});

final backendConnectionProvider = FutureProvider<BackendConnectionAvailability>((ref) async {
  final runtimeAsync = ref.watch(runtimeSnapshotProvider);
  final repo = ref.watch(backendConnectionRepositoryProvider);

  final runtime = runtimeAsync.valueOrNull;

  if (runtime == null ||
      runtime.state != RuntimeBridgeState.ready ||
      runtime.generation <= 0) {
    repo.invalidate();
    return BackendConnectionUnavailable();
  }

  return repo.resolve(
    expectedGeneration: runtime.generation,
  );
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
