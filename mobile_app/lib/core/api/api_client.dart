import 'package:dio/dio.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'api_response.dart';
import 'api_exception.dart';
import 'package:logger/logger.dart';

class ApiClient {
  static final ApiClient _instance = ApiClient._internal();
  factory ApiClient() => _instance;

  late final Dio _dio;
  final Logger _logger = Logger();
  static const String _tokenKey = 'ai_companion_token';
  static const String _baseUrlKey = 'api_base_url';
  static const String _defaultBaseUrl = 'http://127.0.0.1:18899';

  static const Set<String> _publicPaths = {
    '/api/public/auth/status',
    '/api/public/auth/setup',
    '/api/public/auth/login',
  };

  ApiClient._internal() {
    _dio = Dio(BaseOptions(
      baseUrl: _defaultBaseUrl,
      connectTimeout: const Duration(seconds: 30),
      receiveTimeout: const Duration(seconds: 30),
      headers: {'Content-Type': 'application/json'},
    ));
    _setupInterceptors();
  }

  void _setupInterceptors() {
    _dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) async {
        final prefs = await SharedPreferences.getInstance();
        final savedBaseUrl = prefs.getString(_baseUrlKey);
        if (savedBaseUrl != null && savedBaseUrl.isNotEmpty) {
          options.baseUrl = savedBaseUrl;
        }

        final path = options.path;
        if (!_publicPaths.contains(path)) {
          final token = prefs.getString(_tokenKey);
          if (token != null && token.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $token';
          }
        }

        _logger.d('REQUEST[${options.method}] => PATH: ${options.uri}');
        handler.next(options);
      },
      onResponse: (response, handler) {
        _logger.d('RESPONSE[${response.statusCode}] => PATH: ${response.requestOptions.path}');
        handler.next(response);
      },
      onError: (error, handler) {
        _logger.e('ERROR[${error.response?.statusCode}] => PATH: ${error.requestOptions.path}');
        handler.next(error);
      },
    ));
  }

  Future<void> setBaseUrl(String url) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_baseUrlKey, url);
    _dio.options.baseUrl = url;
  }

  String get baseUrl => _dio.options.baseUrl;

  Future<ApiResponse<T>> request<T>(
    String path, {
    String method = 'GET',
    Map<String, dynamic>? data,
    Map<String, dynamic>? queryParameters,
    T Function(dynamic)? fromJson,
  }) async {
    try {
      final response = await _dio.request(
        path,
        data: data,
        queryParameters: queryParameters,
        options: Options(method: method),
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

  Future<ApiResponse<T>> get<T>(String path, {Map<String, dynamic>? queryParameters, T Function(dynamic)? fromJson}) {
    return request<T>(path, method: 'GET', queryParameters: queryParameters, fromJson: fromJson);
  }

  Future<ApiResponse<T>> post<T>(String path, {Map<String, dynamic>? data, T Function(dynamic)? fromJson}) {
    return request<T>(path, method: 'POST', data: data, fromJson: fromJson);
  }

  Future<ApiResponse<T>> put<T>(String path, {Map<String, dynamic>? data, T Function(dynamic)? fromJson}) {
    return request<T>(path, method: 'PUT', data: data, fromJson: fromJson);
  }

  Future<ApiResponse<T>> delete<T>(String path, {T Function(dynamic)? fromJson}) {
    return request<T>(path, method: 'DELETE', fromJson: fromJson);
  }
}
