import 'dart:convert';
import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/services/device_owned_chat_service.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:dio/dio.dart';
import 'package:amitia_app/core/services/owned_conversation_reference.dart';
import 'package:amitia_app/core/services/chat_service.dart';
import 'package:amitia_app/core/services/device_owned_attachments.dart';

class _OwnedApi implements BackendServiceApi {
  String core = 'core-b';
  Map<String, dynamic> resource = {
    'kind': 'message',
    'id': 'message',
    'ownerId': 'device-a',
    'roleId': 'role-a',
    'revision': 7,
    'deleted': false,
    'body': {'content': 'old'},
  };
  Map<String, dynamic>? editRequest;
  Map<String, dynamic>? editAck;
  Map<String, dynamic>? projectionResponse;
  bool wrongAck = false;
  bool summaryMode = false;
  Completer<Map<String, dynamic>>? deferred;
  final pages = <Map<String, dynamic>>[];
  final queries = <Map<String, dynamic>>[];
  Map<String, dynamic>? streamRequest;
  String queryPath = '';
  List<Map<String, dynamic>> streamEvents = [];

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    queryPath = path;
    if (path.endsWith('/projections') && projectionResponse != null)
      return projectionResponse as T;
    if (path.endsWith('/coordination/me'))
      return {
            'coreId': core,
            'coordinationAvailable': true,
            'policy': {
              'coordinated': false,
              'modeRevision': 1,
              'providerEpoch': 1,
              'permissionRevision': 1,
              'selectedRole': 'role-a',
            },
          }
          as T;
    if (path.endsWith('/roles'))
      return {
            'roles': [
              {'id': 'role-a', 'name': 'A', 'revision': 1},
            ],
          }
          as T;
    if (path.endsWith('/resources'))
      return {
            'resource': summaryMode ? null : resource,
            if (summaryMode) 'executionScope': scope(),
          }
          as T;
    if (deferred != null) return await deferred!.future as T;
    queries.add(Map<String, dynamic>.from(queryParameters ?? {}));
    if (pages.isNotEmpty) return pages.removeAt(0) as T;
    return {
          'executionScope': scope(),
          'snapshot': {
            'ownerId': 'device-a',
            'resources': [resource],
          },
        }
        as T;
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    editRequest = Map<String, dynamic>.from(data as Map);
    if (summaryMode && path.endsWith('/summary/generate')) {
      final requestId = editRequest!['requestId'];
      return {
            'saved': true,
            'summaryText': 'generated',
            'sourceResourceId': 'chat/summary',
            'sourceOwnerId': 'a',
            'sourceRevision': 1,
            'executionScope': {...scope(), 'requestId': requestId},
            'acknowledgement': {
              'ownerId': wrongAck ? 'another-owner' : 'a',
              'requestId': '$requestId|summary-result',
              'versions': {'summary/chat/summary': 1},
            },
          }
          as T;
    }
    if (path.endsWith('/projections/rebuild') && projectionResponse != null)
      return projectionResponse as T;
    if (editAck != null) return editAck as T;
    return {
          'ownerId': wrongAck ? 'other-device' : 'device-a',
          'versions': {'message/message': 8},
        }
        as T;
  }

  @override
  Future<Stream<List<int>>> postStream(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    CancelToken? cancelToken,
  }) async {
    streamRequest = Map<String, dynamic>.from(data as Map);
    return bytes(streamEvents);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Map<String, dynamic> scope() => {
  'spaceId': 'core-b',
  'initiatorDeviceId': 'a',
  'targetDeviceId': 'a',
  'coreId': 'core-b',
  'providerEpoch': 1,
  'targetProviderEpoch': 1,
  'coordinated': false,
  'modeRevision': 1,
  'permissionRevision': 1,
  'targetPermissionRevision': 1,
  'roleId': 'role-a',
  'roleRevision': 1,
  'authorizationRealm': 'core-b',
  'turnId': 'turn',
  'executionId': 'execution',
  'roleOwnerId': 'a',
  'resourceOwnerId': 'a',
  'requestId': 'request',
};

Stream<List<int>> bytes(List<Map<String, dynamic>> events) {
  final encoded = utf8.encode(
    events
        .map((event) => 'event: message\ndata: ${jsonEncode(event)}\n\n')
        .join(),
  );
  return Stream.fromIterable(encoded.map((byte) => [byte]));
}

void main() {
  test('手机从所有者原附件恢复文件和视频，不生成本地镜像文件', () {
    final service = DeviceOwnedChatService(_OwnedApi(),
      providerKey: () => 'https://provider');
    for (final kind in ['file', 'video']) {
      final uri = 'data:${kind == 'file' ? 'text/plain' : 'video/mp4'};base64,AQIDBA==';
      final attachment = ownedFileAttachment(uri,
        name: kind == 'file' ? 'notes.txt' : 'clip.mp4', kind: kind);
      final rows = service.messages({
        'executionScope': scope(),
        'snapshot': {'ownerId': 'a', 'resources': [
          {'kind': 'message', 'id': 'message', 'revision': 3,
            'body': {'id': 'message', 'conversationId': 'chat',
              'role': 'user', 'content': '附件', 'attachments': [attachment]}}
        ]},
      });
      expect(rows, hasLength(1));
      expect(rows.single.msgType, kind);
      expect(rows.single.resourceUri, uri);
      expect(rows.single.fileSizeBytes, 4);
      expect(rows.single.fileName, attachment['name']);
      expect(rows.single.sourceOwnerId, 'a');
      expect(rows.single.sourceRevision, 3);
      expect(rows.single.sourceConversationId, 'chat');
    }
  });
  test('手机发送引用冻结原权限且保留实际Owner会话编号', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    api.streamEvents = [
      {
        'type': 'started',
        'executionScope': scope(),
        'conversationId': 'continuation/hash',
      },
      {
        'type': 'completed',
        'data': {
          'requestId': 'request',
          'saved': true,
          'executionScope': scope(),
          'conversationId': 'continuation/hash',
        },
      },
    ];
    final quote = {
      'ownerId': 'a',
      'characterId': 'role-a',
      'conversationId': 'same',
      'messageId': 'original',
      'expectedRevision': 0,
      'contentHash': 'a' * 64,
      'expectedExecutionScope': scope(),
    };
    final events = await service
        .send(
          requestId: 'request',
          message: '回应引用',
          conversationId: 'meshconv1:a:same',
          quote: quote,
        )
        .toList();
    expect(api.streamRequest?['quote'], quote);
    expect(api.streamRequest?['expectedExecutionScope'], scope());
    expect(events.first['sourceConversationId'], 'continuation/hash');
    expect(events.last['data']['sourceConversationId'], 'continuation/hash');
  });
  test('旧引用权限不得刷新成新权限继续发送', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    await expectLater(
      service
          .send(
            requestId: 'request',
            message: '旧引用',
            quote: {
              'expectedExecutionScope': {...scope(), 'permissionRevision': 3},
            },
          )
          .toList(),
      throwsStateError,
    );
    expect(api.streamRequest, isNull);
  });
  test('手机明确导出时只读取选定所有者会话，不调用旧导出写入接口', () async {
    final api = _OwnedApi();
    final chat = ChatService(api, providerKey: () => 'https://provider');
    api.pages.add({
      'executionScope': scope(),
      'snapshot': {
        'ownerId': 'a',
        'resources': [
          {
            'kind': 'message',
            'id': 'message',
            'revision': 1,
            'body': {
              'id': 'message',
              'role': 'user',
              'content': 'exported',
              'createdAt': 'now',
            },
          },
        ],
      },
    });
    final value =
        jsonDecode(
              await chat.exportConversation('meshconv1:a:chat', format: 'json'),
            )
            as Map;
    expect(value['conversationId'], 'meshconv1:a:chat');
    expect(value['messages'], [
      {
        'id': 'message',
        'ownerId': 'a',
        'role': 'user',
        'content': 'exported',
        'createdAt': 'now',
      },
    ]);
    expect(api.editRequest, isNull);
    expect(api.queryPath, '/api/device-mesh/v1/business/conversations/chat');
  });
  test('手机归档详情通过所有者接口读取完整历史', () async {
    final api = _OwnedApi();
    final chat = ChatService(api, providerKey: () => 'https://provider');
    api.pages.add({
      'executionScope': scope(),
      'snapshot': {
        'ownerId': 'a',
        'resources': [
          {
            'kind': 'message',
            'id': 'message',
            'revision': 1,
            'body': {
              'id': 'message',
              'role': 'user',
              'content': 'owned',
              'createdAt': 'now',
            },
          },
        ],
      },
    });
    final rows = await chat.getMessages(
      'meshconv1:a:chat',
      characterId: 'role-a',
    );
    expect(rows.single.content, 'owned');
    expect(rows.single.conversationId, 'meshconv1:a:chat');
    expect(api.queryPath, '/api/device-mesh/v1/business/conversations/chat');
  });
  test('手机摘要编辑使用展示时的所有者版本和真实资源编号', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    final summary = {
      'kind': 'summary',
      'id': 'continuation/actual/summary',
      'ownerId': 'a',
      'roleId': 'role-a',
      'revision': 7,
      'body': {
        'content': {'summary': 'shown'},
      },
    };
    api.pages.add({
      'executionScope': scope(),
      'snapshot': {
        'ownerId': 'a',
        'resources': [summary],
      },
    });
    final displayed = await service.conversationSummary('meshconv1:a:chat');
    expect(displayed?['summaryText'], 'shown');
    expect(displayed?['editable'], isTrue);
    api.pages.add({
      'executionScope': scope(),
      'snapshot': {
        'ownerId': 'a',
        'resources': [
          {...summary, 'revision': 9},
        ],
      },
    });
    await service.query('meshconv1:a:chat');
    api.editAck = {
      'ownerId': 'a',
      'versions': {'summary/continuation/actual/summary': 8},
    };
    await service.editConversationSummary(
      'meshconv1:a:chat',
      text: 'edited',
      displayedViewId: displayed?['summaryViewId'] as String?,
    );
    expect(api.editRequest?['id'], 'continuation/actual/summary');
    expect(api.editRequest?['expectedRevision'], 7);
    expect(api.editRequest?['expectedExecutionScope'], scope());
    await expectLater(
      service.editConversationSummary('meshconv1:a:chat', text: 'again'),
      throwsStateError,
    );
  });
  test('手机拒绝摘要刷新后旧编辑页面的提交', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    Map<String, dynamic> page(int version) => {
      'executionScope': scope(),
      'snapshot': {
        'ownerId': 'a',
        'resources': [
          {
            'kind': 'summary',
            'id': 'chat/summary',
            'ownerId': 'a',
            'revision': version,
            'body': {
              'content': {'summary': 'version $version'},
            },
          },
        ],
      },
    };
    api.pages.add(page(1));
    final previous = await service.conversationSummary('meshconv1:a:chat');
    api.pages.add(page(2));
    await service.conversationSummary('meshconv1:a:chat');
    await expectLater(
      service.editConversationSummary(
        'meshconv1:a:chat',
        text: 'old dialog',
        displayedViewId: previous?['summaryViewId'] as String?,
      ),
      throwsStateError,
    );
    expect(api.editRequest, isNull);
  });
  test('手机摘要生成调用 Core 并校验所有者保存确认', () async {
    final api = _OwnedApi()..summaryMode = true;
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    final result = await service.generateConversationSummary(
      'meshconv1:a:chat',
    );
    expect(result['summaryText'], 'generated');
    expect(result['summaryViewId'], isNotEmpty);
    expect(api.editRequest?['expectedRevision'], 0);
    expect(api.editRequest?['expectedExecutionScope'], scope());
    expect(api.editRequest?['conversationOrigin'], {
      'ownerId': 'a',
      'id': 'chat',
    });
    api.wrongAck = true;
    await expectLater(
      service.generateConversationSummary('meshconv1:a:chat'),
      throwsStateError,
    );
  });
  test('手机历史摘要可查看但禁止修改其他来源', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    api.pages.add({
      'executionScope': scope(),
      'snapshot': {'ownerId': 'a', 'resources': []},
      'historicalSnapshot': {
        'ownerId': 'old',
        'resources': [
          {
            'kind': 'summary',
            'id': 'chat/summary',
            'ownerId': 'old',
            'revision': 3,
            'body': {
              'content': {'summary': 'history'},
            },
          },
        ],
      },
    });
    expect(
      (await service.conversationSummary('meshconv1:a:chat'))?['editable'],
      isFalse,
    );
    await expectLater(
      service.editConversationSummary('meshconv1:a:chat', deleted: true),
      throwsStateError,
    );
    expect(api.editRequest, isNull);
  });
  test('手机完整历史分别读取所有者页面，保留同名来源消息', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    api.pages.addAll([
      {
        'executionScope': scope(),
        'snapshot': {
          'ownerId': 'a',
          'nextCursors': {'message': 'next'},
          'resources': [
            {
              'kind': 'message',
              'id': 'same',
              'revision': 1,
              'body': {
                'id': 'same',
                'conversationId': 'chat',
                'role': 'user',
                'content': 'current',
                'createdAt': '2026-10-04T00:00:00Z',
              },
            },
          ],
        },
        'historicalSnapshot': {
          'ownerId': 'old-device',
          'legacyMessages': [
            {
              'id': 'same',
              'conversationId': 'chat',
              'role': 'user',
              'content': 'old',
              'createdAt': '2026-10-03T00:00:00Z',
            },
          ],
          'resources': [],
        },
      },
      {
        'executionScope': scope(),
        'snapshot': {
          'ownerId': 'a',
          'resources': [
            {
              'kind': 'message',
              'id': 'second',
              'revision': 1,
              'body': {
                'id': 'second',
                'conversationId': 'chat',
                'role': 'assistant',
                'content': 'later',
                'createdAt': '2026-10-04T00:00:01Z',
              },
            },
          ],
        },
      },
    ]);
    final rows = await service.allMessages('meshconv1:a:chat');
    expect(rows.map((row) => row.content), ['old', 'current', 'later']);
    expect(
      rows.every((row) => row.conversationId == 'meshconv1:a:chat'),
      isTrue,
    );
    expect(api.queries.last['cursor'], 'next');
  });
  test('手机同名会话按所有者隔离，只合并明确关联的接续会话', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    api.pages.add({
      'executionScope': {...scope(), 'resourceOwnerId': 'core-b'},
      'historicalConversations': [
        {'id': 'same', 'ownerId': 'a', 'title': 'old'},
      ],
      'snapshot': {
        'ownerId': 'core-b',
        'resources': [
          {
            'kind': 'conversation',
            'id': 'same',
            'body': {'id': 'same', 'title': 'independent'},
          },
          {
            'kind': 'conversation',
            'id': 'continuation/hash',
            'body': {
              'id': 'continuation/hash',
              'title': 'continued',
              'conversationOrigin': {'ownerId': 'a', 'id': 'same'},
            },
          },
        ],
      },
    });
    final rows = await service.conversations();
    expect(rows.length, 2);
    expect(
      rows.singleWhere((row) => row.id == 'meshconv1:a:same').title,
      'continued',
    );
    expect(
      rows.singleWhere((row) => row.id == 'meshconv1:core-b:same').title,
      'independent',
    );
    final origin = {'ownerId': '设备:a', 'id': '对话/id:1'};
    expect(parseConversationReference(conversationReference(origin)), origin);
  });

  test('手机查询与发送明确来源，确认结果保留相同页面会话标识', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    await service.query('meshconv1:a:same');
    expect(api.queryPath, '/api/device-mesh/v1/business/conversations/same');
    expect(api.queries.last['conversationOwnerId'], 'a');
    api.streamEvents = [
      {
        'type': 'started',
        'executionScope': scope(),
        'conversationId': 'continuation/hash',
      },
      {
        'type': 'completed',
        'data': {
          'requestId': 'request',
          'saved': true,
          'executionScope': scope(),
          'conversationId': 'continuation/hash',
          'conversationOrigin': {'ownerId': 'a', 'id': 'same'},
        },
      },
    ];
    final events = await service
        .send(
          requestId: 'request',
          message: 'hello',
          conversationId: 'meshconv1:a:same',
          context: {
            'previousCoreId': 'former-core',
            'conversationId': 'meshconv1:a:same',
            'messages': [],
          },
        )
        .toList();
    expect(api.streamRequest?['conversationId'], 'same');
    expect(api.streamRequest?['conversationOrigin'], {
      'ownerId': 'a',
      'id': 'same',
    });
    expect((api.streamRequest?['context'] as Map)['conversationId'], 'same');
    expect(events.first['conversationId'], 'meshconv1:a:same');
    expect(events.last['data']['conversationId'], 'meshconv1:a:same');
  });
  test('手机展示永久保存冲突的只读提示并隔离其他设备失败状态', () {
    final service = DeviceOwnedChatService(
      _OwnedApi(),
      providerKey: () => 'https://provider',
    );
    final rows = service.messages({
      'executionScope': scope(),
      'snapshot': {'ownerId': 'a', 'resources': []},
      'deliveryFailures': [
        {
          'ownerId': 'a',
          'requestId': 'rejected',
          'conversationId': 'chat',
          'errorCode': 'mesh.owned_resource_version',
          'failedAt': '2026-10-04T00:00:00Z',
        },
        {
          'ownerId': 'other',
          'requestId': 'hidden',
          'errorCode': 'mesh.owned_resource_version',
          'failedAt': '2026-10-04T00:00:00Z',
        },
      ],
    });
    expect(rows, hasLength(1));
    expect(rows.first.role, 'system');
    expect(rows.first.content, contains('有 1 项保存请求'));
    expect(rows.first.sourceRevision, isNull);
  });
  test('手机语音转写事件必须沿用已确认范围，重复与迟到转写拒绝', () async {
    final events = await decodeOwnedChatStream(
      bytes([
        {'type': 'started', 'executionScope': scope()},
        {'type': 'transcribed', 'executionScope': scope(), 'text': '喝茶'},
        {
          'type': 'completed',
          'data': {
            'requestId': 'request',
            'executionScope': scope(),
            'saved': true,
            'transcription': '喝茶',
            'userRevision': 2,
          },
        },
      ]),
      'request',
    ).toList();
    expect(events[1]['text'], '喝茶');
    await expectLater(
      decodeOwnedChatStream(
        bytes([
          {'type': 'started', 'executionScope': scope()},
          {
            'type': 'transcribed',
            'executionScope': {...scope(), 'providerEpoch': 2},
            'text': 'late',
          },
        ]),
        'request',
      ).toList(),
      throwsStateError,
    );
    await expectLater(
      decodeOwnedChatStream(
        bytes([
          {'type': 'started', 'executionScope': scope()},
          {'type': 'transcribed', 'executionScope': scope(), 'text': 'first'},
          {'type': 'transcribed', 'executionScope': scope(), 'text': 'second'},
        ]),
        'request',
      ).toList(),
      throwsStateError,
    );
  });
  test('手机转写完成必须匹配保存文字及版本，缓存结果无需重复转写事件', () async {
    final result = {
      'requestId': 'request',
      'executionScope': scope(),
      'saved': true,
      'transcription': '喝茶',
      'userRevision': 2,
    };
    for (final change in [
      {'transcription': 'changed'},
      {'userRevision': 1},
      {'transcription': null},
      {'transcription': ' '},
    ]) {
      await expectLater(
        decodeOwnedChatStream(
          bytes([
            {'type': 'started', 'executionScope': scope()},
            {'type': 'transcribed', 'executionScope': scope(), 'text': '喝茶'},
            {
              'type': 'completed',
              'data': {...result, ...change},
            },
          ]),
          'request',
        ).toList(),
        throwsStateError,
      );
    }
    final cached = await decodeOwnedChatStream(
      bytes([
        {'type': 'completed', 'data': result},
      ]),
      'request',
    ).toList();
    expect(cached.single['data']['transcription'], '喝茶');
  });
  test('手机索引状态校验所有者，重建携带原始权限快照', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    final expected = scope();
    api.projectionResponse = {
      'executionScope': expected,
      'status': {
        'ownerId': expected['resourceOwnerId'],
        'roleId': 'role-a',
        'layers': [],
      },
    };
    await service.projections('role-a');
    await service.projections('role-a', expectedScope: expected);
    expect(api.editRequest?['expectedExecutionScope'], expected);
    api.projectionResponse!['status']['ownerId'] = 'another-device';
    await expectLater(service.projections('role-a'), throwsStateError);
  });
  test('手机消息保留数据所有者、权限快照与原始版本，历史消息不可原地写入', () {
    final service = DeviceOwnedChatService(
      _OwnedApi(),
      providerKey: () => 'https://provider',
    );
    final originalScope = scope();
    final messages = service.messages({
      'executionScope': originalScope,
      'historicalSnapshot': {
        'ownerId': 'old-device',
        'legacyMessages': [
          {
            'id': 'legacy',
            'role': 'user',
            'content': '历史',
            'createdAt': '2026-10-01T00:00:00Z',
          },
        ],
      },
      'snapshot': {
        'ownerId': 'device-a',
        'resources': [
          {
            'kind': 'message',
            'revision': 7,
            'body': {
              'id': 'message',
              'role': 'assistant',
              'content': '当前',
              'createdAt': '2026-10-02T00:00:00Z',
            },
          },
        ],
      },
    });
    expect(messages.first.sourceOwnerId, 'old-device');
    expect(messages.first.sourceRevision, isNull);
    expect(messages.last.sourceOwnerId, 'device-a');
    expect(messages.last.sourceRevision, 7);
    originalScope['permissionRevision'] = 99;
    expect(messages.last.sourceScope?['permissionRevision'], 1);
  });

  test('编辑手机消息必须提交页面读取时的版本，不能用刷新后的版本覆盖', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    await service.query('conversation');
    final pageScope = {...scope(), 'permissionRevision': 1};
    api.resource['revision'] = 9;
    await service.query('conversation');
    await service.edit(
      'message',
      'message',
      changes: {'content': 'edit'},
      expectedOwnerId: 'device-a',
      expectedScope: pageScope,
      expectedRevision: 7,
    );
    expect(api.editRequest?['expectedRevision'], 7);
    expect(api.editRequest?['expectedExecutionScope'], pageScope);
    api.editRequest = null;
    await expectLater(
      service.edit(
        'message',
        'message',
        changes: {'content': 'edit'},
        expectedOwnerId: 'old-device',
        expectedScope: pageScope,
        expectedRevision: 7,
      ),
      throwsStateError,
    );
    expect(api.editRequest, isNull);
  });
  TestWidgetsFlutterBinding.ensureInitialized();
  setUp(() => SharedPreferences.setMockInitialValues({}));

  test('Core 与历史设备分别分页，已结束的数据源不会重新开启', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    api.pages.addAll([
      {
        'executionScope': scope(),
        'snapshot': {
          'ownerId': 'a',
          'resources': [],
          'nextCursors': {'message': 'core-1', 'legacyMessage': 'local-1'},
        },
        'historicalSnapshot': {
          'ownerId': 'old-device',
          'resources': [],
          'nextCursors': {'message': 'old-1', 'legacyMessage': 'old-local-1'},
        },
      },
      {
        'executionScope': scope(),
        'snapshot': {
          'ownerId': 'a',
          'resources': [],
          'nextCursors': {'legacyMessage': 'local-2'},
        },
        'historicalSnapshot': {'ownerId': 'old-device', 'resources': []},
      },
      {
        'executionScope': scope(),
        'snapshot': {
          'ownerId': 'a',
          'resources': [],
          'nextCursors': {'message': 'unrequested-duplicate'},
        },
      },
    ]);
    await service.query('conversation');
    expect(service.hasMore('conversation'), true);
    await service.query('conversation', older: true);
    expect(api.queries.last, containsPair('cursor', 'core-1'));
    expect(api.queries.last, containsPair('historicalCursor', 'old-1'));
    expect(
      api.queries.last,
      containsPair('historicalLegacyCursor', 'old-local-1'),
    );
    await service.query('conversation', older: true);
    expect(api.queries.last, {
      'characterId': 'role-a',
      'resourceKind': 'message',
      'legacyCursor': 'local-2',
    });
    expect(service.hasMore('conversation'), false);
  });

  test('分页期间角色版本变化，拒绝混入历史', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    api.pages.addAll([
      {
        'executionScope': scope(),
        'snapshot': {
          'ownerId': 'a',
          'resources': [],
          'nextCursors': {'message': 'cursor'},
        },
      },
      {
        'executionScope': {...scope(), 'roleRevision': 2},
        'snapshot': {'ownerId': 'a', 'resources': []},
      },
    ]);
    await service.query('conversation');
    await expectLater(
      service.query('conversation', older: true),
      throwsStateError,
    );
  });

  test('搜索关键词传递到每一页且保留正文匹配的普通标题', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    api.pages.addAll([
      {
        'executionScope': scope(),
        'snapshot': {
          'ownerId': 'a',
          'resources': [
            {
              'kind': 'conversation',
              'body': {'id': 'first', 'title': '普通标题'},
            },
          ],
          'nextCursors': {'conversation': 'search-page'},
        },
      },
      {
        'executionScope': scope(),
        'snapshot': {
          'ownerId': 'a',
          'resources': [
            {
              'kind': 'conversation',
              'body': {'id': 'second', 'title': '普通标题'},
            },
          ],
        },
      },
    ]);
    final rows = await service.conversations(keyword: ' NEEDLE ');
    expect(rows.length, 2);
    expect(api.queries.first, {'characterId': 'role-a', 'keyword': 'needle'});
    expect(api.queries.last, {
      'characterId': 'role-a',
      'keyword': 'needle',
      'resourceKind': 'conversation',
      'cursor': 'search-page',
    });
    await service.conversations(keyword: 'different');
    expect(api.queries.last, {'characterId': 'role-a', 'keyword': 'different'});
  });

  test('会话列表分别读完 Core 和原设备历史分页', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    api.pages.addAll([
      {
        'executionScope': scope(),
        'snapshot': {'ownerId': 'a', 'resources': []},
        'historicalConversations': [
          {'id': 'old-first', 'title': 'first'},
        ],
        'nextHistoricalListCursor': 'device-page-two',
      },
      {
        'executionScope': scope(),
        'snapshot': {'ownerId': 'a', 'resources': []},
        'historicalConversations': [
          {'id': 'old-second', 'title': 'second'},
        ],
      },
    ]);
    final rows = await service.conversations();
    expect(rows.map((row) => row.id).toSet(), {
      'meshconv1:a:old-first',
      'meshconv1:a:old-second',
    });
    expect(api.queries.last, {
      'characterId': 'role-a',
      'resourceKind': 'conversation',
      'historicalListCursor': 'device-page-two',
    });
  });

  test('会话列表重复游标时停止并提示错误', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    for (var i = 0; i < 2; i++) {
      api.pages.add({
        'executionScope': scope(),
        'snapshot': {
          'ownerId': 'a',
          'resources': [],
          'nextCursors': {'conversation': 'repeated'},
        },
      });
    }
    await expectLater(service.conversations(), throwsStateError);
    expect(api.queries.length, 2);
  });

  test('等待新 Core 批准时中断并持久保留切换提示', () async {
    final api = _OwnedApi();
    Map<String, dynamic>? transition;
    final service = DeviceOwnedChatService(
      api,
      providerTransition: () async => transition,
    );
    await service.refresh();
    final previousRevision = service.revision;
    transition = {
      'providerChangePending': true,
      'coreId': 'core-b',
      'successorCoreId': 'core-c',
    };
    await expectLater(service.refresh(), throwsStateError);
    expect(service.revision, greaterThan(previousRevision));
    expect(service.notice, contains('core-b'));
    expect(service.notice, contains('core-c'));
    expect(service.notice, contains('批准'));
  });

  test('原设备记忆游标独立传递，已完成的新存储不会重新分页', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://core-b',
    );
    await service.refresh();
    await service.data(
      'memory',
      characterId: 'role-a',
      legacyCursor: 'original-device-page',
    );
    expect(api.queries.last['cursor'], '');
    expect(api.queries.last['legacyCursor'], 'original-device-page');
    expect(api.queries.last['historicalLegacyCursor'], '');
  });

  test('编辑使用实际归属及版本，错误保存确认不允许成功', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://core-b',
    );
    await service.refresh();
    await service.query('conversation');
    await service.edit('message', 'message', changes: {'content': 'new'});
    expect(api.editRequest!['expectedRevision'], 7);
    expect(api.editRequest!['characterId'], 'role-a');
    expect(api.editRequest!['changes'], {'content': 'new'});
    api.wrongAck = true;
    expect(
      () => service.edit('message', 'message', deleted: true),
      throwsStateError,
    );
  });

  test('提供者切换提示保留至服务实例重新创建', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    api.core = 'core-c';
    await service.refresh();
    expect(service.notice, contains('core-b'));
    expect(service.notice, contains('core-c'));
    await Future<void>.delayed(Duration.zero);
    final restored = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await restored.refresh();
    expect(restored.coreId, 'core-c');
    expect(restored.notice, contains('core-b'));
  });

  test('没有前端历史时从绑定状态恢复切换提示且不重复中断', () async {
    final api = _OwnedApi()..core = 'core-c';
    final transition = {
      'coreId': 'core-c',
      'previousCoreId': 'core-b',
      'providerChangeId': 'cold-start-switch',
    };
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
      providerTransition: () async => transition,
    );
    await service.refresh();
    expect(service.notice, contains('已从「core-b」切换为「core-c」'));
    final revision = service.revision;
    await service.refresh();
    expect(service.revision, revision);
    await Future<void>.delayed(Duration.zero);
    final restored = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
      providerTransition: () async => transition,
    );
    await restored.refresh();
    expect(restored.notice, contains('core-b'));
    expect(restored.revision, 0);
  });

  test('数据查询等待期间切换服务，迟到历史被丢弃', () async {
    final api = _OwnedApi();
    final service = DeviceOwnedChatService(
      api,
      providerKey: () => 'https://provider',
    );
    await service.refresh();
    api.deferred = Completer<Map<String, dynamic>>();
    final query = service.query('conversation');
    final assertion = expectLater(query, throwsStateError);
    service.stopLocal('provider changed');
    api.deferred!.complete({
      'snapshot': {'ownerId': 'device-a', 'resources': []},
    });
    await assertion;
  });
  test('按 UTF-8 字节分块接收，保存确认后才完成', () async {
    final events = await decodeOwnedChatStream(
      bytes([
        {
          'type': 'started',
          'executionScope': scope(),
          'conversationId': 'conversation',
        },
        {'type': 'delta', 'executionScope': scope(), 'text': '你好'},
        {
          'type': 'completed',
          'data': {
            'saved': true,
            'requestId': 'request',
            'executionScope': scope(),
            'reply': '你好',
          },
        },
      ]),
      'request',
    ).toList();
    expect(events[1]['text'], '你好');
    expect(events.last['type'], 'completed');
  });

  test('拦截来自新提供者的混入增量', () async {
    expect(
      () => decodeOwnedChatStream(
        bytes([
          {'type': 'started', 'executionScope': scope()},
          {
            'type': 'delta',
            'executionScope': {...scope(), 'coreId': 'core-c'},
            'text': '迟到',
          },
        ]),
        'request',
      ).toList(),
      throwsStateError,
    );
  });

  test('连接截断不能当作回复成功', () async {
    expect(
      () => decodeOwnedChatStream(
        bytes([
          {'type': 'started', 'executionScope': scope()},
          {'type': 'delta', 'executionScope': scope(), 'text': '部分'},
        ]),
        'request',
      ).toList(),
      throwsStateError,
    );
  });

  test('未确认保存和结束后额外事件均被拒绝', () async {
    for (final saved in [false, true]) {
      expect(
        () => decodeOwnedChatStream(
          bytes([
            {'type': 'started', 'executionScope': scope()},
            {
              'type': 'completed',
              'data': {
                'saved': saved,
                'requestId': 'request',
                'executionScope': scope(),
              },
            },
            if (saved)
              {'type': 'delta', 'executionScope': scope(), 'text': '额外'},
          ]),
          'request',
        ).toList(),
        throwsStateError,
      );
    }
  });
}
