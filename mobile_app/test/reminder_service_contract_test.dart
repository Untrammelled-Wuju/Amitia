import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/reminder_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _Api implements BackendServiceApi {
  dynamic response;
  Map<String, dynamic>? sent;
  Map<String, dynamic>? query;
  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    query = queryParameters;
    return response as T?;
  }

  @override
  Future<T?> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    sent = data as Map<String, dynamic>?;
    return response as T?;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  test('列表拒绝损坏记录而不静默丢弃', () async {
    final api = _Api()..response = [null];
    await expectLater(ReminderService(api).list(), throwsStateError);
    api.response = [
      {
        'id': 'task',
        'generation': 1,
        'ownerId': 'core',
        'saved': true,
        'enabled': 1,
      },
    ];
    expect((await ReminderService(api).list()).single.id, 'task');
    expect(api.query?['kind'], 'user_reminder');
  });
  test('创建必须返回持久化确认与执行核心版本', () async {
    final api = _Api()..response = {'id': 'task'};
    await expectLater(ReminderService(api).create({}), throwsStateError);
  });
  test('手动执行发送版本并仅确认受理，拒绝空回执', () async {
    final api = _Api()
      ..response = {
        'accepted': true,
        'triggerId': 'trigger',
        'triggered': false,
      };
    final result = await ReminderService(api).trigger('task', 7);
    expect(api.sent, {'generation': 7});
    expect(result['triggered'], false);
    api.response = <String, dynamic>{};
    await expectLater(
      ReminderService(api).trigger('task', 7),
      throwsStateError,
    );
  });
  test('渠道和历史必须返回有效的真实结构', () async {
    final api = _Api()..response = ['wechat', null];
    await expectLater(ReminderService(api).channels(), throwsStateError);
    api.response = {'items': [], 'total': 0};
    expect((await ReminderService(api).triggerHistory())['total'], 0);
    api.response = <String, dynamic>{};
    await expectLater(ReminderService(api).triggerHistory(), throwsStateError);
  });
}
