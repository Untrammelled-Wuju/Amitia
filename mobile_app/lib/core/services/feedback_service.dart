import '../backend_transport/backend_service_api.dart';
import '../models/feedback.dart';
import '../models/mood.dart';
import 'device_owned_chat_service.dart';

class FeedbackService {
  final DeviceOwnedChatService _owned;

  FeedbackService(BackendServiceApi api, {DeviceOwnedChatService? owned})
    : _owned = owned ?? DeviceOwnedChatService(api);

  Future<FeedbackDto?> create(String messageId, int rating, {String? comment}) async {
    final resp = await _owned.management(
      'feedback/create',
      body: {
        'messageId': messageId,
        'feedbackType': rating > 0 ? 'good' : 'other',
        'reason': 'rating=$rating${comment == null || comment.isEmpty ? '' : '; $comment'}',
      },
    );
    final feedback = resp['feedback'];
    if (feedback is! Map) throw StateError('反馈保存结果无效');
    return FeedbackDto.fromJson(Map<String, dynamic>.from(feedback));
  }

  Future<FeedbackDto?> getByMessage(String messageId) async {
    final resp = await _owned.management(
      'feedback/list',
      body: {'messageId': messageId},
    );
    final items = resp['items'];
    if (items is! List) throw StateError('反馈查询结果无效');
    if (items.isEmpty) return null;
    return FeedbackDto.fromJson(Map<String, dynamic>.from(items.first as Map));
  }

  Future<Map<String, dynamic>?> stats() async {
    return _owned.management('feedback-stats');
  }

  Future<bool> delete(String id) async {
    final feedbackId = int.tryParse(id);
    if (feedbackId == null || feedbackId <= 0) throw ArgumentError.value(id, 'id');
    final result = await _owned.management('feedback/delete', body: {'feedbackId': feedbackId});
    if (result['deleted'] != true || result['id'].toString() != id) {
      throw StateError('反馈删除尚未确认');
    }
    return true;
  }
}

class MoodService {
  final BackendServiceApi _api;

  MoodService(this._api);

  Future<List<MoodDto>> list() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/moods');
    final items = resp?['moods'];
    if (items is! List) return const [];
    return items.whereType<Map>().map((e) => MoodDto.fromJson(Map<String, dynamic>.from(e))).toList(growable: false);
  }

  Future<List<MoodDto>> getByConversation(String conversationId) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/moods/conversations/$conversationId');
    final items = resp?['items'];
    if (items is! List) return const [];
    return items.whereType<Map>().map((e) => MoodDto.fromJson(Map<String, dynamic>.from(e))).toList(growable: false);
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
