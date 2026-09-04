import '../errors/backend_transport_error.dart';

sealed class BackendTransportState {
  const BackendTransportState();
}

class TransportIdle extends BackendTransportState {
  const TransportIdle();
}

class TransportAvailable extends BackendTransportState {
  final int generation;

  const TransportAvailable({
    required this.generation,
  }) : assert(generation > 0);
}

class TransportUnavailable extends BackendTransportState {
  final BackendTransportError? error;
  const TransportUnavailable({this.error});
}

class TransportClosed extends BackendTransportState {
  const TransportClosed();
}
