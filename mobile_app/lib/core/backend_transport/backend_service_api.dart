import 'errors/backend_transport_error.dart';
import 'errors/backend_transport_error_code.dart';
import 'http/backend_http_method.dart';
import 'http/backend_http_request.dart';
import 'http/backend_http_response.dart';
import 'http/backend_http_transport.dart';

class ServiceApiException implements Exception {
  final int code;
  final String message;
  final String? detail;

  ServiceApiException({
    required this.code,
    required this.message,
    this.detail,
  });

  factory ServiceApiException.fromTransportError(BackendTransportError e) {
    return ServiceApiException(
      code: _mapErrorCodeToApiCode(e.code),
      message: _mapErrorCodeToMessage(e.code),
      detail: e.path,
    );
  }

  factory ServiceApiException.network(String message) {
    return ServiceApiException(code: 10001, message: message);
  }

  factory ServiceApiException.timeout() {
    return ServiceApiException(code: 10002, message: '请求超时');
  }

  static int _mapErrorCodeToApiCode(BackendTransportErrorCode code) {
    switch (code) {
      case BackendTransportErrorCode.authenticationFailed:
        return 401;
      case BackendTransportErrorCode.requestTimeout:
        return 10002;
      case BackendTransportErrorCode.connectionRefused:
      case BackendTransportErrorCode.connectionReset:
      case BackendTransportErrorCode.networkUnreachable:
        return 10001;
      case BackendTransportErrorCode.badRequest:
        return 400;
      case BackendTransportErrorCode.notFound:
        return 404;
      case BackendTransportErrorCode.conflict:
        return 409;
      case BackendTransportErrorCode.validationFailed:
        return 422;
      case BackendTransportErrorCode.serverError:
        return 500;
      case BackendTransportErrorCode.serviceUnavailable:
        return 503;
      default:
        return 10000;
    }
  }

  static String _mapErrorCodeToMessage(BackendTransportErrorCode code) {
    switch (code) {
      case BackendTransportErrorCode.authenticationFailed:
        return '认证失败';
      case BackendTransportErrorCode.requestTimeout:
        return '请求超时';
      case BackendTransportErrorCode.connectionRefused:
      case BackendTransportErrorCode.connectionReset:
        return '核心服务连接中...';
      case BackendTransportErrorCode.networkUnreachable:
        return '网络不可达';
      case BackendTransportErrorCode.badRequest:
        return '请求错误';
      case BackendTransportErrorCode.notFound:
        return '资源不存在';
      case BackendTransportErrorCode.conflict:
        return '资源冲突';
      case BackendTransportErrorCode.validationFailed:
        return '数据验证失败';
      case BackendTransportErrorCode.serverError:
        return '服务器内部错误';
      case BackendTransportErrorCode.serviceUnavailable:
        return '服务暂不可用';
      case BackendTransportErrorCode.transportClosed:
        return '传输已关闭';
      case BackendTransportErrorCode.requestCancelled:
        return '请求已取消';
      default:
        return '网络错误';
    }
  }
}

class BackendServiceApi {
  final BackendHttpTransport _http;
  final int _generation;

  BackendServiceApi(this._http, this._generation);

  int get generation => _generation;

  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final response = await _http.send(BackendHttpRequest(
      method: BackendHttpMethod.get,
      path: path,
      queryParameters: queryParameters,
      headers: headers,
    ));
    return _parseResponse<T>(response, fromJson, path);
  }

  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final response = await _http.send(BackendHttpRequest(
      method: BackendHttpMethod.post,
      path: path,
      queryParameters: queryParameters,
      headers: headers,
      body: data,
    ));
    return _parseResponse<T>(response, fromJson, path);
  }

  /// Invoke endpoints that intentionally use the RPC envelope
  /// `{code,msg,payload}` instead of the normal management `{code,msg,data}` envelope.
  Future<T?> postPayload<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final response = await _http.send(BackendHttpRequest(
      method: BackendHttpMethod.post,
      path: path,
      headers: headers,
      body: data,
    ));
    return _parsePayloadResponse<T>(response, fromJson, path);
  }

  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final response = await _http.send(BackendHttpRequest(
      method: BackendHttpMethod.put,
      path: path,
      queryParameters: queryParameters,
      headers: headers,
      body: data,
    ));
    return _parseResponse<T>(response, fromJson, path);
  }

  Future<T?> patch<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final response = await _http.send(BackendHttpRequest(
      method: BackendHttpMethod.patch,
      path: path,
      queryParameters: queryParameters,
      headers: headers,
      body: data,
    ));
    return _parseResponse<T>(response, fromJson, path);
  }

  Future<void> delete(String path, {Map<String, dynamic>? queryParameters, Map<String, String>? headers}) async {
    final response = await _http.send(BackendHttpRequest(
      method: BackendHttpMethod.delete,
      path: path,
      queryParameters: queryParameters,
      headers: headers,
    ));
    _parseSimpleResponse(response, path);
  }

  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final response = await _http.send(BackendHttpRequest(
      method: BackendHttpMethod.delete,
      path: path,
      headers: headers,
      body: data,
    ));
    return _parseResponse<T>(response, fromJson, path);
  }

  T? _parseResponse<T>(
    BackendHttpResponse response,
    T Function(dynamic)? fromJson,
    String path,
  ) {
    final data = response.data;
    if (data is Map<String, dynamic> && data.containsKey('code')) {
      final code = data['code'] as int? ?? 0;
      final message = data['message'] as String? ?? data['msg'] as String? ?? '';
      final detail = data['detail'] as String?;
      if (code != 200) {
        throw ServiceApiException(code: code, message: message, detail: detail);
      }
      final responseData = data['data'];
      if (responseData == null) {
        return null;
      }
      if (fromJson != null) {
        return fromJson(responseData);
      }
      return responseData as T?;
    }
    if (fromJson != null && data != null) {
      return fromJson(data);
    }
    return data as T?;
  }

  T? _parsePayloadResponse<T>(
    BackendHttpResponse response,
    T Function(dynamic)? fromJson,
    String path,
  ) {
    final data = response.data;
    if (data is Map<String, dynamic> && data.containsKey('code')) {
      final code = data['code'] as int? ?? 0;
      final message = data['message'] as String? ?? data['msg'] as String? ?? '';
      final detail = data['detail'] as String?;
      if (code != 200) {
        throw ServiceApiException(code: code, message: message, detail: detail);
      }
      final payload = data['payload'];
      if (payload == null) return null;
      if (fromJson != null) return fromJson(payload);
      return payload as T?;
    }
    if (fromJson != null && data != null) return fromJson(data);
    return data as T?;
  }

  void _parseSimpleResponse(BackendHttpResponse response, String path) {
    final data = response.data;
    if (data is Map<String, dynamic> && data.containsKey('code')) {
      final code = data['code'] as int? ?? 0;
      final message = data['message'] as String? ?? data['msg'] as String? ?? '';
      if (code != 200) {
        throw ServiceApiException(code: code, message: message);
      }
    }
  }
}
