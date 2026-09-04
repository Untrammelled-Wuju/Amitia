import 'backend_connection_availability.dart';
import 'backend_connection_config.dart';

abstract interface class BackendConnectionSource {
  Future<BackendConnectionAvailability> resolve({int? expectedRuntimeGeneration});
}
