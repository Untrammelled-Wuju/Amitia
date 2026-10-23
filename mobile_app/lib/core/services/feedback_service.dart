import '../backend_transport/backend_service_api.dart';
import '../models/feedback.dart';
import 'device_owned_chat_service.dart';

class FeedbackService {
  final DeviceOwnedChatService _owned;

  FeedbackService(BackendServiceApi api, {DeviceOwnedChatService? owned})
    : _owned = owned ?? DeviceOwnedChatService(api);

  Future<FeedbackDto?> create(
    String messageId,
    int rating, {
    String? comment,
  }) async {
    return submit(messageId, rating > 0 ? 'good' : 'other', comment: comment);
  }

  Future<FeedbackDto?> submit(
    String messageId,
    String feedbackType, {
    String? comment,
    String characterId = '',
  }) async {
    final resp = await _owned.management(
      'feedback/create',
      characterId: characterId,
      body: {
        'messageId': messageId,
        'feedbackType': feedbackType,
        'reason': comment?.trim() ?? '',
      },
    );
    final feedback = resp['feedback'];
    if (feedback is! Map) throw StateError('反馈保存结果无效');
    return FeedbackDto.fromJson(Map<String, dynamic>.from(feedback));
  }

  Future<FeedbackDto?> getByMessage(
    String messageId, {
    String characterId = '',
  }) async {
    final resp = await _owned.management(
      'feedback/list',
      characterId: characterId,
      body: {'messageId': messageId},
    );
    final items = resp['items'];
    if (items is! List) throw StateError('反馈查询结果无效');
    if (items.isEmpty) return null;
    return FeedbackDto.fromJson(Map<String, dynamic>.from(items.first as Map));
  }

  Future<Map<String, dynamic>?> stats({String characterId = ''}) async {
    return _owned.management('feedback-stats', characterId: characterId);
  }

  Future<bool> delete(String id, {String characterId = ''}) async {
    final feedbackId = int.tryParse(id);
    if (feedbackId == null || feedbackId <= 0) {
      throw ArgumentError.value(id, 'id');
    }
    final result = await _owned.management(
      'feedback/delete',
      characterId: characterId,
      body: {'feedbackId': feedbackId},
    );
    if (result['deleted'] != true || result['id'].toString() != id) {
      throw StateError('反馈删除尚未确认');
    }
    return true;
  }
}
