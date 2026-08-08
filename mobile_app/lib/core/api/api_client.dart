import 'package:dio/dio.dart';

import '../backend_connection/backend_connection_config.dart';
import '../backend_connection/backend_uri_builder.dart';
import 'api_response.dart';
import 'api_exception.dart';

class ApiClient {
  static final ApiClient _instance = ApiClient._internal();
  factory ApiClient() => _instance;

  BackendConnectionConfig? _config;
  final BackendUriBuilder _uriBuilder = BackendUriBuilder();
  Dio? _dio;

  ApiClient._internal();

  void updateConfig(BackendConnectionConfig config) {
    _config = config;
    _dio = _createDio(config);
  }

  Dio _createDio(BackendConnectionConfig config) {
    final baseUri = _uriBuilder.httpBase(config);
    return Dio(BaseOptions(
      baseUrl: baseUri.toString(),
      connectTimeout: const Duration(seconds: 5),
      receiveTimeout: const Duration(seconds: 30),
    ));
  }

  Future<ApiResponse<T>> request<T>(
    String path, {
    String method = 'GET',
    Map<String, dynamic>? data,
    Map<String, dynamic>? queryParameters,
    T Function(dynamic)? fromJson,
  }) async {
    final config = _config;
    if (config == null || _dio == null) {
      throw ApiException.network('Backend transport not available');
    }

    final uri = _uriBuilder.http(config, path, queryParameters: queryParameters);

    try {
      final response = await _dio!.requestUri(
        uri,
        data: data,
        options: Options(
          method: method,
          headers: {
            'X-Amitia-Local-Token': config.credential.revealForTransport(),
            'Accept': 'application/json',
            if (data != null) 'Content-Type': 'application/json',
            'User-Agent': 'Amitia-Mobile',
          },
          validateStatus: (status) => true,
        ),
      );

      final body = response.data;
      if (body is Map<String, dynamic> && body.containsKey('code')) {
        final apiResponse = ApiResponse<T>.fromJson(body, fromJson);
        if (!apiResponse.isSuccess) {
          throw ApiException.fromResponse(
            apiResponse.code,
            apiResponse.message,
            detail: apiResponse.detail,
            raw: body,
          );
        }
        return apiResponse;
      }

      return ApiResponse<T>(
        code: 200,
        message: 'success',
        data: body as T?,
      );
    } on DioException catch (e) {
      if (e.type == DioExceptionType.connectionTimeout ||
          e.type == DioExceptionType.sendTimeout ||
          e.type == DioExceptionType.receiveTimeout) {
        throw ApiException.timeout();
      }
      if (e.type == DioExceptionType.connectionError) {
        throw ApiException.network('核心服务连接中...');
      }
      final body = e.response?.data;
      if (body is Map<String, dynamic>) {
        throw ApiException.fromResponse(
          body['code'] as int? ?? e.response?.statusCode ?? 0,
          body['message'] as String? ?? body['msg'] as String? ?? e.message ?? '网络错误',
          detail: body['detail'] as String?,
          raw: body,
        );
      }
      throw ApiException.network(e.message ?? '网络错误');
    }
  }

  Future<ApiResponse<T>> get<T>(String path,
      {Map<String, dynamic>? queryParameters, T Function(dynamic)? fromJson}) {
    return request<T>(path,
        method: 'GET', queryParameters: queryParameters, fromJson: fromJson);
  }

  Future<ApiResponse<T>> post<T>(String path,
      {Map<String, dynamic>? data, T Function(dynamic)? fromJson}) {
    return request<T>(path, method: 'POST', data: data, fromJson: fromJson);
  }

  Future<ApiResponse<T>> put<T>(String path,
      {Map<String, dynamic>? data, T Function(dynamic)? fromJson}) {
    return request<T>(path, method: 'PUT', data: data, fromJson: fromJson);
  }

  Future<ApiResponse<T>> delete<T>(String path, {T Function(dynamic)? fromJson}) {
    return request<T>(path, method: 'DELETE', fromJson: fromJson);
  }
}
