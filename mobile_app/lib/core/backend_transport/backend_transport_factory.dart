import '../backend_connection/backend_connection_config.dart';
import 'backend_transport.dart';

abstract interface class BackendTransportFactory {
  BackendTransport create(
    BackendConnectionConfig config,
  );
}
