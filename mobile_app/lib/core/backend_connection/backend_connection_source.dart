import 'backend_connection_availability.dart';
import 'backend_connection_error.dart';

abstract interface class BackendConnectionSource {
  Future<BackendConnectionAvailability> resolve();
}

class BackendConnectionSourceException {
  final BackendConnectionErrorCode code;
  final String message;
  BackendConnectionSourceException(this.code, this.message);
}
