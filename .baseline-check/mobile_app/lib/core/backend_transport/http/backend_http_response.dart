final class BackendHttpResponse {
  final int statusCode;
  final Map<String, String> headers;
  final Object? data;

  BackendHttpResponse({
    required this.statusCode,
    required this.headers,
    this.data,
  });
}
