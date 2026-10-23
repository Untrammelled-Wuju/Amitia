import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/device_owned_chat_service.dart';
import 'package:amitia_app/core/services/feedback_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _FakeOwnedChatService extends DeviceOwnedChatService {
  _FakeOwnedChatService() : super(_FakeApi());

  String? operation;
  String? role;
  Map<String, dynamic>? body;

  @override
  Future<Map<String, dynamic>> management(
    String operation, {
    Map<String, dynamic> body = const {},
    bool post = false,
    String characterId = '',
    String group = 'chat-management',
  }) async {
    this.operation = operation;
    role = characterId;
    this.body = body;
    if (operation == 'feedback/create') {
      return {
        'feedback': {
          'id': 9,
          'messageId': body['messageId'],
          'feedbackType': body['feedbackType'],
          'reason': body['reason'],
          'revision': 1,
        },
      };
    }
    if (operation == 'feedback/list') {
      return {'items': <dynamic>[]};
    }
    return {'deleted': true, 'id': body['feedbackId'], 'revision': 2};
  }
}

class _FakeApi extends Fake implements BackendServiceApi {}

void main() {
  test('手机消息反馈真实传递角色、类型和原因', () async {
    final owned = _FakeOwnedChatService();
    final service = FeedbackService(_FakeApi(), owned: owned);
    final feedback = await service.submit(
      'reply',
      'unsafe',
      comment: 'risk',
      characterId: 'role',
    );
    expect(feedback?.id, '9');
    expect(owned.operation, 'feedback/create');
    expect(owned.role, 'role');
    expect(owned.body?['feedbackType'], 'unsafe');
    expect(owned.body?['reason'], 'risk');
  });
}
