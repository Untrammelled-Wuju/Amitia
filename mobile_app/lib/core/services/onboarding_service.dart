import 'package:dio/dio.dart';
import '../runtime/backend/backend_topology_resolver.dart';
import '../backend_transport/backend_service_api.dart';

class OnboardingService {
  OnboardingService(this._api);

  final BackendServiceApi _api;

  Dio _publicClientAt(String coreUri) {
    final baseUri = normalizeRemoteCoreUri(coreUri);
    return Dio(
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

  Future<Map<String, dynamic>> runtimeCapabilitiesAt(String coreUri) async {
    final dio = _publicClientAt(coreUri);
    try {
      final response = await dio.get<dynamic>('/api/public/runtime/capabilities');
      return _unwrapPublicResponse(response.data);
    } finally {
      dio.close(force: true);
    }
  }


  Future<Map<String, dynamic>> pairingStatusAt(String coreUri) async {
    final dio = _publicClientAt(coreUri);
    try {
      final response = await dio.get<dynamic>('/api/public/device-mesh/v1/pairing/status');
      return _unwrapPublicResponse(response.data);
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
  }) async {
    if (fingerprint.isNotEmpty) {
      final deadline = DateTime.now().add(const Duration(minutes: 2));
      while (DateTime.now().isBefore(deadline)) {
      final response = await _api.post<Map<String, dynamic>>('/internal/device-mesh/pairing/claim', data: {
        'endpoint': {'url': coreUri, 'fingerprint': fingerprint, 'coreId': coreId},
        'offerToken': offerToken.trim(), 'setupCode': setupCode.trim(), 'label': label.trim(),
      });
      if (response?['pending'] == true) {
        await Future<void>.delayed(const Duration(seconds: 1));
        continue;
      }
      if (response == null || response['spaceId'] != coreId) throw StateError('配对服务身份与二维码不一致');
      return response;
      }
      throw StateError('等待服务提供设备批准配对超时，可再次扫描同一码继续等待');
    }
    final dio = _publicClientAt(coreUri);
    try {
      final status = await pairingStatusAt(coreUri);
      final claimBody = <String, dynamic>{
        'deviceId': deviceId.trim(), 'runtimeId': runtimeId.trim(), 'platform': platform.trim(),
        'label': label.trim(), 'offerToken': offerToken.trim(), 'setupCode': setupCode.trim(),
      };
      final deadline = DateTime.now().add(const Duration(minutes: 2));
      while (DateTime.now().isBefore(deadline)) {
      final proof = await _api.post<Map<String, dynamic>>('/internal/device-mesh/identity/sign-claim', data: {'coreId': status['spaceId'], 'body': claimBody});
      if (proof == null) throw StateError('无法为本机配对身份签名');
      final response = await dio.post<dynamic>(
        '/api/public/device-mesh/v1/pairing/claim',
        data: <String, dynamic>{
          'deviceId': deviceId.trim(),
          'runtimeId': runtimeId.trim(),
          'platform': platform.trim(),
          if (label.trim().isNotEmpty) 'label': label.trim(),
          if (offerToken.trim().isNotEmpty) 'offerToken': offerToken.trim(),
          if (setupCode.trim().isNotEmpty) 'setupCode': setupCode.trim(),
          'proof': proof,
        },
      );
      final result = _unwrapPublicResponse(response.data);
      if (result['pending'] == true) { await Future<void>.delayed(const Duration(seconds: 1)); continue; }
      return result;
      }
      throw StateError('等待服务提供设备批准配对超时，请再次扫码');
    } finally {
      dio.close(force: true);
    }
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
      data: <String, dynamic>{
        'deployMode': deployMode,
        'webChatEnabled': true,
      },
    );
  }
}
