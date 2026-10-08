import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/owned_realtime_invitation.dart';
import 'package:flutter_test/flutter_test.dart';

import 'owned_realtime_test.dart' show scope;
import 'owned_task_submission_test.dart' show TaskTransport;

const callId = 'aa464513-bdb4-4306-9bde-0c7c0397b7bf';

Map<String, dynamic> invitation() => {
  'id': callId,
  'recipientDeviceId': 'phone',
  'characterId': 'role',
  'conversationId': 'conversation',
  'executionScope': scope(),
  'nonce': 'a' * 43,
  'expiresAt': DateTime.now()
      .add(const Duration(seconds: 45))
      .toUtc()
      .toIso8601String(),
  'revision': 1,
  'status': 'pending',
};

void main() {
  test('接听使用原邀请授权且必须获得原所有者单次确认', () async {
    final result = <String, dynamic>{
      'saved': true,
      'invitation': {...invitation(), 'status': 'accepted', 'revision': 2},
      'ticket': {
        'ticket': 'b' * 43,
        'wsPath': '/api/device-mesh/v1/business/realtime/session',
        'executionScope': scope(),
      },
    };
    final transport = TaskTransport([
      {'invitation': invitation()},
      result,
    ]);
    transport.onSend = (request) {
      if (request.path.endsWith('/accept')) {
        final body = request.body as Map;
        result['acknowledgement'] = {
          'ownerId': 'phone',
          'requestId': body['requestId'],
          'versions': {'checkpoint/realtime-invitation/$callId': 2},
        };
      }
    };
    final accepted = await acceptOwnedRealtimeInvitation(
      api: BackendServiceApi(transport, 1),
      isCurrent: () => true,
      callId: callId,
      characterId: 'role',
    );
    expect(accepted.service.scope, scope());
    expect(accepted.service.conversationId, 'conversation');
    expect(accepted.ticket['ticket'], 'b' * 43);
    expect(
      (transport.requests.last.body as Map)['expectedExecutionScope'],
      scope(),
    );
  });

  test('其他接收设备和过期邀请均不得提交接听', () async {
    for (final changes in [
      {'recipientDeviceId': 'foreign'},
      {
        'expiresAt': DateTime.now()
            .subtract(const Duration(seconds: 1))
            .toIso8601String(),
      },
      {'status': 'accepted'},
    ]) {
      final transport = TaskTransport([
        {
          'invitation': {...invitation(), ...changes},
        },
      ]);
      await expectLater(
        acceptOwnedRealtimeInvitation(
          api: BackendServiceApi(transport, 1),
          isCurrent: () => true,
          callId: callId,
          characterId: 'role',
        ),
        throwsStateError,
      );
      expect(transport.requests.length, 1);
    }
  });

  test('连接切换后不得使用迟到邀请或发起接听', () async {
    var current = true;
    final transport = TaskTransport([
      {'invitation': invitation()},
    ]);
    transport.onSend = (_) => current = false;
    await expectLater(
      acceptOwnedRealtimeInvitation(
        api: BackendServiceApi(transport, 1),
        isCurrent: () => current,
        callId: callId,
        characterId: 'role',
      ),
      throwsStateError,
    );
    expect(transport.requests.length, 1);
  });

  test('缺少所有者保存确认不得连接通话', () async {
    final transport = TaskTransport([
      {'invitation': invitation()},
      {
        'saved': true,
        'ticket': {'ticket': 'b' * 43},
      },
    ]);
    await expectLater(
      acceptOwnedRealtimeInvitation(
        api: BackendServiceApi(transport, 1),
        isCurrent: () => true,
        callId: callId,
        characterId: 'role',
      ),
      throwsStateError,
    );
  });
}
