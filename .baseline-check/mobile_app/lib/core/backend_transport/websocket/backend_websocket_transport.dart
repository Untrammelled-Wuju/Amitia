import '../state/backend_websocket_state.dart';
import 'backend_websocket_session.dart';

abstract interface class BackendWebSocketTransport {
  Future<BackendWebSocketSession> connect(
    String path, {
    Map<String, dynamic>? queryParameters,
  });

  BackendWebSocketState get state;

  Future<void> close();
}
