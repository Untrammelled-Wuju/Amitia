import '../api/api_client.dart';
import '../models/feedback.dart';
import '../models/mood.dart';

class FeedbackService {
  final ApiClient _api = ApiClient();

  Future<FeedbackDto?> create(String messageId, int rating, {String? comment}) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/messages/$messageId/feedback',
      data: {
        'rating': rating,
        if (comment != null) 'comment': comment,
      },
    );
    if (resp.data == null) return null;
    return FeedbackDto.fromJson(resp.data!);
  }

  Future<FeedbackDto?> getByMessage(String messageId) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/messages/$messageId/feedback',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp.data == null) return null;
    return FeedbackDto.fromJson(resp.data!);
  }

  Future<Map<String, dynamic>?> stats() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/messages/feedback/stats');
    return resp.data;
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/messages/feedback/$id');
    return true;
  }
}

class MoodService {
  final ApiClient _api = ApiClient();

  Future<List<MoodDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/moods');
    if (resp.data == null) return [];
    return resp.data!.map((e) => MoodDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<List<MoodDto>> getByConversation(String conversationId) async {
    final resp = await _api.get<List<dynamic>>('/api/moods/conversations/$conversationId');
    if (resp.data == null) return [];
    return resp.data!.map((e) => MoodDto.fromJson(e as Map<String, dynamic>)).toList();
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
