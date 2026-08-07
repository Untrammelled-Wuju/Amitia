import 'backend_connection_config.dart';

sealed class BackendConnectionAvailability {}

class BackendConnectionUnavailable extends BackendConnectionAvailability {}

class BackendConnectionAvailable extends BackendConnectionAvailability {
  final BackendConnectionConfig config;
  BackendConnectionAvailable(this.config);
}

class BackendConnectionResolving extends BackendConnectionAvailability {}
