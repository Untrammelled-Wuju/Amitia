import 'backend_connection_availability.dart';
import 'backend_connection_config.dart';

abstract interface class BackendConnectionRepository {
  Future<BackendConnectionAvailability> resolve({
    required int expectedGeneration,
  });
  BackendConnectionConfig? get cached;
  void invalidate();
}
