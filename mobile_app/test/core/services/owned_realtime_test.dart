import 'dart:convert';
import 'dart:typed_data';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/device_owned_realtime_service.dart';
import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';

import 'owned_task_submission_test.dart' show TaskTransport;

Map<String, dynamic> scope() => {
  'spaceId': 'space',
  'coreId': 'core',
  'initiatorDeviceId': 'phone',
  'targetDeviceId': 'phone',
  'roleOwnerId': 'phone',
  'resourceOwnerId': 'phone',
  'authorizationRealm': 'device',
  'roleId': 'role',
  'roleRevision': 1,
  'providerEpoch': 1,
  'targetProviderEpoch': 1,
  'modeRevision': 1,
  'permissionRevision': 1,
  'targetPermissionRevision': 1,
  'coordinated': false,
  'requestId': 'view',
  'turnId': '',
  'executionId': '',
};

DeviceOwnedRealtimeService service(
  TaskTransport transport, {
  bool Function()? current,
}) => DeviceOwnedRealtimeService(
  api: BackendServiceApi(transport, 1),
  isCurrent: current ?? () => true,
  scope: scope(),
  characterId: 'role',
  conversationId: 'conversation',
  conversationOrigin: const {'ownerId': 'phone', 'id': 'conversation'},
);

Map<String, dynamic> audio(String request) {
  final digest = sha256
      .convert(utf8.encode('core\u0000phone\u0000$request'))
      .toString()
      .substring(0, 32);
  final bytes = utf8.encode('ID3audio');
  return {
    'requestId': request,
    'saved': true,
    'executionScope': {
      ...scope(),
      'requestId': 'speech/$request',
      'turnId': 'turn',
      'executionId': 'execution',
    },
    'acknowledgement': {
      'ownerId': 'phone',
      'requestId': 'speech/$request',
      'versions': {
        'checkpoint/speech/$digest': 2,
        'tool-result/speech/$digest': 1,
      },
    },
    'audio': {
      'mime': 'audio/mpeg',
      'data': base64Encode(bytes),
      'sha256': sha256.convert(bytes).toString(),
    },
  };
}

void main() {
  test('票据冻结原角色范围与历史会话归属', () async {
    final transport = TaskTransport([
      {'executionScope': scope()},
      {
        'ticket': 'a' * 43,
        'wsPath': '/api/device-mesh/v1/business/realtime/session',
        'executionScope': scope(),
      },
    ]);
    final owned = service(transport);
    await owned.ticket();
    final body = transport.requests.last.body as Map;
    expect(body['expectedExecutionScope'], scope());
    expect(body['conversationOrigin'], {
      'ownerId': 'phone',
      'id': 'conversation',
    });
    expect(() => owned.scope['roleRevision'] = 2, throwsUnsupportedError);
  });
  test('同Core ABA拒绝旧通话票据', () async {
    final transport = TaskTransport([
      {
        'executionScope': {...scope(), 'providerEpoch': 3},
      },
    ]);
    await expectLater(service(transport).ticket(), throwsStateError);
    expect(transport.requests.length, 1);
  });
  test('票据请求期间换连接丢弃迟到票据', () async {
    var current = true;
    final transport = TaskTransport([
      {'executionScope': scope()},
      {
        'ticket': 'a' * 43,
        'wsPath': '/api/device-mesh/v1/business/realtime/session',
        'executionScope': scope(),
      },
    ]);
    transport.onSend = (request) {
      if (request.path.endsWith('tickets')) current = false;
    };
    await expectLater(
      service(transport, current: () => current).ticket(),
      throwsStateError,
    );
  });
  test('完成消息必须保存且ASR已ACK第二版本', () {
    final owned = service(TaskTransport([]));
    final result = {
      'executionScope': {...scope(), 'requestId': 'turn'},
      'requestId': 'turn',
      'saved': true,
      'userRevision': 2,
      'conversationId': 'conversation',
      'transcription': '已确认转写',
    };
    owned.validateCompleted(result, 'turn');
    expect(
      () => owned.validateCompleted({...result, 'userRevision': 1}, 'turn'),
      throwsStateError,
    );
    expect(
      () => owned.validateCompleted({...result, 'saved': false}, 'turn'),
      throwsStateError,
    );
  });
  test('朗读校验精确OwnerACK及SHA后才允许播放', () async {
    final transport = TaskTransport([
      {'executionScope': scope()},
    ]);
    final result = await service(transport).audio(audio('turn'), 'turn');
    expect(utf8.decode(result.bytes), 'ID3audio');
    final wrong = audio('turn');
    (wrong['acknowledgement'] as Map)['ownerId'] = 'foreign';
    await expectLater(
      service(TaskTransport([])).audio(wrong, 'turn'),
      throwsStateError,
    );
    final hash = audio('turn');
    (hash['audio'] as Map)['sha256'] = '0' * 64;
    await expectLater(
      service(TaskTransport([])).audio(hash, 'turn'),
      throwsStateError,
    );
  });
  test('停止通话后所有迟到结果与视觉帧均拒绝', () async {
    final owned = service(TaskTransport([]));
    owned.close();
    expect(
      () => owned.validateScope({'executionScope': scope()}),
      throwsStateError,
    );
    expect(
      () => owned.image(Uint8List.fromList([1]), 'image/jpeg'),
      throwsStateError,
    );
    await expectLater(owned.audio(audio('turn'), 'turn'), throwsStateError);
  });
  test('视觉发送实际原始字节及哈希限制', () {
    final owned = service(TaskTransport([]));
    final bytes = Uint8List.fromList([1, 2, 3]);
    final frame = owned.image(bytes, 'image/png');
    expect(base64Decode(frame['data'] as String), bytes);
    expect(frame['sha256'], sha256.convert(bytes).toString());
    expect(
      () => owned.image(Uint8List(1048577), 'image/png'),
      throwsStateError,
    );
  });
}
