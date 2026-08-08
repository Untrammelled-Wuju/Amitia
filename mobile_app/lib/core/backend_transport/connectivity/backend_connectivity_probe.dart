import '../http/backend_http_method.dart';
import '../http/backend_http_request.dart';
import '../http/backend_http_transport.dart';

enum BackendConnectivityResult {
  unreachable,
  live,
  ready,
}

class BackendConnectivityProbe {
  final BackendHttpTransport _http;

  BackendConnectivityProbe(this._http);

  Future<BackendConnectivityResult> probe() async {
    try {
      final readyResp = await _http.send(BackendHttpRequest(
        method: BackendHttpMethod.get,
        path: '/readyz',
      ));
      if (readyResp.statusCode == 200) {
        return BackendConnectivityResult.ready;
      }
    } catch (_) {
      // fall through
    }

    try {
      final liveResp = await _http.send(BackendHttpRequest(
        method: BackendHttpMethod.get,
        path: '/livez',
      ));
      if (liveResp.statusCode == 200) {
        return BackendConnectivityResult.live;
      }
    } catch (_) {
      // fall through
    }

    return BackendConnectivityResult.unreachable;
  }
}
