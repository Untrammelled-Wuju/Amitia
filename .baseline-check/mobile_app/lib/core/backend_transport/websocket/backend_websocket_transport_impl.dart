import '../state/backend_websocket_state.dart';
import 'backend_websocket_client.dart';
import 'backend_websocket_session.dart';
import 'backend_websocket_transport.dart';

class BackendWebSocketTransportImpl implements BackendWebSocketTransport {
  final BackendWebSocketClient _client;

  BackendWebSocketTransportImpl(this._client);

  @override
  BackendWebSocketState get state => _client.state;

  @override
  Future<BackendWebSocketSession> connect(
    String path, {
    Map<String, dynamic>? queryParameters,
  }) {
    return _client.connect(path, queryParameters: queryParameters);
  }

  @override
  Future<void> close() {
    return _client.close();
  }
}
