import '../backend_transport/backend_service_api.dart';
import '../models/continuity.dart';

class ContinuityService {
  ContinuityService(this._api);

  final BackendServiceApi _api;

  Future<List<ContinuityThreadDto>> list({
    String query = '',
    String status = '',
    int limit = 100,
  }) async {
    final response = await _api.get<List<dynamic>>(
      '/api/continuity/threads',
      queryParameters: {
        if (query.trim().isNotEmpty) 'q': query.trim(),
        if (status.trim().isNotEmpty) 'status': status.trim(),
        'limit': limit,
      },
    );
    return (response ?? const <dynamic>[])
        .whereType<Map>()
        .map(
          (item) =>
              ContinuityThreadDto.fromJson(Map<String, dynamic>.from(item)),
        )
        .toList(growable: false);
  }

  Future<ContinuityDetailDto> get(String id) async {
    final response = await _api.get<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(id)}',
    );
    return ContinuityDetailDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<ContinuityThreadDto> create(Map<String, dynamic> data) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/api/continuity/threads',
      data: data,
    );
    return ContinuityThreadDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<ContinuityThreadDto> update(
    String id,
    Map<String, dynamic> data,
  ) async {
    final response = await _api.patch<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(id)}',
      data: data,
    );
    return ContinuityThreadDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<ContinuityWaitDto> createWait(
    String threadId,
    Map<String, dynamic> data,
  ) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(threadId)}/waits',
      data: data,
    );
    return ContinuityWaitDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<ContinuityWaitDto> resolveWait(
    String threadId,
    String waitId, {
    bool resume = true,
  }) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(threadId)}/waits/${Uri.encodeComponent(waitId)}/resolve',
      data: {'resume': resume},
    );
    return ContinuityWaitDto.fromJson(response ?? const <String, dynamic>{});
  }

  Future<ContinuityWaitDto> cancelWait(String threadId, String waitId) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/api/continuity/threads/${Uri.encodeComponent(threadId)}/waits/${Uri.encodeComponent(waitId)}/cancel',
    );
    return ContinuityWaitDto.fromJson(response ?? const <String, dynamic>{});
  }
}
