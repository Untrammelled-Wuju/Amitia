import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../backend_connection_availability.dart';
import '../backend_connection_config.dart';
import '../backend_connection_repository.dart';
import '../backend_connection_source.dart';
import 'runtime_backend_connection_source.dart';

final backendConnectionRepositoryProvider = Provider<BackendConnectionRepository>((ref) {
  final source = ref.watch(backendConnectionSourceProvider);
  return _DefaultBackendConnectionRepository(source);
});

final backendConnectionSourceProvider = Provider<BackendConnectionSource>((ref) {
  return const RuntimeBackendConnectionSource();
});

final backendConnectionProvider = FutureProvider<BackendConnectionAvailability>((ref) async {
  final repo = ref.watch(backendConnectionRepositoryProvider);
  return repo.resolve();
});

class _DefaultBackendConnectionRepository implements BackendConnectionRepository {
  final BackendConnectionSource _source;
  BackendConnectionConfig? _cached;

  _DefaultBackendConnectionRepository(this._source);

  @override
  Future<BackendConnectionAvailability> resolve() async {
    final result = await _source.resolve();
    if (result is BackendConnectionAvailable) {
      _cached = result.config;
    } else {
      _cached = null;
    }
    return result;
  }

  @override
  BackendConnectionConfig? get cached => _cached;

  @override
  void invalidate() {
    _cached = null;
  }
}
