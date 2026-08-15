import 'dart:async';

import 'package:dio/dio.dart';

import '../../backend_connection/backend_connection_config.dart';
import '../../backend_connection/backend_uri_builder.dart';
import '../auth/backend_auth_header.dart';
import '../errors/backend_transport_error.dart';
import '../errors/backend_transport_error_code.dart';
import '../state/backend_http_state.dart';
import 'backend_http_request.dart';
import 'backend_http_response.dart';
import 'backend_http_transport.dart';

class BackendHttpClient implements BackendHttpTransport {
  final BackendConnectionConfig _config;
  final BackendUriBuilder _uriBuilder;
  final Dio _dio;
  BackendHttpState _state = BackendHttpState.idle;
  bool _closed = false;

  BackendHttpClient(
    this._config, {
    BackendUriBuilder? uriBuilder,
    Dio? dio,
  })  : _uriBuilder = uriBuilder ?? BackendUriBuilder(),
        _dio = dio ?? _createDio(_config) {
    _state = BackendHttpState.available;
  }

  static Dio _createDio(BackendConnectionConfig config) {
    final baseUri = BackendUriBuilder().httpBase(config);
    return Dio(BaseOptions(
      baseUrl: baseUri.toString(),
      connectTimeout: const Duration(seconds: 5),
      receiveTimeout: const Duration(seconds: 30),
    ));
  }

  @override
  BackendHttpState get state => _closed ? BackendHttpState.closed : _state;

  @override
  Future<BackendHttpResponse> send(
    BackendHttpRequest request,
  ) async {
    if (_closed) {
      throw BackendTransportError(
        code: BackendTransportErrorCode.transportClosed,
        method: request.method.value,
        path: request.path,
        generation: _config.generation,
      );
    }

    final uri = _uriBuilder.http(
      _config,
      request.path,
      queryParameters: request.queryParameters,
    );

    final headers = <String, String>{
      BackendBaseHeaders.userAgent: BackendBaseHeaders.userAgentValue,
      if (request.body != null)
        BackendBaseHeaders.contentType: BackendBaseHeaders.contentTypeJsonValue,
    };

    final token = _config.credential.revealForTransport();
    switch (_config.authStrategy) {
      case BackendAuthStrategy.localToken:
        headers[BackendAuthHeader.localToken] = token;
      case BackendAuthStrategy.bearer:
        headers[BackendAuthHeader.authorization] = 'Bearer $token';
    }

    if (request.headers != null) {
      for (final entry in request.headers!.entries) {
        if (BackendAuthHeader.protectedHeaders.contains(entry.key)) {
          throw BackendTransportError(
            code: BackendTransportErrorCode.configInvalid,
            method: request.method.value,
            path: request.path,
            generation: _config.generation,
          );
        }
        headers[entry.key] = entry.value;
      }
    }

    _state = BackendHttpState.available;

    try {
      final response = await _dio.requestUri(
        uri,
        data: request.body,
        options: Options(
          method: request.method.value,
          headers: headers,
          validateStatus: (status) => true,
          receiveTimeout: request.timeout,
        ),
      );

      final statusCode = response.statusCode ?? 0;
      final responseHeaders = <String, String>{};
      response.headers.forEach((name, values) {
        if (values.isNotEmpty) responseHeaders[name] = values.first;
      });

      if (statusCode == 401 || statusCode == 403) {
        throw BackendTransportError(
          code: BackendTransportErrorCode.authenticationFailed,
          method: request.method.value,
          path: request.path,
          statusCode: statusCode,
          generation: _config.generation,
        );
      }

      return BackendHttpResponse(
        statusCode: statusCode,
        headers: responseHeaders,
        data: response.data,
      );
    } on DioException catch (e) {
      if (_closed) {
        throw BackendTransportError(
          code: BackendTransportErrorCode.transportClosed,
          method: request.method.value,
          path: request.path,
          generation: _config.generation,
          cause: e,
        );
      }

      switch (e.type) {
        case DioExceptionType.connectionTimeout:
        case DioExceptionType.sendTimeout:
        case DioExceptionType.receiveTimeout:
          throw BackendTransportError(
            code: BackendTransportErrorCode.requestTimeout,
            method: request.method.value,
            path: request.path,
            generation: _config.generation,
            cause: e,
          );
        case DioExceptionType.connectionError:
          throw BackendTransportError(
            code: BackendTransportErrorCode.connectionRefused,
            method: request.method.value,
            path: request.path,
            generation: _config.generation,
            cause: e,
          );
        case DioExceptionType.badResponse:
          final statusCode = e.response?.statusCode;
          if (statusCode != null && statusCode >= 500) {
            throw BackendTransportError(
              code: BackendTransportErrorCode.serverError,
              method: request.method.value,
              path: request.path,
              statusCode: statusCode,
              generation: _config.generation,
              cause: e,
            );
          }
          throw BackendTransportError(
            code: _mapClientErrorCode(statusCode),
            method: request.method.value,
            path: request.path,
            statusCode: statusCode,
            generation: _config.generation,
            cause: e,
          );
        case DioExceptionType.cancel:
          throw BackendTransportError(
            code: BackendTransportErrorCode.requestCancelled,
            method: request.method.value,
            path: request.path,
            generation: _config.generation,
            cause: e,
          );
        default:
          throw BackendTransportError(
            code: BackendTransportErrorCode.unknown,
            method: request.method.value,
            path: request.path,
            generation: _config.generation,
            cause: e,
          );
      }
    }
  }

  static BackendTransportErrorCode _mapClientErrorCode(int? statusCode) {
    if (statusCode == null) return BackendTransportErrorCode.unknown;
    switch (statusCode) {
      case 400:
        return BackendTransportErrorCode.badRequest;
      case 404:
        return BackendTransportErrorCode.notFound;
      case 409:
        return BackendTransportErrorCode.conflict;
      case 422:
        return BackendTransportErrorCode.validationFailed;
      case 503:
        return BackendTransportErrorCode.serviceUnavailable;
      default:
        return BackendTransportErrorCode.unknown;
    }
  }

  @override
  Future<void> close() async {
    if (_closed) return;
    _closed = true;
    _state = BackendHttpState.closed;
    _dio.close();
  }
}
