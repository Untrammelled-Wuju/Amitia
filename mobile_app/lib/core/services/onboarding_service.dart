import '../backend_transport/backend_service_api.dart';

class OnboardingService {
  OnboardingService(this._api);

  final BackendServiceApi _api;

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
      '/api/public/model/detect-models',
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

  Future<void> complete({
    required String deployMode,
    required String username,
  }) async {
    await _api.post<Map<String, dynamic>>(
      '/api/public/onboarding/complete',
      data: <String, dynamic>{
        'deployMode': deployMode,
        'username': username,
        'webChatEnabled': true,
      },
    );
  }
}
