import '../backend_transport/backend_service_api.dart';
import '../models/feedback.dart';
import '../models/mood.dart';

class FeedbackService {
  final BackendServiceApi _api;

  FeedbackService(this._api);

  Future<FeedbackDto?> create(String messageId, int rating, {String? comment}) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/messages/$messageId/feedback',
      data: {
        'rating': rating,
        if (comment != null) 'comment': comment,
      },
    );
    if (resp == null) return null;
    return FeedbackDto.fromJson(resp);
  }

  Future<FeedbackDto?> getByMessage(String messageId) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/messages/$messageId/feedback',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp == null) return null;
    return FeedbackDto.fromJson(resp);
  }

  Future<Map<String, dynamic>?> stats() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/messages/feedback/stats');
    return resp;
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/messages/feedback/$id');
    return true;
  }
}

class MoodService {
  final BackendServiceApi _api;

  MoodService(this._api);

  Future<List<MoodDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/moods');
    if (resp == null) return [];
    return resp.map((e) => MoodDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<List<MoodDto>> getByConversation(String conversationId) async {
    final resp = await _api.get<List<dynamic>>('/api/moods/conversations/$conversationId');
    if (resp == null) return [];
    return resp.map((e) => MoodDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/moods/$id');
    return true;
  }

  Future<bool> deleteByConversation(String conversationId) async {
    await _api.delete('/api/moods/conversations/$conversationId');
    return true;
  }
}
