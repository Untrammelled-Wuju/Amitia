import '../state/backend_http_state.dart';
import 'backend_http_request.dart';
import 'backend_http_response.dart';

abstract interface class BackendHttpTransport {
  Future<BackendHttpResponse> send(
    BackendHttpRequest request,
  );

  BackendHttpState get state;

  Future<void> close();
}
