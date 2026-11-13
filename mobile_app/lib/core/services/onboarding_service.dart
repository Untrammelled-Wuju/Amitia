import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:dio/io.dart';

import '../errors/original_error.dart';
import '../runtime/backend/backend_topology_resolver.dart';
import '../backend_transport/backend_service_api.dart';
import '../backend_transport/errors/backend_transport_error.dart';
import '../backend_transport/errors/backend_transport_error_code.dart';
import '../security/certificate_pin.dart';

class OnboardingService {
  OnboardingService(
    this._api, {
    Duration pairingApprovalWait = const Duration(minutes: 31),
  }) : _pairingApprovalWait = pairingApprovalWait;

  final BackendServiceApi _api;
  final Duration _pairingApprovalWait;

  Dio _publicClientAt(
    String coreUri, {
    void Function(X509Certificate certificate)? onBadCertificate,
  }) {
    final baseUri = normalizeRemoteCoreUri(coreUri);
    final dio = Dio(
      BaseOptions(
        baseUrl: baseUri.toString().replaceAll(RegExp(r'/+$'), ''),
        connectTimeout: const Duration(seconds: 8),
        receiveTimeout: const Duration(seconds: 15),
        headers: const <String, String>{
          'Accept': 'application/json',
          'X-Amitia-Client-Type': 'mobile',
        },
      ),
    );
    if (onBadCertificate != null) {
      dio.httpClientAdapter = IOHttpClientAdapter(
        createHttpClient: () {
          final client = HttpClient();
          client.badCertificateCallback = (certificate, host, port) {
            onBadCertificate(certificate);
            return true;
          };
          return client;
        },
      );
    }
    return dio;
  }

  Map<String, dynamic> _unwrapPublicResponse(dynamic raw) {
    if (raw is! Map) return const <String, dynamic>{};
    final outer = Map<String, dynamic>.from(raw);
    final data = outer['data'];
    return data is Map ? Map<String, dynamic>.from(data) : outer;
  }

  Future<bool> livenessAt(String coreUri) async {
    final dio = _publicClientAt(coreUri);
    try {
      final response = await dio.get<dynamic>(
        '/livez',
        options: Options(validateStatus: (status) => status != null),
      );
      return response.statusCode == 200;
    } finally {
      dio.close(force: true);
    }
  }

  Future<bool> readinessAt(String coreUri) async {
    final dio = _publicClientAt(coreUri);
    try {
      final response = await dio.get<dynamic>(
        '/readyz',
        options: Options(validateStatus: (status) => status != null),
      );
      return response.statusCode == 200;
    } finally {
      dio.close(force: true);
    }
  }

  Future<Map<String, dynamic>> healthAt(String coreUri) async {
    final dio = _publicClientAt(coreUri);
    try {
      final response = await dio.get<dynamic>('/api/public/health');
      return _unwrapPublicResponse(response.data);
    } finally {
      dio.close(force: true);
    }
  }

  Future<Map<String, dynamic>> coreInfoAt(String coreUri) async {
    final dio = _publicClientAt(coreUri);
    try {
      final response = await dio.get<dynamic>('/api/public/core/info');
      return _unwrapPublicResponse(response.data);
    } finally {
      dio.close(force: true);
    }
  }

  Future<Map<String, dynamic>> runtimeCapabilitiesAt(String coreUri) async {
    final dio = _publicClientAt(coreUri);
    try {
      final response = await dio.get<dynamic>(
        '/api/public/runtime/capabilities',
      );
      return _unwrapPublicResponse(response.data);
    } finally {
      dio.close(force: true);
    }
  }

  Future<Map<String, dynamic>> pairingStatusAt(String coreUri) async {
    final dio = _publicClientAt(coreUri);
    try {
      final response = await dio.get<dynamic>(
        '/api/public/device-mesh/v1/pairing/status',
      );
      return _unwrapPublicResponse(response.data);
    } finally {
      dio.close(force: true);
    }
  }

  Future<Map<String, dynamic>> discoverPairingAt(String coreUri) async {
    X509Certificate? untrustedCertificate;
    final dio = _publicClientAt(
      coreUri,
      onBadCertificate: (certificate) {
        untrustedCertificate = certificate;
      },
    );
    try {
      final response = await dio.get<dynamic>(
        '/api/public/device-mesh/v1/pairing/status',
      );
      final status = _unwrapPublicResponse(response.data);
      return <String, dynamic>{
        ...status,
        'fingerprint': untrustedCertificate == null
            ? ''
            : certificatePublicKeyInfoSha256(untrustedCertificate!.der),
      };
    } finally {
      dio.close(force: true);
    }
  }

  Future<Map<String, dynamic>> claimPairingAt(
    String coreUri, {
    required String deviceId,
    required String runtimeId,
    required String platform,
    String label = '',
    String offerToken = '',
    String setupCode = '',
    String fingerprint = '',
    String coreId = '',
    Future<Map<String, dynamic>> Function(
      Map<String, dynamic> claimBody,
      String coreId,
    )?
    proofSigner,
  }) async {
    if (fingerprint.isNotEmpty) {
      var currentOfferToken = offerToken.trim();
      Future<Map<String, dynamic>?> claimPinned() async {
        final response = await _api.post<Map<String, dynamic>>(
          '/internal/device-mesh/pairing/claim',
          data: {
            'endpoint': {
              'url': coreUri,
              'fingerprint': fingerprint,
              'coreId': coreId,
            },
            'offerToken': currentOfferToken,
            'setupCode': setupCode.trim(),
            'label': label.trim(),
          },
        );
        final returnedOfferToken = (response?['offerToken'] ?? '')
            .toString()
            .trim();
        if (currentOfferToken.isEmpty && returnedOfferToken.isNotEmpty) {
          currentOfferToken = returnedOfferToken;
        }
        return response;
      }

      late Map<String, dynamic>? response;
      try {
        response = await claimPinned();
      } catch (error, stackTrace) {
        Object? updateError;
        try {
          final updated = await _api.post<Map<String, dynamic>>(
            '/internal/device-mesh/endpoint/update',
            data: {
              'cloudBaseUrl': coreUri,
              'coreId': coreId,
              if (fingerprint.isNotEmpty) 'fingerprint': fingerprint,
            },
            headers: const {'X-Amitia-Request-Timeout-Seconds': '50'},
          );
          if (updated?['ok'] == true) {
            return {
              'alreadyPaired': true,
              'spaceId': coreId,
              'deviceId': deviceId,
              'runtimeId': runtimeId,
            };
          }
          updateError = StateError('本机设备未确认统筹中心地址更新结果');
        } catch (error) {
          updateError = error;
        }
        Map<String, dynamic>? status;
        try {
          status = await _api.get<Map<String, dynamic>>(
            '/internal/device-mesh/status',
          );
        } catch (statusError) {
          Error.throwWithStackTrace(
            StateError(
              '${originalErrorText(error)}\n${originalErrorText(statusError)}',
            ),
            stackTrace,
          );
        }
        if ((status?['state'] ?? '').toString().toLowerCase() != 'revoked' &&
            !_isRevokedCredentialError(error)) {
          Error.throwWithStackTrace(
            StateError(
              '${originalErrorText(error)}\n${originalErrorText(updateError!)}',
            ),
            stackTrace,
          );
        }
        String? cleanupError;
        try {
          await _api.delete('/internal/device-mesh/credential');
        } catch (error) {
          cleanupError = originalErrorText(error);
        }
        try {
          response = await claimPinned();
        } catch (error, stackTrace) {
          final details = [originalErrorText(error), ?cleanupError];
          Error.throwWithStackTrace(StateError(details.join('\n')), stackTrace);
        }
      }
      if (response?['pending'] == true) {
        final requestId = (response?['requestId'] ?? '').toString().trim();
        if (requestId.isEmpty) throw StateError('统筹中心未返回配对申请编号');
        await _waitPairingEvents(
          () => _api.getStream(
            '/internal/device-mesh/pairing/requests/$requestId/events',
            queryParameters: {
              'url': coreUri,
              'fingerprint': fingerprint,
              'coreId': coreId,
            },
            headers: const {'Accept': 'text/event-stream'},
          ),
        );
        response = await claimPinned();
      }
      if (response == null || response['spaceId'] != coreId) {
        throw StateError('配对服务身份与二维码不一致');
      }
      return response;
    }
    final dio = _publicClientAt(coreUri);
    try {
      final status = await pairingStatusAt(coreUri);
      var currentOfferToken = offerToken.trim();
      Future<Map<String, dynamic>> claimRemote() async {
        final claimBody = <String, dynamic>{
          'deviceId': deviceId.trim(),
          'runtimeId': runtimeId.trim(),
          'platform': platform.trim(),
          'label': label.trim(),
          'offerToken': currentOfferToken,
          'setupCode': setupCode.trim(),
        };
        final statusCoreId = (status['spaceId'] ?? '').toString().trim();
        if (statusCoreId.isEmpty) throw StateError('统筹中心身份缺失');
        final proof = proofSigner != null
            ? await proofSigner(claimBody, statusCoreId)
            : await _api.post<Map<String, dynamic>>(
                '/internal/device-mesh/identity/sign-claim',
                data: {'coreId': statusCoreId, 'body': claimBody},
              );
        if (proof == null || proof.isEmpty) throw StateError('无法为本机配对身份签名');
        final response = await dio.post<dynamic>(
          '/api/public/device-mesh/v1/pairing/claim',
          data: <String, dynamic>{
            'deviceId': deviceId.trim(),
            'runtimeId': runtimeId.trim(),
            'platform': platform.trim(),
            if (label.trim().isNotEmpty) 'label': label.trim(),
            if (currentOfferToken.isNotEmpty) 'offerToken': currentOfferToken,
            if (setupCode.trim().isNotEmpty) 'setupCode': setupCode.trim(),
            'proof': proof,
          },
        );
        final result = _unwrapPublicResponse(response.data);
        final returnedOfferToken = (result['offerToken'] ?? '')
            .toString()
            .trim();
        if (currentOfferToken.isEmpty && returnedOfferToken.isNotEmpty) {
          currentOfferToken = returnedOfferToken;
        }
        return result;
      }

      var result = await claimRemote();
      if (result['pending'] == true) {
        final requestId = (result['requestId'] ?? '').toString().trim();
        if (requestId.isEmpty) throw StateError('统筹中心未返回配对申请编号');
        await _waitPairingEvents(() async {
          final next = await dio.get<ResponseBody>(
            '/api/public/device-mesh/v1/pairing/requests/$requestId/events',
            options: Options(
              responseType: ResponseType.stream,
              receiveTimeout: _pairingApprovalWait,
              headers: const {'Accept': 'text/event-stream'},
            ),
          );
          final nextBody = next.data;
          if (nextBody == null) {
            throw StateError('配对审批事件流缺少响应正文');
          }
          return nextBody.stream.map<List<int>>((chunk) => chunk);
        });
        result = await claimRemote();
      }
      return result;
    } on DioException catch (error) {
      final response = _unwrapPublicResponse(error.response?.data);
      final message = (response['message'] ?? response['error'] ?? '')
          .toString()
          .trim();
      if (message.isNotEmpty) throw StateError(message);
      if (error.type == DioExceptionType.connectionTimeout ||
          error.type == DioExceptionType.receiveTimeout ||
          error.type == DioExceptionType.sendTimeout) {
        throw StateError('配对请求超时，请检查两台设备的连接后重试');
      }
      throw StateError(originalErrorText(error));
    } finally {
      dio.close(force: true);
    }
  }

  Future<void> _waitPairingEvents(
    Future<Stream<List<int>>> Function() openStream,
  ) async {
    final deadline = DateTime.now().add(_pairingApprovalWait);
    Stream<List<int>>? stream;
    var reconnectAttempt = 0;
    Object? lastError;
    while (DateTime.now().isBefore(deadline)) {
      try {
        if (stream == null) {
          try {
            stream = await openStream();
          } catch (error, stackTrace) {
            if (!_retryablePairingError(error)) {
              Error.throwWithStackTrace(error, stackTrace);
            }
            throw _PairingApprovalReconnect(error, stackTrace);
          }
        }
        await _consumePairingEvents(stream, deadline);
        return;
      } catch (error, stackTrace) {
        lastError = error;
        if (error is! _PairingApprovalReconnect) {
          Error.throwWithStackTrace(error, stackTrace);
        }
        stream = null;
        reconnectAttempt++;
        final seconds = reconnectAttempt > 5 ? 10 : 1 << (reconnectAttempt - 1);
        final delay = Duration(seconds: seconds);
        if (!DateTime.now().add(delay).isBefore(deadline)) break;
        await Future<void>.delayed(delay);
      }
    }
    final detail = lastError == null ? '' : '\n${originalErrorText(lastError)}';
    throw StateError('等待配对审批超时，请重新生成二维码$detail');
  }

  bool _retryablePairingError(Object error) {
    if (error is BackendTransportError) {
      return const {
        BackendTransportErrorCode.connectionRefused,
        BackendTransportErrorCode.connectionReset,
        BackendTransportErrorCode.networkUnreachable,
        BackendTransportErrorCode.requestTimeout,
        BackendTransportErrorCode.serverError,
        BackendTransportErrorCode.serviceUnavailable,
        BackendTransportErrorCode.responseInvalid,
        BackendTransportErrorCode.transportClosed,
        BackendTransportErrorCode.unknown,
      }.contains(error.code);
    }
    if (error is DioException) {
      return error.type == DioExceptionType.connectionTimeout ||
          error.type == DioExceptionType.sendTimeout ||
          error.type == DioExceptionType.receiveTimeout ||
          error.type == DioExceptionType.connectionError ||
          (error.response?.statusCode ?? 0) >= 500;
    }
    return error is SocketException ||
        error is HttpException ||
        error is TimeoutException;
  }

  bool _isRevokedCredentialError(Object error) {
    final text = originalErrorText(error).toLowerCase();
    return text.contains('credential: revoked');
  }

  Future<void> _consumePairingEvents(
    Stream<List<int>> stream,
    DateTime overallDeadline,
  ) async {
    final decision = Completer<void>();
    final buffer = StringBuffer();
    var deadline = overallDeadline;
    Timer? deadlineTimer;
    late StreamSubscription<String> subscription;
    void setDeadline(DateTime value) {
      if (value.isBefore(deadline)) deadline = value;
      deadlineTimer?.cancel();
      final remaining = deadline.difference(DateTime.now());
      deadlineTimer = Timer(
        remaining.isNegative ? Duration.zero : remaining,
        () {
          if (!decision.isCompleted) {
            decision.completeError(StateError('配对申请已过期，请重新生成二维码'));
          }
        },
      );
    }

    setDeadline(overallDeadline);
    subscription = stream
        .transform(utf8.decoder)
        .listen(
          (chunk) {
            buffer.write(chunk);
            var value = buffer.toString();
            var boundary = value.indexOf('\n\n');
            while (boundary >= 0) {
              final block = value.substring(0, boundary);
              value = value.substring(boundary + 2);
              buffer
                ..clear()
                ..write(value);
              final event = _pairingEvent(block);
              final expiry = _pairingEventExpiry(event.$2);
              if (expiry != null) {
                setDeadline(expiry.add(const Duration(seconds: 5)));
              }
              if (event.$1 == 'pairing_approved' && !decision.isCompleted) {
                decision.complete();
              } else if (event.$1 == 'pairing_rejected' &&
                  !decision.isCompleted) {
                decision.completeError(
                  StateError(event.$2.isEmpty ? '统筹中心拒绝了本次配对' : event.$2),
                );
              } else if (event.$1 == 'pairing_expired' &&
                  !decision.isCompleted) {
                decision.completeError(StateError('配对申请已过期，请重新生成二维码'));
              }
              boundary = value.indexOf('\n\n');
            }
          },
          onError: (Object error, StackTrace stackTrace) {
            if (!decision.isCompleted) {
              decision.completeError(
                _PairingApprovalReconnect(error, stackTrace),
                stackTrace,
              );
            }
          },
          onDone: () {
            if (!decision.isCompleted) {
              decision.completeError(const _PairingApprovalReconnect());
            }
          },
          cancelOnError: true,
        );
    try {
      await decision.future;
    } finally {
      deadlineTimer?.cancel();
      await subscription.cancel();
    }
  }

  DateTime? _pairingEventExpiry(String raw) {
    if (raw.isEmpty) return null;
    try {
      final payload = jsonDecode(raw);
      if (payload is! Map) return null;
      final value = payload['expiresAt']?.toString().trim() ?? '';
      if (value.isEmpty) return null;
      return DateTime.tryParse(value)?.toUtc();
    } catch (_) {
      return null;
    }
  }

  (String, String) _pairingEvent(String block) {
    var event = 'message';
    final data = <String>[];
    for (final line in const LineSplitter().convert(block)) {
      if (line.startsWith('event:')) event = line.substring(6).trim();
      if (line.startsWith('data:')) data.add(line.substring(5).trimLeft());
    }
    final raw = data.join('\n');
    if (event == 'pairing_rejected' && raw.isNotEmpty) {
      try {
        final payload = jsonDecode(raw);
        if (payload is Map && payload['message'] != null) {
          return (event, payload['message'].toString());
        }
      } catch (_) {
        return (event, raw);
      }
    }
    return (event, raw);
  }

  Future<Map<String, dynamic>> health() async {
    return await _api.get<Map<String, dynamic>>('/api/public/health') ??
        const <String, dynamic>{};
  }

  Future<Map<String, dynamic>> runtimeCapabilities() async {
    return await _api.get<Map<String, dynamic>>(
          '/api/public/runtime/capabilities',
        ) ??
        const <String, dynamic>{};
  }

  Future<Map<String, dynamic>> onboardingStatus() async {
    return await _api.get<Map<String, dynamic>>(
          '/api/public/onboarding/status',
        ) ??
        const <String, dynamic>{};
  }

  Future<List<Map<String, dynamic>>> detectModels({
    required String baseUrl,
    required String apiKey,
    String apiType = 'openai-compatible',
  }) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/api/model/detect-models',
      data: <String, dynamic>{
        'baseUrl': baseUrl.trim(),
        'apiKey': apiKey.trim(),
        'apiType': apiType,
      },
    );
    final models = response?['models'];
    if (models is! List) return const <Map<String, dynamic>>[];
    return models
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }

  Future<void> complete({required String deployMode}) async {
    await _api.post<Map<String, dynamic>>(
      '/api/onboarding/complete',
      data: <String, dynamic>{'deployMode': deployMode, 'webChatEnabled': true},
    );
  }
}

class _PairingApprovalReconnect implements Exception {
  const _PairingApprovalReconnect([this.cause, this.stackTrace]);

  final Object? cause;
  final StackTrace? stackTrace;

  @override
  String toString() => cause == null ? '配对审批事件流已断开' : cause.toString();
}
