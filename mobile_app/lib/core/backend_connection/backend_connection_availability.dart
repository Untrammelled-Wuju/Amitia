import 'backend_connection_config.dart';
import 'backend_connection_error.dart';

sealed class BackendConnectionAvailability {
  const BackendConnectionAvailability();
}

class BackendConnectionUnavailable extends BackendConnectionAvailability {
  final BackendConnectionError? error;

  const BackendConnectionUnavailable([this.error]);
}

class BackendConnectionAvailable extends BackendConnectionAvailability {
  final BackendConnectionConfig config;

  const BackendConnectionAvailable(this.config);
}

class BackendConnectionResolving extends BackendConnectionAvailability {
  const BackendConnectionResolving();
}
