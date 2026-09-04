import 'backend_transport.dart';

final class BackendTransportSession {
  final int generation;
  final BackendTransport transport;

  BackendTransportSession({
    required this.generation,
    required this.transport,
  });
}
