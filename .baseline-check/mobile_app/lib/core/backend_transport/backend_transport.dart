import 'http/backend_http_transport.dart';
import 'websocket/backend_websocket_transport.dart';

abstract interface class BackendTransport {
  BackendHttpTransport get http;

  BackendWebSocketTransport get webSocket;

  int get generation;

  Future<void> close();
}
