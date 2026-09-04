import '../backend_connection/backend_connection_config.dart';
import 'backend_transport.dart';
import 'http/backend_http_client.dart';
import 'http/backend_http_transport.dart';
import 'websocket/backend_websocket_client.dart';
import 'websocket/backend_websocket_transport.dart';

class DefaultBackendTransport implements BackendTransport {
  @override
  final int generation;
  @override
  final BackendHttpTransport http;
  @override
  final BackendWebSocketTransport webSocket;

  DefaultBackendTransport({
    required this.generation,
    required this.http,
    required this.webSocket,
  });

  factory DefaultBackendTransport.create(BackendConnectionConfig config) {
    return DefaultBackendTransport(
      generation: config.generation,
      http: BackendHttpClient(config),
      webSocket: BackendWebSocketClient(config),
    );
  }

  @override
  Future<void> close() async {
    await http.close();
    await webSocket.close();
  }
}
