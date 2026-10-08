import 'dart:convert';
import 'package:dio/dio.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/chat_service.dart';
import 'package:amitia_app/core/services/device_owned_chat_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:amitia_app/features/chat/runtime/conversation_runtime_controller.dart';
import 'package:amitia_app/shared/models/models.dart';

class _ProjectApi implements BackendServiceApi {
  String core = 'b';
  bool coordinated = false;
  bool available = true;
  bool badAck = false;
  bool failMove = false;
  bool wrongEditRequest = false;
  final requests = <Map<String, dynamic>>[];
  final paths = <String>[];
  final projectPages = <Map<String, dynamic>>[];
  final projectQueries = <Map<String, dynamic>>[];
  Map<String, dynamic> scope() => {
    'spaceId': 'space',
    'initiatorDeviceId': 'a',
    'targetDeviceId': 'a',
    'coreId': core,
    'providerEpoch': 1,
    'targetProviderEpoch': 1,
    'coordinated': coordinated,
    'modeRevision': 1,
    'permissionRevision': 1,
    'targetPermissionRevision': 1,
    'roleId': 'role',
    'roleRevision': 1,
    'authorizationRealm': 'space',
    'roleOwnerId': 'a',
    'resourceOwnerId': coordinated ? core : 'a',
    'requestId': 'read',
    'turnId': 'turn',
    'executionId': 'execution',
  };
  Map<String, dynamic> row(String owner, {bool readOnly = false}) => {
    'id': 'same',
    'title': owner,
    'ownerId': owner,
    'roleId': 'role',
    'revision': 3,
    'readOnly': readOnly,
  };
  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    paths.add(path);
    if (path.endsWith('/projects')) {
      projectQueries.add(Map<String, dynamic>.from(queryParameters ?? {}));
      if (projectPages.isNotEmpty) return projectPages.removeAt(0) as T;
    }
    if (path.endsWith('/coordination/me'))
      return {
            'coreId': core,
            'coordinationAvailable': available,
            'policy': {
              'selectedRole': 'role',
              'coordinated': coordinated,
              'providerEpoch': 1,
              'modeRevision': 1,
              'permissionRevision': 1,
            },
          }
          as T;
    if (path.endsWith('/historical-roles'))
      return {
            'executionScope': scope(),
            'roles': [
              {'id': 'role'},
            ],
          }
          as T;
    if (path.endsWith('/roles'))
      return {
            'roles': [
              {'id': 'role', 'revision': 1},
            ],
          }
          as T;
    if (path.endsWith('/projects'))
      return {
            'executionScope': scope(),
            'projects': [row(coordinated ? core : 'a')],
            'historicalProjects': coordinated ? [row('a', readOnly: true)] : [],
            'nextCursor': '',
            'nextHistoricalCursor': '',
          }
          as T;
    if (path.endsWith('/data'))
      return {
            'executionScope': scope(),
            'snapshot': {'resources': []},
          }
          as T;
    if (path.contains('/business/conversations'))
      return {
            'executionScope': scope(),
            'conversationId': 'chat',
            'snapshot': {
              'ownerId': coordinated ? core : 'a',
              'resources': [
                {
                  'kind': 'conversation',
                  'id': 'chat',
                  'ownerId': coordinated ? core : 'a',
                  'roleId': 'role',
                  'revision': 1,
                  'body': {'id': 'chat', 'characterId': 'role'},
                },
              ],
            },
          }
          as T;
    throw StateError('非预期接口 $path');
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    paths.add(path);
    final request = Map<String, dynamic>.from(data as Map);
    requests.add(request);
    final owner = coordinated ? core : 'a';
    if (request['kind'] == 'conversation') {
      if (failMove) throw StateError('归组拒绝');
      return {
            'ownerId': owner,
            'requestId': request['requestId'],
            'versions': {'conversation/chat': 2},
          }
          as T;
    }
    if (path.endsWith('/projects'))
      return {
            'saved': true,
            'project': {'id': 'new', 'title': request['title']},
            'executionScope': {...scope(), 'requestId': request['requestId']},
            'acknowledgement': {
              'requestId': request['requestId'],
              'ownerId': badAck ? 'foreign' : owner,
              'versions': {
                'project/new': 1,
                'checkpoint/project/${request['requestId']}': 1,
              },
            },
          }
          as T;
    return {
          'ownerId': owner,
          'requestId': wrongEditRequest
              ? 'other-request'
              : request['requestId'],
          'versions': {'project/${request['id']}': 4},
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
    final request = Map<String, dynamic>.from(data as Map);
    final current = {...scope(), 'requestId': request['requestId']};
    final frames = [
      {'type': 'started', 'conversationId': 'chat', 'executionScope': current},
      {
        'type': 'completed',
        'data': {
          'requestId': request['requestId'],
          'conversationId': 'chat',
          'executionScope': current,
          'saved': true,
          'reply': '已保存回复',
        },
      },
    ];
    return Stream.value(
      utf8.encode(
        frames.map((frame) => 'data: ${jsonEncode(frame)}\n\n').join(),
      ),
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));
  test('同 ID 的 Core 项目与 Source 历史项目保留各自归属且历史不可编辑', () async {
    final api = _ProjectApi()..coordinated = true;
    final service = DeviceOwnedChatService(api, providerKey: () => 'cloud');
    await service.refresh();
    final rows = await service.projects();
    expect(rows.map((row) => row.ownerId), ['b', 'a']);
    expect(rows.last.readOnly, isTrue);
    await expectLater(
      service.editProject(rows.last, changes: {'title': 'changed'}),
      throwsStateError,
    );
    expect(api.requests, isEmpty);
  });
  test('项目修改提交展示时的 owner revision scope，Core 切换后拒绝旧记录', () async {
    final api = _ProjectApi();
    final service = DeviceOwnedChatService(api, providerKey: () => 'cloud');
    await service.refresh();
    final project = (await service.projects()).single;
    await service.editProject(project, changes: {'pinned': true});
    expect(api.requests.single['expectedRevision'], 3);
    expect(api.requests.single['expectedExecutionScope']['coreId'], 'b');
    api.core = 'c';
    await service.refresh();
    await service.projects();
    await expectLater(
      service.editProject(project, changes: {'title': 'old'}),
      throwsStateError,
    );
    expect(api.requests.length, 1);
  });
  test('项目创建验证所有者 ACK，旧草稿 Core 切换不能提交', () async {
    final api = _ProjectApi();
    final service = DeviceOwnedChatService(api, providerKey: () => 'cloud');
    await service.refresh();
    final original = api.scope();
    final project = await service.createProject('new', expectedScope: original);
    expect(project.ownerId, 'a');
    api.badAck = true;
    await expectLater(
      service.createProject('bad', expectedScope: original),
      throwsStateError,
    );
    api.core = 'c';
    await service.refresh();
    await expectLater(
      service.createProject('old', expectedScope: original),
      throwsStateError,
    );
    expect(api.requests.length, 2);
  });
  test('项目编辑不能接受其他请求的保存确认', () async {
    final api = _ProjectApi()..wrongEditRequest = true;
    final service = DeviceOwnedChatService(api, providerKey: () => 'cloud');
    await service.refresh();
    final project = (await service.projects()).single;
    await expectLater(
      service.editProject(project, changes: {'pinned': true}),
      throwsStateError,
    );
  });
  test('绑定项目禁止目录关联、打开及无原始归属的变更，不调用 legacy', () async {
    final api = _ProjectApi();
    final service = ChatService(api, providerKey: () => 'cloud');
    await expectLater(
      service.createProject(name: 'folder', workspaceId: 'mount'),
      throwsStateError,
    );
    await expectLater(service.projectLocation('same'), throwsStateError);
    await expectLater(
      service.updateProject('same', name: 'rename'),
      throwsStateError,
    );
    await expectLater(service.deleteProject('same'), throwsStateError);
    expect(api.paths.where((path) => path.contains('/web-chat/')), isEmpty);
    expect(api.requests, isEmpty);
  });
  test('绑定状态未就绪必须 fail closed，不回退 legacy 目录或聊天', () async {
    final api = _ProjectApi()..available = false;
    final service = ChatService(api, providerKey: () => 'cloud');
    await expectLater(service.conversationSidebar(), throwsStateError);
    await expectLater(service.createProject(name: 'new'), throwsStateError);
    expect(api.paths.where((path) => path.contains('/web-chat/')), isEmpty);
    expect(api.requests, isEmpty);
  });
  test('当前项目结束后历史继续分页，不能重新接入当前页游标', () async {
    final api = _ProjectApi()..coordinated = true;
    api.projectPages.addAll([
      {
        'executionScope': api.scope(),
        'projects': [api.row('b')],
        'historicalProjects': [api.row('a', readOnly: true)],
        'nextCursor': '',
        'nextHistoricalCursor': 'old-2',
      },
      {
        'executionScope': api.scope(),
        'projects': [
          {...api.row('b'), 'id': 'ignored'},
        ],
        'historicalProjects': [
          {...api.row('a', readOnly: true), 'id': 'older'},
        ],
        'nextCursor': 'must-ignore',
        'nextHistoricalCursor': '',
      },
    ]);
    final service = DeviceOwnedChatService(api, providerKey: () => 'cloud');
    await service.refresh();
    final rows = await service.projects();
    expect(rows.map((row) => row.id), ['same', 'same', 'older']);
    expect(api.projectQueries.last['historicalCursor'], 'old-2');
    expect(api.projectPages, isEmpty);
  });
  test('首条回复保存后才归组，归组失败保留已保存消息并提示', () async {
    for (final fail in [false, true]) {
      final api = _ProjectApi()..failMove = fail;
      final chat = ChatService(api, providerKey: () => 'cloud');
      await chat.owned.refresh();
      final project = (await chat.owned.projects()).single;
      final controller = ConversationRuntimeController(chat);
      controller.startDraft(
        workspace: ConversationWorkspaceDto(
          projectId: project.id,
          workspaceId: '',
          rootUri: '',
          workspaceKind: 'logical',
          logicalProject: project,
        ),
      );
      await controller.sendText('你好');
      expect(api.requests.single['changes']['projectId'], 'same');
      expect(api.requests.single['expectedExecutionScope']['coreId'], 'b');
      final answer = controller.messages.last;
      expect(answer.content, '已保存回复');
      expect(answer.status, MessageStatus.delivered);
      if (fail)
        expect(controller.lastError.toString(), contains('对话已保存，但项目归组未保存'));
      else
        expect(controller.lastError, isNull);
      controller.dispose();
    }
  });
}
