import 'package:dio/dio.dart';

import 'backend_http_method.dart';

final class BackendHttpRequest {
  final BackendHttpMethod method;
  final String path;
  final Map<String, dynamic>? queryParameters;
  final Map<String, String>? headers;
  final Object? body;
  final Duration? timeout;
  final bool streamResponse;
  final CancelToken? cancelToken;

  BackendHttpRequest({
    required this.method,
    required this.path,
    this.queryParameters,
    this.headers,
    this.body,
    this.timeout,
    this.streamResponse = false,
    this.cancelToken,
  });
}
