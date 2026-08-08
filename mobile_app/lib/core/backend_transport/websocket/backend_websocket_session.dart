import '../state/backend_websocket_state.dart';
import 'backend_websocket_message.dart';

abstract interface class BackendWebSocketSession {
  int get generation;

  BackendWebSocketState get state;

  Stream<BackendWebSocketMessage> get messages;

  Future<void> send(
    BackendWebSocketMessage message,
  );

  Future<void> close();
}
