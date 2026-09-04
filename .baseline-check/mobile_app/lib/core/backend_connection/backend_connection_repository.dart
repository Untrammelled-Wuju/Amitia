import 'backend_connection_availability.dart';
import 'backend_connection_config.dart';

abstract interface class BackendConnectionRepository {
  Future<BackendConnectionAvailability> resolve({int? expectedRuntimeGeneration});
  BackendConnectionConfig? get cached;
  void invalidate();
}
