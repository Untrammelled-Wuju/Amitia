sealed class BackendWebSocketMessage {
  const BackendWebSocketMessage();
}

class WebSocketTextMessage extends BackendWebSocketMessage {
  final String data;
  const WebSocketTextMessage(this.data);
}

class WebSocketBinaryMessage extends BackendWebSocketMessage {
  final List<int> data;
  const WebSocketBinaryMessage(this.data);
}

class WebSocketCloseMessage extends BackendWebSocketMessage {
  final int? code;
  final String? reason;
  const WebSocketCloseMessage({this.code, this.reason});
}

class WebSocketErrorMessage extends BackendWebSocketMessage {
  final Object error;
  const WebSocketErrorMessage(this.error);
}
