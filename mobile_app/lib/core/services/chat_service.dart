import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';

import '../backend_transport/backend_service_api.dart';
import '../models/conversation.dart';
import '../models/project.dart';
import 'device_owned_chat_service.dart';

class ChatSubmitResult {
  final String conversationId;
  final String userMessageId;
  final String requestId;
  final String turnId;
  final String executionId;
  final String status;

  const ChatSubmitResult({
    required this.conversationId,
    required this.userMessageId,
    required this.requestId,
    required this.turnId,
    required this.executionId,
    required this.status,
  });

  factory ChatSubmitResult.fromJson(Map<String, dynamic> json) {
    return ChatSubmitResult(
      conversationId: (json['conversationId'] ?? '').toString(),
      userMessageId: (json['userMessageId'] ?? '').toString(),
      requestId: (json['requestId'] ?? '').toString(),
      turnId: (json['turnId'] ?? '').toString(),
      executionId: (json['executionId'] ?? '').toString(),
      status: (json['status'] ?? '').toString(),
    );
  }
}

class ConversationSnapshotDto {
  final int version;
  final int revision;
  final int lastEventSequence;
  final ConversationDto? conversation;
  final ConversationWorkspaceDto? workspace;
  final List<MessageDto> messages;
  final List<AssistantTurnDto> turns;
  final int messageNextBefore;
  final bool hasMoreMessages;
  final int turnNextBefore;
  final String turnNextCursor;
  final bool hasMoreTurns;
  final Map<String, dynamic>? activeTurn;
  final List<Map<String, dynamic>> approvals;

  const ConversationSnapshotDto({
    required this.version,
    required this.revision,
    required this.lastEventSequence,
    required this.conversation,
    required this.workspace,
    required this.messages,
    required this.turns,
    required this.messageNextBefore,
    required this.hasMoreMessages,
    required this.turnNextBefore,
    this.turnNextCursor = '',
    required this.hasMoreTurns,
    required this.activeTurn,
    required this.approvals,
  });

  factory ConversationSnapshotDto.fromJson(Map<String, dynamic> json) {
    final rawConversation = json['conversation'];
    final rawWorkspace = json['workspace'];
    final rawMessages = json['messages'];
    final rawTurns = json['turns'];
    final rawMessageHistory = json['messageHistory'];
    final rawTurnHistory = json['turnHistory'];
    final messageHistory = rawMessageHistory is Map
        ? Map<String, dynamic>.from(rawMessageHistory)
        : const <String, dynamic>{};
    final turnHistory = rawTurnHistory is Map
        ? Map<String, dynamic>.from(rawTurnHistory)
        : const <String, dynamic>{};
    final rawActiveTurn = json['activeTurn'];
    final rawApprovals = json['approvals'];
    return ConversationSnapshotDto(
      version: (json['version'] as num?)?.toInt() ?? 0,
      revision: (json['revision'] as num?)?.toInt() ?? 0,
      lastEventSequence: (json['lastEventSequence'] as num?)?.toInt() ?? 0,
      conversation: rawConversation is Map
          ? ConversationDto.fromJson(Map<String, dynamic>.from(rawConversation))
          : null,
      workspace: rawWorkspace is Map
          ? ConversationWorkspaceDto.fromJson(
              Map<String, dynamic>.from(rawWorkspace),
            )
          : null,
      messages: rawMessages is List
          ? rawMessages
                .whereType<Map>()
                .map(
                  (item) =>
                      MessageDto.fromJson(Map<String, dynamic>.from(item)),
                )
                .toList(growable: false)
          : const <MessageDto>[],
      turns: rawTurns is List
          ? rawTurns
                .whereType<Map>()
                .map(
                  (item) => AssistantTurnDto.fromJson(
                    Map<String, dynamic>.from(item),
                  ),
                )
                .toList(growable: false)
          : const <AssistantTurnDto>[],
      messageNextBefore: (messageHistory['nextBefore'] as num?)?.toInt() ?? 0,
      hasMoreMessages: messageHistory['hasMore'] == true,
      turnNextBefore: (turnHistory['nextBefore'] as num?)?.toInt() ?? 0,
      turnNextCursor: (turnHistory['nextCursor'] ?? '').toString(),
      hasMoreTurns: turnHistory['hasMore'] == true,
      activeTurn: rawActiveTurn is Map
          ? Map<String, dynamic>.from(rawActiveTurn)
          : null,
      approvals: rawApprovals is List
          ? rawApprovals
                .whereType<Map>()
                .map((item) => Map<String, dynamic>.from(item))
                .toList(growable: false)
          : const <Map<String, dynamic>>[],
    );
  }
}

class AssistantTurnPageDto {
  final List<AssistantTurnDto> items;
  final int nextBefore;
  final String nextCursor;
  final bool hasMore;

  const AssistantTurnPageDto({
    required this.items,
    required this.nextBefore,
    this.nextCursor = '',
    required this.hasMore,
  });
}

class ChatStreamCancellation {
  final CancelToken _token = CancelToken();

  CancelToken get token => _token;
  bool get isCancelled => _token.isCancelled;

  void cancel([String reason = 'cancelled']) {
    if (!_token.isCancelled) _token.cancel(reason);
  }
}

class ChatStreamEvent {
  final String type;
  final Map<String, dynamic> data;

  const ChatStreamEvent(this.type, this.data);
}

class ConversationWorkspaceDto {
  final String conversationId;
  final String projectId;
  final String workspaceId;
  final String deviceId;
  final String workspaceName;
  final String workspaceKind;
  final String rootUri;
  final List<ProjectWorkspaceDto> workspaces;
  final String primaryWorkspaceId;

  const ConversationWorkspaceDto({
    this.conversationId = '',
    this.projectId = '',
    required this.workspaceId,
    this.deviceId = '',
    this.workspaceName = '',
    this.workspaceKind = 'local',
    required this.rootUri,
    this.workspaces = const <ProjectWorkspaceDto>[],
    this.primaryWorkspaceId = '',
  });

  factory ConversationWorkspaceDto.fromJson(Map<String, dynamic> json) {
    final workspaceId = (json['workspaceId'] ?? '').toString().trim();
    return ConversationWorkspaceDto(
      conversationId: (json['conversationId'] ?? '').toString().trim(),
      projectId: (json['projectId'] ?? '').toString().trim(),
      workspaceId: workspaceId,
      deviceId: (json['deviceId'] ?? '').toString().trim(),
      workspaceName: (json['workspaceName'] ?? '').toString().trim(),
      workspaceKind: (json['workspaceKind'] ?? 'local').toString().trim(),
      rootUri: (json['rootUri'] ?? 'amitia://workspace/@$workspaceId/')
          .toString()
          .trim(),
      workspaces: ((json['workspaces'] as List<dynamic>?) ?? const [])
          .whereType<Map>()
          .map(
            (item) =>
                ProjectWorkspaceDto.fromJson(Map<String, dynamic>.from(item)),
          )
          .toList(growable: false),
      primaryWorkspaceId: (json['primaryWorkspaceId'] ?? '').toString().trim(),
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'workspaceId': workspaceId,
    'projectId': projectId,
    'deviceId': deviceId,
    'workspaceName': workspaceName,
    'workspaceKind': workspaceKind,
    'rootUri': rootUri,
    if (workspaces.isNotEmpty)
      'workspaces': workspaces.map((workspace) => workspace.toJson()).toList(),
    if (primaryWorkspaceId.isNotEmpty) 'primaryWorkspaceId': primaryWorkspaceId,
  };
}

class ChatService {
  final BackendServiceApi _api;
  final DeviceOwnedChatService owned;

  ChatService(
    this._api, {
    DeviceOwnedChatService? owned,
    String Function()? providerKey,
    Future<Map<String, dynamic>?> Function()? providerTransition,
  }) : owned =
           owned ??
           DeviceOwnedChatService(
             _api,
             providerKey: providerKey,
             providerTransition: providerTransition,
           );

  Future<void> _requireOwned() async {
    if (!await owned.refresh())
      throw StateError(owned.notice.isEmpty ? '设备对话服务尚未就绪' : owned.notice);
  }

  Future<List<ConversationDto>> listConversations() async {
    await _requireOwned();
    return owned.conversations();
  }

  Future<List<ConversationDto>> archivedConversations({
    String projectId = '',
    String keyword = '',
  }) async {
    await _requireOwned();
    final rows = await owned.conversations(
      keyword: keyword.trim().toLowerCase(),
    );
    return rows
        .where(
          (row) =>
              row.archivedAt.isNotEmpty &&
              (projectId.isEmpty || row.projectId == projectId),
        )
        .toList();
  }

  Future<void> updateConversationModelSettings(
    String conversationId, {
    required int modelConfigId,
    required String reasoningEffort,
    required bool reasoningEnabled,
  }) async {
    await owned.edit(
      'conversation',
      conversationId,
      changes: <String, dynamic>{
        'modelConfigId': modelConfigId,
        'reasoningEffort': reasoningEffort,
        'reasoningEnabled': reasoningEnabled,
      },
    );
  }

  Future<void> updateConversationPermissionMode(
    String conversationId,
    String permissionMode,
  ) async {
    await owned.edit(
      'conversation',
      conversationId,
      changes: <String, dynamic>{'permissionMode': permissionMode},
    );
  }

  Future<ConversationSidebarDto> conversationSidebar() async {
    await _requireOwned();
    final rows = (await owned.conversations())
        .where((row) => row.archivedAt.isEmpty)
        .toList();
    final projects = (await owned.projects())
        .where((project) => !project.readOnly)
        .toList(growable: false);
    final grouped = projects
        .map(
          (project) => ProjectDto(
            id: project.id,
            name: project.name,
            workspaceId: project.workspaceId,
            deviceId: project.deviceId,
            rootUri: project.rootUri,
            workspaceKind: project.workspaceKind,
            pinnedAt: project.pinnedAt,
            ownerId: project.ownerId,
            roleId: project.roleId,
            revision: project.revision,
            readOnly: project.readOnly,
            workspaces: project.workspaces,
            primaryWorkspaceId: project.primaryWorkspaceId,
            hiddenAt: project.hiddenAt,
            executionScope: project.executionScope,
            conversations: rows
                .where(
                  (row) =>
                      row.projectId == project.id &&
                      row.sourceOwnerId == project.ownerId &&
                      row.characterId == project.roleId,
                )
                .toList(),
          ),
        )
        .toList();
    final groupedIds = grouped
        .expand((project) => project.conversations)
        .map((row) => row.id)
        .toSet();
    return ConversationSidebarDto(
      pinned: rows
          .where(
            (row) => row.pinnedAt.isNotEmpty && !groupedIds.contains(row.id),
          )
          .toList(),
      recent: rows
          .where((row) => row.pinnedAt.isEmpty && !groupedIds.contains(row.id))
          .toList(),
      projects: grouped.where((project) => project.hiddenAt.isEmpty).toList(),
      removedProjects: grouped
          .where((project) => project.hiddenAt.isNotEmpty)
          .toList(),
    );
  }

  Future<List<ConversationDto>> channelConversations() async {
    await _requireOwned();
    return (await owned.conversations())
        .where((row) => row.channel.isNotEmpty && row.channel != 'web')
        .toList();
  }

  Future<ProjectDto> createProject({
    required String name,
    String workspaceId = '',
    String deviceId = '',
    String rootUri = '',
    List<ProjectWorkspaceDto> workspaces = const <ProjectWorkspaceDto>[],
    String primaryWorkspaceId = '',
    Map<String, dynamic>? expectedScope,
  }) async {
    await _requireOwned();
    if (expectedScope == null) throw StateError('请先加载项目创建表单的数据归属');
    return owned.createProject(
      name,
      expectedScope: expectedScope,
      workspaces: workspaces,
      primaryWorkspaceId: primaryWorkspaceId,
    );
  }

  Future<void> hideProject(String projectId, {ProjectDto? project}) async {
    await _requireOwned();
    if (project == null || project.id != projectId)
      throw StateError('请重新加载项目归属后再移除');
    return owned.editProject(project, changes: const {'hidden': true});
  }

  Future<void> deleteProject(String projectId, {ProjectDto? project}) {
    return hideProject(projectId, project: project);
  }

  Future<void> updateProject(
    String projectId, {
    String? name,
    List<ProjectWorkspaceDto>? workspaces,
    String? primaryWorkspaceId,
    bool? pinned,
    bool? hidden,
    ProjectDto? project,
  }) async {
    await _requireOwned();
    if (project == null || project.id != projectId)
      throw StateError('请重新加载项目归属后再修改');
    return owned.editProject(
      project,
      changes: {
        if (name != null) 'title': name.trim(),
        if (pinned != null) 'pinned': pinned,
        if (hidden != null) 'hidden': hidden,
        if (workspaces != null)
          'workspaces': workspaces
              .map((workspace) => workspace.toJson())
              .toList(),
        if (primaryWorkspaceId != null)
          'primaryWorkspaceId': primaryWorkspaceId,
      },
    );
  }

  Future<Map<String, dynamic>> projectLocation(String projectId) async {
    await _requireOwned();
    final project = (await owned.projects())
        .where((item) => item.id == projectId)
        .firstOrNull;
    final primary = project?.workspaces
        .where((item) => item.id == project?.primaryWorkspaceId)
        .firstOrNull;
    final workspaceId = primary?.id ?? project?.workspaceId ?? '';
    if (workspaceId.isEmpty) throw StateError('项目没有可用工作区');
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/workspaces/${Uri.encodeComponent(workspaceId)}/location',
      fromJson: (e) => Map<String, dynamic>.from(e as Map),
    );
    return resp ?? <String, dynamic>{};
  }

  Future<void> moveConversationToProject(
    String conversationId,
    String projectId, {
    ProjectDto? project,
  }) async {
    throw StateError('现有对话不能手动移入或移出项目');
  }

  Future<List<MessageDto>> getMessages(
    String conversationId, {
    String? characterId,
    int beforeSequence = 0,
    int limit = 50,
  }) async {
    await _requireOwned();
    return owned.allMessages(conversationId, characterId: characterId);
  }

  Future<AssistantTurnPageDto> getAssistantTurnPage(
    String conversationId, {
    int before = 0,
    String cursor = '',
    int limit = 50,
  }) async {
    final resp = await owned.runtime(
      'turns',
      body: <String, dynamic>{
        'conversationId': conversationId,
        if (cursor.isNotEmpty) 'cursor': cursor,
        'limit': limit,
      },
    );
    final rows = resp['items'];
    final items = rows is List
        ? rows
              .whereType<Map>()
              .map(
                (row) => AssistantTurnDto.fromJson({
                  ...Map<String, dynamic>.from(row),
                  'conversationId': conversationId,
                }),
              )
              .toList(growable: false)
        : const <AssistantTurnDto>[];
    return AssistantTurnPageDto(
      items: items,
      nextBefore: (resp['nextBefore'] as num?)?.toInt() ?? 0,
      nextCursor: (resp['nextCursor'] ?? '').toString(),
      hasMore: resp['hasMore'] == true,
    );
  }

  Future<List<AssistantTurnDto>> getAssistantTurns(
    String conversationId, {
    int limit = 50,
  }) async {
    final page = await getAssistantTurnPage(conversationId, limit: limit);
    return page.items;
  }

  Future<ConversationSnapshotDto> conversationSnapshot(
    String conversationId,
  ) async {
    final resp = await owned.runtime(
      'snapshot',
      body: {'conversationId': conversationId},
    );
    return ConversationSnapshotDto.fromJson({
      ...resp,
      if (resp['conversation'] is Map)
        'conversation': {...resp['conversation'] as Map, 'id': conversationId},
      if (resp['turns'] is List)
        'turns': (resp['turns'] as List)
            .whereType<Map>()
            .map((row) => {...row, 'conversationId': conversationId})
            .toList(),
    });
  }

  Stream<ChatStreamEvent> conversationEvents({
    required String conversationId,
    required int afterSequence,
    required ChatStreamCancellation cancellation,
  }) async* {
    await for (final event in owned.runtimeEvents(
      conversationId: conversationId,
      afterSequence: afterSequence,
      cancellation: cancellation.token,
    )) {
      yield ChatStreamEvent(
        event['event'].toString(),
        Map<String, dynamic>.from(event['data'] as Map),
      );
    }
  }

  Future<bool> deleteConversation(String id) async {
    await _requireOwned();
    await owned.edit('conversation', id, deleted: true);
    return true;
  }

  Future<void> renameConversation(String id, String title) async {
    final trimmed = title.trim();
    if (trimmed.isEmpty) throw ArgumentError('会话标题不能为空');
    await _requireOwned();
    await owned.edit('conversation', id, changes: {'title': trimmed});
  }

  Future<void> setConversationPinned(String id, bool pinned) async {
    await _requireOwned();
    await owned.edit('conversation', id, changes: {'pinned': pinned});
    return;
  }

  Future<void> markConversationRead(
    String id, {
    int lastTerminalTurnSequence = 0,
    String? characterId,
  }) async {
    await _requireOwned();
    var terminalSequence = lastTerminalTurnSequence;
    if (terminalSequence <= 0) {
      final role = owned.selectRole(characterId);
      final result = await owned.query(id, characterId: role);
      final snapshot = result['snapshot'];
      if (snapshot is Map) {
        for (final row
            in (snapshot['resources'] as List? ?? []).whereType<Map>()) {
          if (row['kind'] != 'conversation' || row['body'] is! Map) continue;
          final body = Map<String, dynamic>.from(row['body'] as Map);
          terminalSequence =
              (body['lastTerminalTurnSequence'] as num?)?.toInt() ?? 0;
          break;
        }
      }
    }
    await owned.edit(
      'conversation',
      id,
      changes: {'lastReadTurnSequence': terminalSequence},
    );
    return;
  }

  Future<void> archiveConversation(String id) async {
    await _requireOwned();
    await owned.edit('conversation', id, changes: {'archived': true});
    return;
  }

  Future<void> restoreArchivedConversation(String id) async {
    await _requireOwned();
    await owned.edit('conversation', id, changes: {'archived': false});
    return;
  }

  Future<void> deleteMessages(String conversationId) async {
    await _requireOwned();
    await owned.edit('conversation', conversationId, clear: true);
    return;
  }

  Future<void> deleteMessage(
    String messageId, {
    Map<String, dynamic>? expectedScope,
    String? expectedOwnerId,
    int? expectedRevision,
  }) async {
    await _requireOwned();
    if (expectedScope == null ||
        expectedOwnerId == null ||
        expectedRevision == null) {
      throw StateError('消息缺少原始数据来源，请重新加载后再操作');
    }
    await owned.edit(
      'message',
      messageId,
      deleted: true,
      expectedScope: expectedScope,
      expectedOwnerId: expectedOwnerId,
      expectedRevision: expectedRevision,
    );
    return;
  }

  Future<void> updateMessage(
    String messageId,
    String content, {
    Map<String, dynamic>? expectedScope,
    String? expectedOwnerId,
    int? expectedRevision,
  }) async {
    await _requireOwned();
    if (expectedScope == null ||
        expectedOwnerId == null ||
        expectedRevision == null) {
      throw StateError('消息缺少原始数据来源，请重新加载后再操作');
    }
    await owned.edit(
      'message',
      messageId,
      changes: {'content': content.trim()},
      expectedScope: expectedScope,
      expectedOwnerId: expectedOwnerId,
      expectedRevision: expectedRevision,
    );
    return;
  }

  Future<Map<String, dynamic>?> conversationSummary(
    String conversationId,
  ) async {
    await _requireOwned();
    return owned.conversationSummary(conversationId);
  }

  Future<Map<String, dynamic>?> generateConversationSummary(
    String conversationId, {
    String characterId = '',
  }) async {
    await _requireOwned();
    return owned.generateConversationSummary(
      conversationId,
      characterId: characterId,
    );
  }

  Future<void> deleteConversationSummary(
    String conversationId, {
    String? displayedViewId,
  }) async {
    await _requireOwned();
    await owned.editConversationSummary(
      conversationId,
      deleted: true,
      displayedViewId: displayedViewId,
    );
    return;
  }

  Future<Map<String, dynamic>?> updateConversationSummary(
    String conversationId,
    String summaryText, {
    String? displayedViewId,
  }) async {
    await _requireOwned();
    await owned.editConversationSummary(
      conversationId,
      text: summaryText,
      displayedViewId: displayedViewId,
    );
    return {'summaryText': summaryText.trim()};
  }

  Future<String> exportConversation(
    String conversationId, {
    String format = 'markdown',
    String characterId = '',
  }) async {
    await _requireOwned();
    final rows = await owned.allMessages(
      conversationId,
      characterId: characterId,
    );
    if (format == 'json') {
      return jsonEncode({
        'conversationId': conversationId,
        'messages': rows
            .map(
              (row) => {
                'id': row.id,
                'ownerId': row.sourceOwnerId,
                'role': row.role,
                'content': row.content,
                'createdAt': row.createdAt,
                if (row.imageUrl.isNotEmpty) 'imageUrl': row.imageUrl,
                if (row.audioUrl.isNotEmpty) 'audioUrl': row.audioUrl,
              },
            )
            .toList(),
      });
    }
    return rows
        .map(
          (row) =>
              '## ${row.role == 'user'
                  ? '用户'
                  : row.role == 'assistant'
                  ? '助手'
                  : '系统'}\n\n${row.content}\n',
        )
        .join('\n');
  }

  Future<Map<String, dynamic>?> contextPreview(String conversationId) {
    return owned.management(
      'context-preview',
      body: {'conversationId': conversationId},
    );
  }

  Future<List<MessageDto>> searchMessages(
    String keyword, {
    String? conversationId,
    int page = 1,
    int pageSize = 50,
  }) async {
    final resp = await owned.management(
      'search',
      body: {
        'keyword': keyword,
        'page': page,
        'pageSize': pageSize,
        if (conversationId != null && conversationId.isNotEmpty)
          'conversationId': conversationId,
      },
    );
    final rows = resp?['items'];
    if (rows is! List) return const [];
    return rows
        .whereType<Map>()
        .map((row) => MessageDto.fromJson(Map<String, dynamic>.from(row)))
        .toList(growable: false);
  }

  ChatStreamCancellation createStreamCancellation() => ChatStreamCancellation();

  Future<void> sendNotificationReply({
    required String message,
    required String clientMessageId,
    required String conversationId,
    String characterId = '',
    String expectedCoreId = '',
  }) async {
    await _requireOwned();
    if (expectedCoreId.isNotEmpty && owned.coreId != expectedCoreId) throw StateError('通知所属Core已变化，不能向当前服务发送原通知回复');
    final requestId = clientMessageId.trim();
    if (requestId.isEmpty || conversationId.isEmpty) throw ArgumentError('通知回复缺少原会话及请求编号');
    Map<String, dynamic>? completed;
    await for (final event in owned.send(
      requestId: requestId,
      message: message,
      conversationId: conversationId,
      characterId: characterId,
    )) {
      if (event['type'] == 'completed' && event['data'] is Map) {
        completed = Map<String, dynamic>.from(event['data'] as Map);
      }
    }
    if (completed?['saved'] != true) throw StateError('消息尚未获得数据所有者保存确认');
  }

  Future<void> interruptTurn(String conversationId, String turnId) async {
    await owned.runtime(
      'interrupt',
      body: {'conversationId': conversationId, 'turnId': turnId},
    );
  }

  Future<void> steerTurn(
    String conversationId,
    String turnId,
    String content,
  ) async {
    await owned.runtime(
      'steer',
      body: {
        'conversationId': conversationId,
        'turnId': turnId,
        'content': content.trim(),
      },
    );
  }

  Future<ChatSubmitResult> retryTurn(
    String conversationId,
    String turnId, {
    String messageStyle = 'flow',
  }) async {
    final resp = await owned.runtime(
      'retry',
      body: {
        'conversationId': conversationId,
        'turnId': turnId,
        'messageStyle': messageStyle,
      },
    );
    return ChatSubmitResult.fromJson(<String, dynamic>{
      ...resp,
      'userMessageId': resp['userMessageId'] ?? '',
    });
  }

  Future<void> resolveTurnApproval(
    String conversationId,
    String turnId,
    String approvalId,
    bool approved,
  ) async {
    await owned.runtime(
      'approval',
      body: {
        'conversationId': conversationId,
        'turnId': turnId,
        'approvalId': approvalId,
        'approved': approved,
      },
    );
  }

}
