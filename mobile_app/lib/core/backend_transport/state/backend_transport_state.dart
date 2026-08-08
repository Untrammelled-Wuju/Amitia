import '../errors/backend_transport_error.dart';

sealed class BackendTransportState {
  const BackendTransportState();
}

class TransportIdle extends BackendTransportState {
  const TransportIdle();
}

class TransportAvailable extends BackendTransportState {
  const TransportAvailable();
}

class TransportUnavailable extends BackendTransportState {
  final BackendTransportError? error;
  const TransportUnavailable({this.error});
}

class TransportClosed extends BackendTransportState {
  const TransportClosed();
}
