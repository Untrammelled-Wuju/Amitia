import '../state/backend_http_state.dart';
import 'backend_http_client.dart';
import 'backend_http_request.dart';
import 'backend_http_response.dart';
import 'backend_http_transport.dart';

class BackendHttpTransportImpl implements BackendHttpTransport {
  final BackendHttpClient _client;

  BackendHttpTransportImpl(this._client);

  @override
  BackendHttpState get state => _client.state;

  @override
  Future<BackendHttpResponse> send(
    BackendHttpRequest request,
  ) {
    return _client.send(request);
  }

  @override
  Future<void> close() {
    return _client.close();
  }
}
