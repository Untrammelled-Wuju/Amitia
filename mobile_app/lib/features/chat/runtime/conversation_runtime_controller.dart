import 'dart:async';
import '../../../../core/settings/chat_appearance_preferences.dart';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/models/conversation.dart';
import '../../../core/services/chat_service.dart';
import '../../../core/services/device_owned_attachments.dart';
import '../../../core/services/device_owned_chat_service.dart';
import '../../../core/services/device_owned_realtime_service.dart';
import '../../../core/services/owned_quote_reference.dart';
import '../../../core/services/providers.dart';
import '../../../core/settings/chat_permission_preferences.dart';
import '../../../shared/models/models.dart';
import '../../conversation/rendering/stream/markdown_stream_scheduler.dart';
import 'agent_event_reducer.dart';
import 'citation_projection.dart';
import 'citation_snapshot.dart';
import '../../workbench/runtime/workbench_mobile_events.dart';
import 'automation_status.dart';
import 'conversation_message_ledger.dart';

Set<String> groupMentionIds(String text, List<OwnedGroupMember> members) {
  if (text.contains('@所有成员'))
    return members.map((member) => member.memberId).toSet();
  final sorted = [...members]
    ..sort((a, b) => b.characterName.length.compareTo(a.characterName.length));
  final result = <String>{};
  for (var index = 0; index < text.length; index++) {
    if (text[index] != '@') continue;
    for (final member in sorted) {
      final token = '@${member.characterName}';
      if (!text.startsWith(token, index)) continue;
      final end = index + token.length;
      if (end < text.length &&
          !RegExp(r'[\s，。！？、,.!?;；:：]').hasMatch(text[end]))
        continue;
      result.add(member.memberId);
      index = end - 1;
      break;
    }
  }
  return result;
}

List<Map<String, dynamic>> groupMentionsFromText(
  String text,
  List<OwnedGroupMember> members,
) {
  final active = members
      .where((member) => member.status == 'active')
      .toList(growable: false);
  final ids = groupMentionIds(text, active);
  return active
      .where((member) => ids.contains(member.memberId))
      .map((member) => member.toJson())
      .toList(growable: false);
}

class ConversationRuntimeController extends ChangeNotifier {
  ConversationRuntimeController(
    this._chatService, {
    ChatPermissionPreferencesNotifier? permissionPreferences,
    ChatAppearancePreferencesNotifier? appearancePreferences,
  }) : _permissionPreferences = permissionPreferences,
       _appearancePreferences = appearancePreferences,
       _permissionMode = normalizeChatPermissionMode(
         permissionPreferences?.mode,
       ) {
    _ownedPoll = Timer.periodic(
      const Duration(seconds: 3),
      (_) => unawaited(_pollOwnedState()),
    );
  }

  final ChatService _chatService;
  final ChatPermissionPreferencesNotifier? _permissionPreferences;
  final ChatAppearancePreferencesNotifier? _appearancePreferences;
  final ConversationMessageLedger _messages = ConversationMessageLedger();
  final AgentEventReducer _agentReducer = AgentEventReducer();
  final MarkdownStreamScheduler _streamScheduler = MarkdownStreamScheduler();
  final Map<String, Map<String, dynamic>> _pendingApprovals =
      <String, Map<String, dynamic>>{};
  String? _conversationId;
  String _conversationTitle = '';
  String? _characterId;
  ConversationWorkspaceDto? _workspace;
  bool _sending = false;
  Object? _lastError;
  int _draftEpoch = 0;
  int _conversationUpdateEpoch = 0;
  int _modelConfigId = 0;
  String _reasoningEffort = 'high';
  bool _reasoningEnabled = true;
  String _permissionMode;
  bool _disposed = false;
  int _runtimeEpoch = 0;
  ChatStreamCancellation? _eventCancellation;
  bool _loadingSnapshot = false;
  bool _loadingOlderMessages = false;
  bool _hasMoreMessageHistory = false;
  bool _hasMoreTurnHistory = false;
  bool _automationStateUncertain = false;
  String _turnHistoryCursor = '';
  Timer? _ownedPoll;
  bool _ownedPolling = false;
  String _ownedNotice = '';
  String _ownedRequestId = '';
  String _conversationCoreId = '';
  Map<String, dynamic>? _realtimeScope;
  OwnedGroupState? _group;
  List<Map<String, dynamic>> _groupMentions = <Map<String, dynamic>>[];

  Map<String, dynamic>? get realtimeScope => _realtimeScope;
  OwnedGroupState? get group => _group;
  List<Map<String, dynamic>> get groupMentions =>
      List<Map<String, dynamic>>.unmodifiable(_groupMentions);

  List<ChatMessage> get messages => _messages.messages;
  String? get conversationId => _conversationId;
  String get conversationTitle => _conversationTitle;
  ConversationWorkspaceDto? get workspace => _workspace;
  bool get sending => _sending;
  Object? get lastError => _lastError;
  String get state => _sending ? 'sending' : 'idle';
  int get draftEpoch => _draftEpoch;
  int get modelConfigId => _modelConfigId;
  String get reasoningEffort => _reasoningEffort;
  bool get reasoningEnabled => _reasoningEnabled;
  String get permissionMode => _permissionMode;
  bool get hasMoreHistory => _hasMoreMessageHistory || _hasMoreTurnHistory;
  bool get isLoadingOlderMessages => _loadingOlderMessages;
  int get conversationUpdateEpoch => _conversationUpdateEpoch;
  String get activeTurnId => _agentReducer.activeTurnId;
  String get activeExecutionId => _agentReducer.activeExecutionId;
  AutomationStatus? get automationStatus {
    final status = projectAutomationStatus(_agentReducer.turns);
    if (status == null || !_automationStateUncertain) return status;
    return AutomationStatus(
      phase: 'waiting',
      label: '正在同步自动化状态',
      detail: '连接恢复后确认操作状态',
      count: status.count,
    );
  }

  List<Map<String, dynamic>> get pendingApprovals => _pendingApprovals.values
      .map((item) => Map<String, dynamic>.from(item))
      .toList(growable: false);

  void setCharacterId(String? characterId) {
    final value = characterId?.trim() ?? '';
    _characterId = value.isEmpty ? null : value;
  }

  void setGroupMentions(List<Map<String, dynamic>> mentions) {
    _groupMentions = mentions
        .map((mention) => Map<String, dynamic>.from(mention))
        .toList(growable: false);
    notifyListeners();
  }

  Future<OwnedGroupState?> refreshGroup() async {
    final id = _conversationId?.trim() ?? '';
    if (id.isEmpty) {
      _group = null;
      notifyListeners();
      return null;
    }
    _group = await _chatService.owned.groupState(id, characterId: _characterId);
    notifyListeners();
    return _group;
  }

  Future<OwnedGroupState> inviteGroupMembers(
    List<Map<String, dynamic>> members,
  ) async {
    final id = _conversationId?.trim() ?? '';
    final role = _characterId?.trim() ?? '';
    if (id.isEmpty || role.isEmpty) {
      throw StateError('请先发送一条消息建立会话');
    }
    _group = await _chatService.owned.inviteGroupMembers(
      id,
      characterId: role,
      expectedMembershipRevision: _group?.membershipRevision ?? 0,
      members: members,
    );
    notifyListeners();
    return _group!;
  }

  Future<OwnedGroupState> createGroupConversation(
    String baseRoleId,
    List<String> invitedRoleIds,
  ) async {
    if (baseRoleId.trim().isEmpty || invitedRoleIds.isEmpty) {
      throw StateError('请选择默认发言角色和至少一个邀请角色');
    }
    final group = await _chatService.owned.createGroupConversation(
      characterId: baseRoleId,
      members: invitedRoleIds
          .where((id) => id != baseRoleId)
          .map(
            (id) => <String, dynamic>{'characterId': id, 'shareHistory': false},
          )
          .toList(growable: false),
    );
    _group = group;
    _groupMentions = <Map<String, dynamic>>[];
    _characterId = baseRoleId;
    _conversationId = group.conversationId;
    _conversationTitle = group.title;
    _messages.clear();
    _lastError = null;
    _sending = false;
    _conversationUpdateEpoch++;
    notifyListeners();
    return group;
  }

  Future<OwnedGroupState> removeGroupMember(OwnedGroupMember member) async {
    final id = _conversationId?.trim() ?? '';
    final role = _characterId?.trim() ?? '';
    if (id.isEmpty || role.isEmpty || _group == null) {
      throw StateError('请先打开一个有效群聊');
    }
    _group = await _chatService.owned.removeGroupMember(
      id,
      characterId: role,
      memberId: member.memberId,
      expectedMembershipRevision: _group!.membershipRevision,
    );
    _groupMentions = _groupMentions
        .where((mention) => mention['memberId']?.toString() != member.memberId)
        .toList(growable: false);
    notifyListeners();
    return _group!;
  }

  Future<OwnedGroupState> updateGroupMentionsEnabled(bool enabled) async {
    final id = _conversationId?.trim() ?? '';
    final role = _characterId?.trim() ?? '';
    if (id.isEmpty || role.isEmpty || _group == null) {
      throw StateError('请先打开一个有效群聊');
    }
    _group = await _chatService.owned.updateGroupSettings(
      id,
      characterId: role,
      expectedSettingsRevision: _group!.settingsRevision,
      allowCharacterMentions: enabled,
    );
    notifyListeners();
    return _group!;
  }

  Future<OwnedGroupState> renameGroup(String name) async {
    final id = _conversationId?.trim() ?? '';
    final role = _characterId?.trim() ?? '';
    if (id.isEmpty || role.isEmpty || _group == null) {
      throw StateError('请先打开一个有效群聊');
    }
    _group = await _chatService.owned.updateGroupSettings(
      id,
      characterId: role,
      expectedSettingsRevision: _group!.settingsRevision,
      name: name,
    );
    _conversationTitle = _group!.title;
    notifyListeners();
    return _group!;
  }

  Future<void> requestGroupReply(
    OwnedGroupMember member, {
    required String sourceMessageId,
  }) async {
    final id = _conversationId?.trim() ?? '';
    final role = _characterId?.trim() ?? '';
    if (id.isEmpty || role.isEmpty || sourceMessageId.isEmpty) {
      throw StateError('请先发送内容后再点名角色');
    }
    await _chatService.owned.requestGroupReplies(
      id,
      characterId: role,
      sourceMessageId: sourceMessageId,
      members: [member.toJson()],
    );
    await openConversation(id, characterId: role);
  }

  void setWorkspace(ConversationWorkspaceDto? workspace) {
    final previous = _workspace;
    if (previous?.workspaceId == workspace?.workspaceId &&
        previous?.projectId == workspace?.projectId &&
        previous?.deviceId == workspace?.deviceId &&
        previous?.workspaceName == workspace?.workspaceName &&
        previous?.rootUri == workspace?.rootUri) {
      return;
    }
    _workspace = workspace;
    notifyListeners();
  }

  Future<void> updateModelSettings(
    int modelConfigId,
    String reasoningEffort,
    bool reasoningEnabled,
  ) async {
    previewModelSettings(modelConfigId, reasoningEffort, reasoningEnabled);
    final id = _conversationId?.trim() ?? '';
    if (id.isEmpty) return;
    await _chatService.updateConversationModelSettings(
      id,
      modelConfigId: _modelConfigId,
      reasoningEffort: _reasoningEffort,
      reasoningEnabled: _reasoningEnabled,
    );
  }

  void previewModelSettings(
    int modelConfigId,
    String reasoningEffort,
    bool reasoningEnabled,
  ) {
    _modelConfigId = modelConfigId;
    _reasoningEffort = reasoningEffort.trim().isEmpty
        ? 'high'
        : reasoningEffort.trim();
    _reasoningEnabled = reasoningEnabled;
    notifyListeners();
  }

  Future<void> updatePermissionMode(String permissionMode) async {
    final next = normalizeChatPermissionMode(permissionMode);
    if (_permissionMode != next) {
      _permissionMode = next;
      notifyListeners();
    }
    await _permissionPreferences?.setMode(next);
    final id = _conversationId?.trim() ?? '';
    if (id.isEmpty) return;
    await _chatService.updateConversationPermissionMode(id, next);
  }

  Future<void> sendText(
    String text, {
    String? replyToMessageId,
    String? replyToExcerpt,
    ChatMessage? quotedMessage,
  }) async {
    final value = text.trim();
    if (value.isEmpty || _sending) return;
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.text,
        content: value,
        time: DateTime.now(),
        status: MessageStatus.sending,
        replyToMessageId: replyToMessageId,
        replyToExcerpt: replyToExcerpt,
      ),
      message: value,
      replyToMessageId: replyToMessageId,
      quotedMessage: quotedMessage,
    );
  }

  Future<void> sendCode(String language, String code) async {
    final body = code.trim();
    if (body.isEmpty || _sending) return;
    final lang = language.trim().isEmpty ? 'text' : language.trim();
    final content = '```$lang\n$body\n```';
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.code,
        content: content,
        time: DateTime.now(),
        status: MessageStatus.sending,
      ),
      message: content,
    );
  }

  Future<void> sendImage({
    required String resourceUri,
    required String displayUrl,
    required String fileName,
    String mimeType = 'image/*',
    String text = '',
  }) async {
    if (_sending) return;
    final content = text.trim().isEmpty ? '[图片]' : text.trim();
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.image,
        content: content,
        time: DateTime.now(),
        status: MessageStatus.sending,
        fileName: fileName,
        resourceUri: resourceUri,
        mediaUrl: displayUrl,
        mimeType: mimeType,
      ),
      message: content,
      imageUrl: resourceUri,
    );
  }

  Future<void> sendVideo({
    required String resourceUri,
    required String displayUrl,
    required String fileName,
    String mimeType = 'video/*',
    int durationMs = 0,
    String text = '',
  }) async {
    if (_sending) return;
    final content = text.trim().isEmpty ? '[视频]' : text.trim();
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.video,
        content: content,
        time: DateTime.now(),
        status: MessageStatus.sending,
        fileName: fileName,
        resourceUri: resourceUri,
        mediaUrl: displayUrl,
        mimeType: mimeType,
        durationMs: durationMs,
      ),
      message: content,
      videoUrl: resourceUri,
    );
  }

  Future<void> sendVoice({
    required String resourceUri,
    required String displayUrl,
    required String fileName,
    String mimeType = 'audio/*',
    int durationMs = 0,
    String text = '',
  }) async {
    if (_sending) return;
    final content = text.trim().isEmpty ? '[语音]' : text.trim();
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.audio,
        content: content,
        time: DateTime.now(),
        status: MessageStatus.sending,
        fileName: fileName,
        resourceUri: resourceUri,
        mediaUrl: displayUrl,
        mimeType: mimeType,
        durationMs: durationMs,
      ),
      message: content,
      audioUrl: resourceUri,
      audioDuration: durationMs / 1000.0,
    );
  }

  Future<void> sendFile({
    required String resourceUri,
    required String fileName,
    required int sizeBytes,
    String mimeType = 'application/octet-stream',
  }) async {
    if (_sending) return;
    final content = '[文件] $fileName\n$resourceUri';
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.file,
        content: content,
        time: DateTime.now(),
        status: MessageStatus.sending,
        fileName: fileName,
        fileSizeKB: (sizeBytes / 1024).ceil(),
        resourceUri: resourceUri,
        mimeType: mimeType,
      ),
      message: content,
    );
  }

  Future<void> addUserMessage(ChatMessage message) async {
    if (message.content.trim().isEmpty || _sending) return;
    await _send(message, message: message.content);
  }

  Future<void> _send(
    ChatMessage localMessage, {
    required String message,
    String? imageUrl,
    String? audioUrl,
    double audioDuration = 0,
    String? videoUrl,
    String? replyToMessageId,
    ChatMessage? quotedMessage,
  }) async {
    if (_sending) return;
    try {
      if (await _chatService.owned.refresh()) {
        await _sendOwned(
          localMessage,
          message: message,
          imageUrl: imageUrl,
          audioUrl: audioUrl,
          videoUrl: videoUrl,
          quotedMessage: quotedMessage,
        );
        return;
      }
    } catch (error) {
      _lastError = error;
      notifyListeners();
      return;
    }
    _lastError = StateError(
      _chatService.owned.notice.isNotEmpty
          ? _chatService.owned.notice
          : '设备对话服务尚未就绪',
    );
    notifyListeners();
  }

  Future<void> _pollOwnedState() async {
    if (_ownedPolling || _disposed) return;
    _ownedPolling = true;
    try {
      await _chatService.owned.poll();
    } catch (_) {
      if (_chatService.owned.enabled && _ownedRequestId.isNotEmpty)
        _chatService.owned.stopLocal('云端连接中断，当前回复已暂停；恢复连接后不会自动重新发送。');
    } finally {
      _ownedPolling = false;
    }
    if (_disposed) return;
    final notice = _chatService.owned.notice;
    if (notice.isNotEmpty && notice != _ownedNotice) {
      _ownedNotice = notice;
      _messages.upsert(
        ChatMessage(
          id: _localId('provider-notice'),
          role: MessageRole.system,
          type: MessageType.systemNotice,
          content: notice,
          time: DateTime.now(),
          sequence: _nextLocalSequence(),
        ),
      );
      notifyListeners();
    }
  }

  Future<void> _sendOwned(
    ChatMessage localMessage, {
    required String message,
    String? imageUrl,
    String? audioUrl,
    String? videoUrl,
    ChatMessage? quotedMessage,
  }) async {
    final attachments = <Map<String, dynamic>>[
      if ((audioUrl ?? '').isNotEmpty)
        ownedAudioAttachment(
          audioUrl!,
          name: localMessage.fileName ?? 'voice.wav',
        ),
      if ((imageUrl ?? '').isNotEmpty)
        ownedImageAttachment(
          imageUrl!,
          name: localMessage.fileName ?? 'image.png',
        ),
      if ((videoUrl ?? '').isNotEmpty)
        ownedFileAttachment(
          videoUrl!,
          name: localMessage.fileName ?? 'video.mp4',
          kind: 'video',
        ),
      if (localMessage.type == MessageType.file)
        ownedFileAttachment(
          localMessage.resourceUri ?? '',
          name: localMessage.fileName ?? '',
        ),
    ];
    if (attachments.length > 2 ||
        ((audioUrl ?? '').isNotEmpty &&
            attachments.any(
              (row) => row['kind'] != 'audio' && row['kind'] != 'image',
            ))) {
      throw StateError('单条消息最多两个附件，语音只能同时携带图片');
    }
    if (localMessage.type == MessageType.file) {
      message = '[文件] ${localMessage.fileName}';
    }
    final requestId = _localId('mobile-');
    if (_group != null && (_conversationId?.trim().isNotEmpty ?? false)) {
      await _sendOwnedGroup(
        localMessage,
        requestId: requestId,
        message: message,
        attachments: attachments,
        quotedMessage: quotedMessage,
      );
      return;
    }
    final wasDraft = (_conversationId ?? '').isEmpty;
    final previousContext =
        _conversationCoreId.isNotEmpty &&
            _conversationCoreId != _chatService.owned.coreId &&
            (_conversationId ?? '').isNotEmpty
        ? <String, dynamic>{
            'previousCoreId': _conversationCoreId,
            'conversationId': _conversationId,
            'summary':
                _chatService.owned.previousSummary(
                  _conversationCoreId,
                  _conversationId!,
                ) ??
                '',
            'messages': _messages.messages
                .where(
                  (row) =>
                      (row.role == MessageRole.user ||
                          row.role == MessageRole.assistant) &&
                      row.status != MessageStatus.sending &&
                      row.status != MessageStatus.error,
                )
                .toList()
                .reversed
                .take(128)
                .toList()
                .reversed
                .map(
                  (row) => {
                    'id': row.id,
                    'ownerId': row.sourceOwnerId,
                    'role': row.role.name,
                    'content': row.content,
                    'status': row.status.name,
                  },
                )
                .toList(),
          }
        : null;
    final pending = _cloneMessage(
      localMessage,
      id: '$requestId/user',
      renderId: requestId,
      sequence: _nextLocalSequence(),
      status: MessageStatus.sending,
    );
    _messages.upsert(pending);
    _sending = true;
    _ownedRequestId = requestId;
    _lastError = null;
    var text = '';
    var reasoning = '';
    var reasoningComplete = false;
    var outputComplete = false;
    var citationSnapshot = const <Map<String, dynamic>>[];
    var accepted = false;
    var completed = false;
    final draftProjectId = wasDraft ? (_workspace?.projectId ?? '') : '';
    Map<String, dynamic> acceptedScope = {};
    String sourceConversationId = '';
    int? savedRevision;
    final time = DateTime.now();
    final sequence = _nextLocalSequence();
    void project(MessageStatus status) {
      if (_disposed) return;
      final local = _messages.findByRenderId(
        _assistantRenderId(requestId),
        role: MessageRole.assistant,
        sourceOwnerId: '',
      );
      if (local != null && acceptedScope.isNotEmpty) _messages.remove(local);
      _messages.upsert(
        ChatMessage(
          id: '$requestId/assistant',
          sourceOwnerId: (acceptedScope['resourceOwnerId'] ?? '').toString(),
          sourceConversationId: sourceConversationId,
          sourceScope: acceptedScope,
          sourceRevision: savedRevision,
          renderId: _assistantRenderId(requestId),
          characterId: _characterId ?? '',
          role: MessageRole.assistant,
          type: MessageType.text,
          content: text,
          reasoningContent: reasoning,
          reasoningComplete: reasoningComplete,
          outputComplete: outputComplete,
          citationSnapshot: citationSnapshot,
          time: time,
          sequence: sequence,
          status: status,
        ),
      );
      notifyListeners();
    }

    project(MessageStatus.queued);
    try {
      await _appearancePreferences?.init();
      await for (final event in _chatService.owned.send(
        requestId: requestId,
        message: message,
        conversationId: _conversationId,
        projectId: draftProjectId,
        characterId: _characterId,
        context: previousContext,
        modelConfigId: _modelConfigId,
        reasoningEffort: _reasoningEffort,
        reasoningEnabled: _reasoningEnabled,
        permissionMode: _permissionMode,
        messageStyle: _appearancePreferences?.messageStyle.name ?? 'flow',
        attachments: attachments,
        quote: quotedMessage == null
            ? null
            : ownedQuoteReference(quotedMessage),
      )) {
        if (_disposed || _ownedRequestId != requestId) break;
        final type = event['type'];
        if (event['sourceConversationId'] is String) {
          sourceConversationId = event['sourceConversationId'] as String;
        }
        if (type == 'started') {
          accepted = true;
          _conversationId = (event['conversationId'] ?? '').toString();
          final scope = event['executionScope'] as Map;
          acceptedScope = Map<String, dynamic>.from(scope);
          _realtimeScope = Map.unmodifiable(acceptedScope);
          _connectEventStream(_conversationId!);
          _characterId = scope['roleId'].toString();
          _conversationCoreId = scope['coreId'].toString();
          _messages.remove(pending);
          _messages.upsert(
            _cloneMessage(
              pending,
              status: MessageStatus.delivered,
              sourceOwnerId: scope['resourceOwnerId'].toString(),
              sourceConversationId: sourceConversationId,
              characterId: scope['roleId'].toString(),
              sourceScope: acceptedScope,
              sourceRevision: 1,
            ),
          );
          if (wasDraft) _conversationUpdateEpoch++;
          project(MessageStatus.streaming);
        } else if (type == 'task') {
          if (accepted)
            WorkbenchMobileEvents.taskChanged(
              event,
              coreId: _conversationCoreId,
              characterId: _characterId ?? '',
              conversationId: _conversationId ?? '',
            );
        } else if (type == 'transcribed') {
          final current = _messages.findById(
            pending.id,
            sourceOwnerId: (acceptedScope['resourceOwnerId'] ?? '').toString(),
          );
          if (current != null)
            _messages.upsert(
              _cloneMessage(
                current,
                content: (event['text'] ?? '').toString(),
                sourceRevision: 2,
              ),
            );
          notifyListeners();
        } else if (type == 'delta') {
          if (event['reasoning'] == true) {
            reasoningComplete = false;
            reasoning += (event['text'] ?? '').toString();
          } else {
            reasoningComplete = true;
            text += (event['text'] ?? '').toString();
          }
          project(MessageStatus.streaming);
        } else if (type == 'reasoning-completed') {
          reasoningComplete = true;
          project(MessageStatus.streaming);
        } else if (type == 'output-completed') {
          reasoningComplete = true;
          outputComplete = true;
          project(MessageStatus.streaming);
        } else if (type == 'completed') {
          final result = event['data'] as Map;
          text = (result['reply'] ?? text).toString();
          reasoning = (result['reasoning'] ?? reasoning).toString();
          citationSnapshot = normalizeCitationSnapshot(
            result['citationSnapshot'],
          );
          completed = true;
          reasoningComplete = true;
          outputComplete = true;
          savedRevision = result['saved'] == true ? 1 : null;
          final current = _messages.findById(
            pending.id,
            sourceOwnerId: (acceptedScope['resourceOwnerId'] ?? '').toString(),
          );
          if (current != null &&
              result['transcription'] is String &&
              (result['transcription'] as String).isNotEmpty)
            _messages.upsert(
              _cloneMessage(
                current,
                content: result['transcription'] as String,
                sourceRevision: result['userRevision'] is int
                    ? result['userRevision'] as int
                    : 2,
              ),
            );
          project(MessageStatus.delivered);
        } else if (type == 'interrupted' && event['data'] is Map) {
          final result = event['data'] as Map;
          if (result['saved'] == true) {
            text = (result['reply'] ?? text).toString();
            reasoning = (result['reasoning'] ?? reasoning).toString();
          }
          project(MessageStatus.interrupted);
        }
      }
    } catch (error) {
      if (!_disposed && !completed) {
        _lastError = error;
        if (!accepted) {
          _messages.upsert(_cloneMessage(pending, status: MessageStatus.error));
          final placeholder = _messages.findByRenderId(
            _assistantRenderId(requestId),
            role: MessageRole.assistant,
          );
          if (placeholder != null) _messages.remove(placeholder);
        }
        if (accepted || text.isNotEmpty || reasoning.isNotEmpty)
          project(MessageStatus.interrupted);
      }
    } finally {
      if (_ownedRequestId == requestId) {
        _ownedRequestId = '';
        if (!_disposed) {
          _sending = false;
          if (completed) {
            _conversationUpdateEpoch++;
            final id = _conversationId?.trim() ?? '';
            if (id.isNotEmpty) {
              unawaited(
                _chatService
                    .markConversationRead(id, characterId: _characterId)
                    .catchError((Object _) {}),
              );
            }
          }
          notifyListeners();
        }
      }
    }
  }

  Future<void> _sendOwnedGroup(
    ChatMessage localMessage, {
    required String requestId,
    required String message,
    required List<Map<String, dynamic>> attachments,
    ChatMessage? quotedMessage,
  }) async {
    final conversationId = _conversationId?.trim() ?? '';
    final group = _group;
    final role = _characterId?.trim() ?? '';
    if (conversationId.isEmpty || group == null || role.isEmpty) {
      throw StateError('群聊状态尚未就绪');
    }
    final pending = _cloneMessage(
      localMessage,
      id: '$requestId/user',
      renderId: requestId,
      sequence: _nextLocalSequence(),
      status: MessageStatus.sending,
    );
    _messages.upsert(pending);
    _sending = true;
    _ownedRequestId = requestId;
    _lastError = null;
    final mentions = groupMentionsFromText(message, group.activeMembers);
    var completed = false;
    String sourceConversationId = conversationId;
    Map<String, dynamic> expectedScope = <String, dynamic>{};
    try {
      await _appearancePreferences?.init();
      await for (final event in _chatService.owned.sendGroup(
        requestId: requestId,
        message: message,
        conversationId: conversationId,
        characterId: role,
        group: group,
        mentions: mentions,
        context: null,
        modelConfigId: _modelConfigId,
        reasoningEffort: _reasoningEffort,
        reasoningEnabled: _reasoningEnabled,
        permissionMode: _permissionMode,
        messageStyle: _appearancePreferences?.messageStyle.name ?? 'flow',
        attachments: attachments,
      )) {
        if (_disposed || _ownedRequestId != requestId) break;
        final type = event['type'];
        if (event['sourceConversationId'] is String) {
          sourceConversationId = event['sourceConversationId'] as String;
        }
        if (type == 'started') {
          final scope = event['executionScope'];
          if (scope is! Map) continue;
          expectedScope = Map<String, dynamic>.from(scope);
          _realtimeScope = Map.unmodifiable(expectedScope);
          final deliveryId = (event['deliveryId'] ?? '').toString();
          final childRequestId = (scope['requestId'] ?? '').toString();
          if (deliveryId.isEmpty || childRequestId.isEmpty) continue;
          final currentUser = _messages.findById(pending.id, sourceOwnerId: '');
          if (currentUser != null) {
            _messages.upsert(
              _cloneMessage(
                currentUser,
                status: MessageStatus.delivered,
                sourceOwnerId: scope['resourceOwnerId'].toString(),
                sourceConversationId: sourceConversationId,
                sourceScope: expectedScope,
                sourceRevision: 1,
              ),
            );
          }
          final assistantId = '$childRequestId/assistant';
          _messages.upsert(
            ChatMessage(
              id: assistantId,
              sourceOwnerId: (scope['resourceOwnerId'] ?? '').toString(),
              sourceConversationId: sourceConversationId,
              sourceScope: expectedScope,
              renderId: 'group-delivery:$deliveryId',
              characterId: (event['characterId'] ?? '').toString(),
              characterName: (event['characterName'] ?? '').toString(),
              characterAvatar: (event['characterAvatar'] ?? '').toString(),
              groupRoundId: (event['groupRoundId'] ?? '').toString(),
              deliveryId: deliveryId,
              role: MessageRole.assistant,
              type: MessageType.text,
              content: '',
              time: DateTime.now(),
              sequence: _nextLocalSequence(),
              status: MessageStatus.streaming,
            ),
          );
          _connectEventStream(sourceConversationId);
          notifyListeners();
        } else if (type == 'delta') {
          final deliveryId = (event['deliveryId'] ?? '').toString();
          final current = _messages.messages
              .where((item) => item.deliveryId == deliveryId)
              .firstOrNull;
          if (current == null) continue;
          final reasoning = event['reasoning'] == true;
          _messages.upsert(
            _cloneMessage(
              current,
              content: reasoning
                  ? current.content
                  : current.content + (event['text'] ?? '').toString(),
              status: MessageStatus.streaming,
            ),
          );
          final reasoningText = reasoning
              ? current.reasoningContent + (event['text'] ?? '').toString()
              : current.reasoningContent;
          final updated = _messages.messages
              .where((item) => item.deliveryId == deliveryId)
              .firstOrNull;
          if (updated != null) {
            _messages.upsert(
              ChatMessage(
                id: updated.id,
                sourceOwnerId: updated.sourceOwnerId,
                sourceConversationId: updated.sourceConversationId,
                sourceScope: updated.sourceScope,
                renderId: updated.renderId,
                characterId: updated.characterId,
                characterName: updated.characterName,
                characterAvatar: updated.characterAvatar,
                groupRoundId: updated.groupRoundId,
                deliveryId: updated.deliveryId,
                role: updated.role,
                type: updated.type,
                content: updated.content,
                reasoningContent: reasoningText,
                reasoningComplete: !reasoning,
                time: updated.time,
                sequence: updated.sequence,
                status: MessageStatus.streaming,
              ),
            );
          }
          notifyListeners();
        } else if (type == 'group.delivery.failed') {
          final deliveryId = (event['deliveryId'] ?? '').toString();
          final current = _messages.messages
              .where((item) => item.deliveryId == deliveryId)
              .firstOrNull;
          if (current != null) {
            _messages.upsert(
              _cloneMessage(
                current,
                content: current.content.isEmpty
                    ? (event['text'] ?? '生成失败').toString()
                    : current.content,
                status: MessageStatus.error,
              ),
            );
            notifyListeners();
          }
        } else if (type == 'group.completed') {
          final result = event['data'];
          if (result is! Map) continue;
          final response = Map<String, dynamic>.from(result);
          final responseConversation =
              (response['conversationId'] ?? sourceConversationId).toString();
          _conversationId = responseConversation;
          sourceConversationId = responseConversation;
          final userMessageId = (response['userMessageId'] ?? '').toString();
          final currentUser = _messages.messages
              .where(
                (item) => item.id == pending.id || item.id == '$requestId/user',
              )
              .firstOrNull;
          if (currentUser != null && userMessageId.isNotEmpty) {
            _messages.upsert(
              _cloneMessage(
                currentUser,
                id: userMessageId,
                status: MessageStatus.delivered,
                sourceOwnerId: (expectedScope['resourceOwnerId'] ?? '')
                    .toString(),
                sourceConversationId: sourceConversationId,
                sourceRevision: 1,
              ),
            );
          }
          for (final raw
              in (response['deliveries'] as List? ?? []).whereType<Map>()) {
            final delivery = Map<String, dynamic>.from(raw);
            final deliveryId = (delivery['deliveryId'] ?? '').toString();
            final scope = delivery['executionScope'];
            if (deliveryId.isEmpty || scope is! Map) continue;
            final existing = _messages.messages
                .where((item) => item.deliveryId == deliveryId)
                .firstOrNull;
            final message = ChatMessage(
              id: '${(delivery['requestId'] ?? '').toString()}/assistant',
              sourceOwnerId: (scope['resourceOwnerId'] ?? '').toString(),
              sourceConversationId: sourceConversationId,
              sourceScope: Map<String, dynamic>.from(scope),
              sourceRevision: delivery['saved'] == true ? 1 : null,
              renderId: 'group-delivery:$deliveryId',
              characterId: (scope['roleId'] ?? '').toString(),
              characterName: (delivery['characterName'] ?? '').toString(),
              characterAvatar: (delivery['characterAvatar'] ?? '').toString(),
              groupRoundId: (delivery['groupRoundId'] ?? '').toString(),
              deliveryId: deliveryId,
              role: MessageRole.assistant,
              type: MessageType.text,
              content: (delivery['reply'] ?? '').toString(),
              reasoningContent: (delivery['reasoning'] ?? '').toString(),
              reasoningComplete: true,
              outputComplete: true,
              time: existing?.time ?? DateTime.now(),
              sequence: existing?.sequence ?? _nextLocalSequence(),
              status: MessageStatus.delivered,
              citationSnapshot: (delivery['citationSnapshot'] as List? ?? [])
                  .whereType<Map>()
                  .map((row) => Map<String, dynamic>.from(row))
                  .toList(growable: false),
            );
            _messages.upsert(message);
          }
          _groupMentions = <Map<String, dynamic>>[];
          completed = true;
          notifyListeners();
        }
      }
    } catch (error) {
      if (!_disposed && !completed) {
        _lastError = error;
        final current =
            _messages.findById(
              pending.id,
              sourceOwnerId: (expectedScope['resourceOwnerId'] ?? '')
                  .toString(),
            ) ??
            _messages.findById(pending.id, sourceOwnerId: '');
        if (current != null) {
          _messages.upsert(_cloneMessage(current, status: MessageStatus.error));
        }
      }
    } finally {
      if (_ownedRequestId == requestId) {
        _ownedRequestId = '';
        if (!_disposed) {
          _sending = false;
          if (completed) _conversationUpdateEpoch++;
          notifyListeners();
        }
      }
    }
  }

  Future<void> _refreshSnapshot(String conversationId) async {
    if (_loadingSnapshot || _disposed || _conversationId != conversationId) {
      return;
    }
    _loadingSnapshot = true;
    try {
      final snapshot = await _chatService.conversationSnapshot(conversationId);
      if (_disposed || _conversationId != conversationId) return;
      _applySnapshot(snapshot);
    } finally {
      _loadingSnapshot = false;
    }
  }

  void _applySnapshot(ConversationSnapshotDto snapshot) {
    if (snapshot.version != 1) {
      throw StateError('不支持的会话快照版本: ${snapshot.version}');
    }
    _automationStateUncertain = false;
    final conversation = snapshot.conversation;
    if (conversation != null) {
      _conversationId = conversation.id;
      _conversationTitle = conversation.title;
      _modelConfigId = conversation.modelConfigId;
      _reasoningEffort = conversation.reasoningEffort.trim().isEmpty
          ? 'high'
          : conversation.reasoningEffort.trim();
      _reasoningEnabled = conversation.reasoningEnabled != 0;
      _permissionMode = normalizeChatPermissionMode(
        conversation.permissionMode,
      );
    }
    if (snapshot.workspace != null) {
      _workspace = snapshot.workspace;
    } else if ((conversation?.projectId ?? '').isEmpty) {
      _workspace = null;
    }
    final snapshotMessageIds = <String>{};
    for (final dto in snapshot.messages) {
      final message = _messageFromDto(dto);
      snapshotMessageIds.add(message.id);
      _messages.upsert(message);
    }
    _messages.removeWhere(
      (message) =>
          !snapshotMessageIds.contains(message.id) &&
          !_isLiveAssistantMessage(message) &&
          message.status != MessageStatus.sending,
    );
    _agentReducer.reset(
      turns: snapshot.turns,
      lastEventSequence: snapshot.lastEventSequence,
      activeTurn: snapshot.activeTurn,
    );
    _pendingApprovals.clear();
    for (final approval in snapshot.approvals) {
      final approvalId = (approval['id'] ?? '').toString().trim();
      if (approvalId.isEmpty) continue;
      _pendingApprovals[approvalId] = Map<String, dynamic>.from(approval);
    }
    _hasMoreMessageHistory = snapshot.hasMoreMessages;
    _turnHistoryCursor = snapshot.turnNextCursor;
    _hasMoreTurnHistory = snapshot.hasMoreTurns;
    final active = _agentReducer.turn(_agentReducer.activeTurnId);
    _sending = active != null && active.status != 'needs_reconciliation';
    _projectTurns();
    notifyListeners();
  }

  void _connectEventStream(String conversationId) {
    final id = conversationId.trim();
    if (id.isEmpty || _disposed) return;
    _eventCancellation?.cancel('conversation changed');
    _eventCancellation = null;
    final epoch = ++_runtimeEpoch;
    unawaited(_runEventStream(id, epoch));
  }

  Future<void> _runEventStream(String conversationId, int epoch) async {
    while (!_disposed &&
        epoch == _runtimeEpoch &&
        _conversationId == conversationId) {
      final cancellation = _chatService.createStreamCancellation();
      _eventCancellation = cancellation;
      try {
        await for (final streamEvent in _chatService.conversationEvents(
          conversationId: conversationId,
          afterSequence: _agentReducer.lastEventSequence,
          cancellation: cancellation,
        )) {
          if (_disposed ||
              epoch != _runtimeEpoch ||
              cancellation.isCancelled ||
              _conversationId != conversationId) {
            return;
          }
          if (streamEvent.type == 'snapshot.required') {
            await _refreshSnapshot(conversationId);
            cancellation.cancel('snapshot refreshed');
            break;
          }
          if (streamEvent.type != 'agent_ui_event') continue;
          final event = AgentUIEvent.fromJson(streamEvent.data);
          final result = _agentReducer.apply(event);
          if (result == AgentEventApplyResult.gap) {
            await _refreshSnapshot(conversationId);
            cancellation.cancel('event gap');
            break;
          }
          if (result != AgentEventApplyResult.applied) continue;
          _handleAppliedEvent(event);
        }
      } catch (error) {
        if (_disposed ||
            epoch != _runtimeEpoch ||
            cancellation.isCancelled ||
            _conversationId != conversationId) {
          return;
        }
        debugPrint('Conversation event stream reconnect: $error');
      } finally {
        if (identical(_eventCancellation, cancellation)) {
          _eventCancellation = null;
        }
        if (!_disposed &&
            epoch == _runtimeEpoch &&
            _conversationId == conversationId) {
          _automationStateUncertain = true;
          notifyListeners();
        }
      }
      if (_disposed ||
          epoch != _runtimeEpoch ||
          _conversationId != conversationId) {
        return;
      }
      await Future<void>.delayed(const Duration(milliseconds: 900));
    }
  }

  void _handleAppliedEvent(AgentUIEvent event) {
    _automationStateUncertain = false;
    final type = event.type;
    if (type == 'workbench.open') {
      unawaited(WorkbenchMobileEvents.ingest(event));
    }
    if (type == 'approval.requested') {
      final approvalId = (event.payload['approvalId'] ?? '').toString().trim();
      if (approvalId.isNotEmpty) {
        _pendingApprovals[approvalId] = <String, dynamic>{
          'id': approvalId,
          'conversationId': event.conversationId,
          'turnId': event.turnId,
          'toolCallId': event.callId,
          'toolName':
              event.payload['tool'] ?? event.payload['toolName'] ?? '工具调用',
          'arguments': event.payload['arguments'] ?? '',
          'riskLevel':
              event.payload['risk'] ?? event.payload['riskLevel'] ?? '',
          'expiresAt': event.payload['expiresAt'] ?? '',
        };
      }
    } else if (type == 'approval.approved' ||
        type == 'approval.denied' ||
        type == 'approval.expired') {
      final approvalId = (event.payload['approvalId'] ?? '').toString().trim();
      if (approvalId.isNotEmpty) _pendingApprovals.remove(approvalId);
    }
    if (type == 'turn.queued' || type == 'turn.started') {
      _sending = true;
      _conversationUpdateEpoch++;
    }
    if (type == 'turn.waiting' && event.status == 'needs_reconciliation') {
      _sending = false;
    }
    final terminal =
        type == 'turn.completed' ||
        type == 'turn.failed' ||
        type == 'turn.interrupted';
    if (terminal) {
      _sending = false;
      _conversationUpdateEpoch++;
      _streamScheduler.schedule(_flushStreamingProjection);
      _streamScheduler.flush();
      final id = _conversationId?.trim() ?? '';
      if (id.isNotEmpty) {
        unawaited(
          _chatService
              .markConversationRead(id, characterId: _characterId)
              .catchError((Object _) {}),
        );
        unawaited(_refreshSnapshot(id));
      }
      return;
    }
    _streamScheduler.schedule(_flushStreamingProjection);
  }

  void _flushStreamingProjection() {
    if (_disposed) return;
    _projectTurns();
    notifyListeners();
  }

  Future<void> resolveApproval(String approvalId, bool approved) async {
    final id = approvalId.trim();
    final conversationId = _conversationId?.trim() ?? '';
    final approval = _pendingApprovals[id];
    final turnId = (approval?['turnId'] ?? '').toString().trim();
    if (id.isEmpty || conversationId.isEmpty || turnId.isEmpty) {
      throw StateError('审批请求缺少 Conversation 或 Turn 绑定');
    }
    await _chatService.resolveTurnApproval(
      conversationId,
      turnId,
      id,
      approved,
    );
  }

  void _projectTurns() {
    final turns = _agentReducer.turns;
    final activeRenderIds = <String>{};
    for (final turn in turns) {
      final renderId = _assistantRenderIdForTurn(turn);
      activeRenderIds.add(renderId);
      AssistantTurnItemDto? finalItem;
      for (final item in turn.items.reversed) {
        if (item.type == 'text' && item.messageId.trim().isNotEmpty) {
          finalItem = item;
          break;
        }
      }
      if (finalItem != null) {
        final persisted = _messages.findById(finalItem.messageId);
        if (persisted != null && persisted.role == MessageRole.assistant) {
          _messages.upsert(
            _cloneMessage(
              persisted,
              renderId: renderId,
              assistantTurn: turn,
              status: _isTerminalTurn(turn.status)
                  ? _statusForDto(turn.status)
                  : persisted.status,
              citationSnapshot: projectCitationSnapshot(
                persisted.citationSnapshot,
                finalItem,
              ),
            ),
          );
          continue;
        }
      }

      final existing = _messages.findByRenderId(
        renderId,
        role: MessageRole.assistant,
      );
      final text = turn.items
          .where((item) => item.type == 'text')
          .map((item) => item.content)
          .join();
      final reasoningItems = turn.items
          .where((item) => item.type == 'reasoning')
          .toList(growable: false);
      final reasoning = reasoningItems.map((item) => item.content).join();
      final reasoningDurationMs = reasoningItems.fold<int>(
        0,
        (total, item) => total + item.durationMs,
      );
      final terminal = _isTerminalTurn(turn.status);
      final status = switch (turn.status.toLowerCase()) {
        'queued' || 'pending' => MessageStatus.queued,
        'failed' => MessageStatus.error,
        'interrupted' => MessageStatus.interrupted,
        'completed' => MessageStatus.delivered,
        _ => MessageStatus.streaming,
      };
      if (terminal &&
          text.isEmpty &&
          turn.items.isEmpty &&
          status != MessageStatus.error) {
        if (existing != null) _messages.remove(existing);
        continue;
      }
      final characterId = turn.characterId.trim().isNotEmpty
          ? turn.characterId
          : (existing?.characterId ?? _characterId ?? '');
      _messages.upsert(
        ChatMessage(
          id: existing?.id ?? renderId,
          sourceOwnerId: existing?.sourceOwnerId ?? '',
          sourceConversationId: existing?.sourceConversationId ?? '',
          sourceScope: existing?.sourceScope,
          sourceRevision: existing?.sourceRevision,
          renderId: renderId,
          characterId: characterId,
          role: MessageRole.assistant,
          type: MessageType.text,
          content: text,
          reasoningContent: reasoning,
          reasoningDurationMs: reasoningDurationMs,
          time:
              existing?.time ??
              DateTime.tryParse(turn.createdAt) ??
              DateTime.now(),
          status: status,
          assistantTurn: turn,
          citationSnapshot: projectCitationSnapshot(
            existing?.citationSnapshot ?? const <Map<String, dynamic>>[],
            finalItem,
          ),
        ),
      );
    }
    _messages.removeWhere(
      (message) =>
          _isLiveAssistantMessage(message) &&
          !activeRenderIds.contains(message.renderId),
    );
  }

  ChatMessage _messageFromDto(MessageDto dto) {
    final role = _roleFor(dto.role);
    final type = _typeForDto(dto, null);
    final task = type == MessageType.agentTask
        ? _agentTaskPayload(dto.content)
        : const <String, dynamic>{};
    final altText = dto.altText?.trim() ?? '';
    return ChatMessage(
      id: dto.id,
      sourceOwnerId: dto.sourceOwnerId,
      sourceConversationId: dto.sourceConversationId,
      sourceScope: dto.sourceScope,
      sourceRevision: dto.sourceRevision,
      renderId: dto.requestId.trim().isEmpty
          ? dto.id
          : role == MessageRole.assistant
          ? _assistantRenderId(dto.requestId)
          : dto.requestId.trim(),
      characterId: dto.characterId,
      characterName: dto.characterName,
      characterAvatar: dto.characterAvatar,
      groupRoundId: dto.groupRoundId,
      deliveryId: dto.deliveryId,
      role: role,
      type: type,
      content: dto.content,
      reasoningContent: dto.reasoningContent,
      reasoningDurationMs: dto.reasoningDurationMs,
      time: DateTime.tryParse(dto.createdAt) ?? DateTime.now(),
      sequence: dto.sequence > 0 ? dto.sequence : null,
      status: _statusForDto(dto.status),
      agentTaskId: _firstString(task, const <String>[
        'taskRunId',
        'task_run_id',
        'runId',
      ]),
      agentTaskTitle: _firstString(task, const <String>[
        'title',
        'taskTitle',
        'taskDefinitionId',
      ]),
      agentTaskSteps: _stringList(task['steps']),
      agentTaskProgress: _progressInt(task),
      agentTaskElapsed: _firstString(task, const <String>[
        'elapsed',
        'elapsedTime',
      ]),
      fileName: dto.fileName.isNotEmpty
          ? dto.fileName
          : type == MessageType.file
          ? altText.isNotEmpty
                ? altText
                : _fileNameFromContent(dto.content)
          : null,
      resourceUri: _resourceForDto(dto, null),
      fileSizeKB: dto.fileSizeBytes > 0
          ? (dto.fileSizeBytes / 1024).ceil()
          : null,
      mediaUrl: _resourceForDto(dto, null),
      mimeType: dto.mimeType.isNotEmpty
          ? dto.mimeType
          : dto.imageUrl.isNotEmpty
          ? 'image/*'
          : dto.videoUrl.isNotEmpty
          ? 'video/*'
          : dto.audioUrl.isNotEmpty
          ? 'audio/*'
          : null,
      durationMs: dto.audioDuration > 0
          ? (dto.audioDuration * 1000).round()
          : null,
      replyToMessageId: dto.replyToMessageId,
      replyToExcerpt: dto.replyToExcerpt,
      citationSnapshot: dto.citationSnapshot,
    );
  }

  Future<bool> loadOlderMessages() async {
    final conv = _conversationId?.trim() ?? '';
    if (conv.isEmpty || _loadingOlderMessages || !hasMoreHistory) return false;
    _loadingOlderMessages = true;
    notifyListeners();
    var changed = false;
    try {
      if (!await _chatService.owned.refresh())
        throw StateError('设备对话服务尚未就绪，请恢复连接后加载历史记录');
      if (_chatService.owned.hasMore(conv, characterId: _characterId)) {
        final result = await _chatService.owned.query(
          conv,
          characterId: _characterId,
          older: true,
        );
        if (_disposed || _conversationId != conv) return false;
        for (final dto in _chatService.owned.messages(result)) {
          changed = _messages.upsert(_messageFromDto(dto)) || changed;
        }
      }
      _hasMoreMessageHistory = _chatService.owned.hasMore(
        conv,
        characterId: _characterId,
      );
      if (_hasMoreTurnHistory) {
        final page = await _chatService.getAssistantTurnPage(
          conv,
          cursor: _turnHistoryCursor,
        );
        if (_disposed || _conversationId != conv) return false;
        _agentReducer.mergeTurns(page.items);
        _turnHistoryCursor = page.nextCursor;
        _hasMoreTurnHistory = page.hasMore;
        changed = changed || page.items.isNotEmpty;
        _projectTurns();
      }
      if (changed) notifyListeners();
      return changed;
    } finally {
      if (!_disposed) {
        _loadingOlderMessages = false;
        notifyListeners();
      }
    }
  }

  bool canRetryMessage(int index) {
    final list = _messages.messages;
    if (_sending || index < 0 || index >= list.length) return false;
    final message = list[index];
    final turn = message.assistantTurn;
    if (turn != null) {
      final status = turn.status.toLowerCase();
      if (status == 'failed' || status == 'interrupted') {
        return true;
      }
    }
    return message.role == MessageRole.user &&
        message.status == MessageStatus.error;
  }

  Future<void> retryMessage(int index) async {
    if (!canRetryMessage(index)) return;
    final message = _messages.messages[index];
    final turn = message.assistantTurn;
    final conv = _conversationId?.trim() ?? '';
    if (turn != null && conv.isNotEmpty) {
      try {
        await _appearancePreferences?.init();
        await _chatService.retryTurn(
          conv,
          turn.id,
          messageStyle: _appearancePreferences?.messageStyle.name ?? 'flow',
        );
        _sending = true;
        _lastError = null;
        _connectEventStream(conv);
        notifyListeners();
      } catch (error) {
        _lastError = error;
        notifyListeners();
      }
      return;
    }
    _messages.remove(message);
    notifyListeners();
    switch (message.type) {
      case MessageType.image:
        if (message.resourceUri == null || message.mediaUrl == null) {
          return sendText(message.content);
        }
        return sendImage(
          resourceUri: message.resourceUri!,
          displayUrl: message.mediaUrl!,
          fileName: message.fileName ?? 'image',
          mimeType: message.mimeType ?? 'image/*',
          text: message.content == '[图片]' ? '' : message.content,
        );
      case MessageType.video:
        if (message.resourceUri == null || message.mediaUrl == null) {
          return sendText(message.content);
        }
        return sendVideo(
          resourceUri: message.resourceUri!,
          displayUrl: message.mediaUrl!,
          fileName: message.fileName ?? 'video',
          mimeType: message.mimeType ?? 'video/*',
          durationMs: message.durationMs ?? 0,
          text: message.content == '[视频]' ? '' : message.content,
        );
      case MessageType.audio:
        if (message.resourceUri == null || message.mediaUrl == null) {
          return sendText(message.content);
        }
        return sendVoice(
          resourceUri: message.resourceUri!,
          displayUrl: message.mediaUrl!,
          fileName: message.fileName ?? 'audio',
          mimeType: message.mimeType ?? 'audio/*',
          durationMs: message.durationMs ?? 0,
          text: message.content == '[语音]' ? '' : message.content,
        );
      case MessageType.file:
        if (message.resourceUri == null) return sendText(message.content);
        return sendFile(
          resourceUri: message.resourceUri!,
          fileName: message.fileName ?? 'file',
          sizeBytes: (message.fileSizeKB ?? 0) * 1024,
          mimeType: message.mimeType ?? 'application/octet-stream',
        );
      case MessageType.code:
      case MessageType.emote:
      case MessageType.text:
      case MessageType.agentTask:
      case MessageType.toolCall:
      case MessageType.systemNotice:
        return sendText(message.content);
    }
  }

  Future<void> stop() async {
    if (_ownedRequestId.isNotEmpty) {
      try {
        await _chatService.owned.interrupt();
      } catch (error) {
        _lastError = error;
        notifyListeners();
      }
      return;
    }
    final conv = _conversationId?.trim() ?? '';
    final turnId = _agentReducer.activeTurnId.trim();
    if (!_sending || conv.isEmpty || turnId.isEmpty) return;
    try {
      await _chatService.interruptTurn(conv, turnId);
    } catch (error) {
      _lastError = error;
      notifyListeners();
    }
  }

  Future<void> steer(String content) async {
    final value = content.trim();
    final conv = _conversationId?.trim() ?? '';
    final turnId = _agentReducer.activeTurnId.trim();
    if (value.isEmpty || conv.isEmpty || turnId.isEmpty) return;
    await _chatService.steerTurn(conv, turnId, value);
  }

  Future<void> deleteMessage(String messageId, {String? sourceOwnerId}) async {
    final conversationId = _conversationId;
    final epoch = _runtimeEpoch;
    final id = messageId.trim();
    if (id.isEmpty || id.startsWith('turn:')) return;
    final existing = _messages.findById(id, sourceOwnerId: sourceOwnerId);
    if (existing == null) throw StateError('消息来源不唯一或已变化，请重新选择消息');
    await _chatService.deleteMessage(
      id,
      expectedScope: existing.sourceScope,
      expectedOwnerId: existing.sourceOwnerId,
      expectedRevision: existing.sourceRevision,
    );
    if (_disposed ||
        _runtimeEpoch != epoch ||
        _conversationId != conversationId ||
        !identical(
          _messages.findById(id, sourceOwnerId: existing.sourceOwnerId),
          existing,
        ))
      return;
    if (_messages.remove(existing)) notifyListeners();
  }

  Future<void> editMessage(
    String messageId,
    String content, {
    String? sourceOwnerId,
  }) async {
    final conversationId = _conversationId;
    final epoch = _runtimeEpoch;
    final id = messageId.trim();
    final value = content.trim();
    if (id.isEmpty || value.isEmpty || id.startsWith('turn:')) return;
    final existing = _messages.findById(id, sourceOwnerId: sourceOwnerId);
    if (existing == null) throw StateError('消息来源不唯一或已变化，请重新选择消息');
    await _chatService.updateMessage(
      id,
      value,
      expectedScope: existing.sourceScope,
      expectedOwnerId: existing.sourceOwnerId,
      expectedRevision: existing.sourceRevision,
    );
    if (_disposed ||
        _runtimeEpoch != epoch ||
        _conversationId != conversationId ||
        !identical(
          _messages.findById(id, sourceOwnerId: existing.sourceOwnerId),
          existing,
        ))
      return;
    _messages.upsert(
      _cloneMessage(
        existing,
        content: value,
        sourceRevision: existing.sourceRevision == null
            ? null
            : existing.sourceRevision! + 1,
      ),
    );
    notifyListeners();
  }

  Future<void> openConversation(
    String conversationId, {
    String? characterId,
  }) async {
    final id = conversationId.trim();
    if (id.isEmpty) return;
    _characterId = characterId?.trim().isEmpty == true
        ? null
        : characterId?.trim();
    if (_conversationId != id) {
      _disconnectRuntime();
      _conversationId = id;
      _conversationTitle = '';
      _messages.clear();
      _agentReducer.reset(
        turns: const <AssistantTurnDto>[],
        lastEventSequence: 0,
      );
      _resetHistory();
      _lastError = null;
      _sending = false;
      notifyListeners();
    }
    try {
      if (await _chatService.owned.refresh()) {
        final result = await _chatService.owned.query(
          id,
          characterId: _characterId,
        );
        if (_disposed || _conversationId != id) return;
        final snapshotData = result['snapshot'];
        _group = snapshotData is Map && snapshotData['group'] is Map
            ? OwnedGroupState.fromJson(
                Map<String, dynamic>.from(snapshotData['group'] as Map),
              )
            : null;
        _groupMentions = <Map<String, dynamic>>[];
        _messages.clear();
        for (final message in _chatService.owned.messages(result)) {
          _messages.upsert(_messageFromDto(message));
        }
        final scope = result['executionScope'];
        if (scope is Map) {
          _realtimeScope = Map.unmodifiable(Map<String, dynamic>.from(scope));
          _characterId = scope['roleId']?.toString();
          _conversationCoreId = scope['coreId']?.toString() ?? '';
        }
        _hasMoreMessageHistory = _chatService.owned.hasMore(
          id,
          characterId: _characterId,
        );
        _hasMoreTurnHistory = false;
        _sending = false;
        await _chatService
            .markConversationRead(id, characterId: _characterId)
            .catchError((Object _) {});
        await _refreshSnapshot(id);
        _connectEventStream(id);
        notifyListeners();
        return;
      }
      throw StateError('设备对话服务尚未就绪，请恢复连接后加载对话');
    } catch (error) {
      _lastError = error;
      notifyListeners();
    }
  }

  Future<bool> createRealtimeConversation(
    String? characterId, {
    String projectId = '',
  }) async {
    try {
      ConversationDto? conversation;
      if (await _chatService.owned.refresh()) {
        final context = await _chatService.owned.query(
          '',
          characterId: characterId,
        );
        if (context['executionScope'] is! Map) throw StateError('通话缺少原角色权限范围');
        _realtimeScope = Map.unmodifiable(
          Map<String, dynamic>.from(context['executionScope'] as Map),
        );
        conversation = ConversationDto.fromJson({
          'id': ownedRealtimeRequestId(),
          'characterId': characterId ?? '',
          'title': '实时通话',
        });
      } else {
        throw StateError('设备对话服务尚未就绪，无法发起通话');
      }
      _disconnectRuntime();
      _conversationId = conversation.id;
      _conversationTitle = conversation.title;
      _characterId = characterId;
      _messages.clear();
      _agentReducer.reset(
        turns: const <AssistantTurnDto>[],
        lastEventSequence: 0,
      );
      _resetHistory();
      _lastError = null;
      _sending = false;
      _modelConfigId = conversation.modelConfigId;
      _reasoningEffort = conversation.reasoningEffort.trim().isEmpty
          ? 'high'
          : conversation.reasoningEffort.trim();
      _reasoningEnabled = conversation.reasoningEnabled != 0;
      _permissionMode = normalizeChatPermissionMode(
        _permissionPreferences?.mode,
      );
      _conversationUpdateEpoch++;
      _connectEventStream(conversation.id);
      notifyListeners();
      return true;
    } catch (error) {
      _lastError = error;
      notifyListeners();
      return false;
    }
  }

  void clear({bool addSystemNotice = false}) {
    _messages.clear();
    if (addSystemNotice) {
      _messages.upsert(
        ChatMessage(
          id: _localId('sys'),
          role: MessageRole.system,
          type: MessageType.systemNotice,
          content: '聊天记录已从当前界面清空',
          time: DateTime.now(),
          sequence: _nextLocalSequence(),
        ),
      );
    }
    notifyListeners();
  }

  void startDraft({ConversationWorkspaceDto? workspace}) {
    _conversationCoreId = '';
    _disconnectRuntime();
    _draftEpoch++;
    _conversationId = null;
    _conversationTitle = '';
    _messages.clear();
    _agentReducer.reset(
      turns: const <AssistantTurnDto>[],
      lastEventSequence: 0,
    );
    _pendingApprovals.clear();
    _resetHistory();
    _lastError = null;
    _sending = false;
    _workspace = workspace;
    _permissionMode = normalizeChatPermissionMode(_permissionPreferences?.mode);
    notifyListeners();
  }

  void _disconnectRuntime() {
    if (_ownedRequestId.isNotEmpty)
      _chatService.owned.stopLocal('当前对话已切换，回复已中断。');
    _ownedRequestId = '';
    _runtimeEpoch++;
    _eventCancellation?.cancel('runtime disconnected');
    _eventCancellation = null;
  }

  void _resetHistory() {
    _hasMoreMessageHistory = false;
    _hasMoreTurnHistory = false;
    _turnHistoryCursor = '';
    _loadingOlderMessages = false;
  }

  ChatMessage _cloneMessage(
    ChatMessage message, {
    String? sourceOwnerId,
    String? sourceConversationId,
    String? characterId,
    String? characterName,
    String? characterAvatar,
    String? groupRoundId,
    String? deliveryId,
    Map<String, dynamic>? sourceScope,
    int? sourceRevision,
    String? id,
    String? renderId,
    String? content,
    int? sequence,
    MessageStatus? status,
    AssistantTurnDto? assistantTurn,
    List<Map<String, dynamic>>? citationSnapshot,
  }) {
    return ChatMessage(
      id: id ?? message.id,
      sourceOwnerId: sourceOwnerId ?? message.sourceOwnerId,
      sourceConversationId:
          sourceConversationId ?? message.sourceConversationId,
      sourceScope: sourceScope ?? message.sourceScope,
      sourceRevision: sourceRevision ?? message.sourceRevision,
      renderId: renderId ?? message.renderId,
      characterId: characterId ?? message.characterId,
      characterName: characterName ?? message.characterName,
      characterAvatar: characterAvatar ?? message.characterAvatar,
      groupRoundId: groupRoundId ?? message.groupRoundId,
      deliveryId: deliveryId ?? message.deliveryId,
      role: message.role,
      type: message.type,
      content: content ?? message.content,
      reasoningContent: message.reasoningContent,
      reasoningComplete: message.reasoningComplete,
      outputComplete: message.outputComplete,
      reasoningDurationMs: message.reasoningDurationMs,
      time: message.time,
      sequence: sequence ?? message.sequence,
      status: status ?? message.status,
      agentTaskId: message.agentTaskId,
      agentTaskTitle: message.agentTaskTitle,
      agentTaskSteps: message.agentTaskSteps,
      agentTaskProgress: message.agentTaskProgress,
      agentTaskElapsed: message.agentTaskElapsed,
      fileName: message.fileName,
      fileSizeKB: message.fileSizeKB,
      resourceUri: message.resourceUri,
      mediaUrl: message.mediaUrl,
      mimeType: message.mimeType,
      durationMs: message.durationMs,
      toolName: message.toolName,
      toolResult: message.toolResult,
      replyToMessageId: message.replyToMessageId,
      replyToExcerpt: message.replyToExcerpt,
      assistantTurn: assistantTurn ?? message.assistantTurn,
      citationSnapshot: citationSnapshot ?? message.citationSnapshot,
    );
  }

  Map<String, dynamic> _agentTaskPayload(String content) {
    final value = content.trim();
    if (value.isEmpty || (!value.startsWith('{') && !value.startsWith('['))) {
      return const <String, dynamic>{};
    }
    try {
      final decoded = jsonDecode(value);
      if (decoded is Map) return Map<String, dynamic>.from(decoded);
      if (decoded is List && decoded.isNotEmpty && decoded.first is Map) {
        return Map<String, dynamic>.from(decoded.first as Map);
      }
    } catch (_) {}
    return const <String, dynamic>{};
  }

  String? _firstString(Map<String, dynamic> source, List<String> keys) {
    for (final key in keys) {
      final value = (source[key] ?? '').toString().trim();
      if (value.isNotEmpty) return value;
    }
    return null;
  }

  List<String>? _stringList(dynamic value) {
    if (value is! List) return null;
    final result = value
        .map((item) {
          if (item is Map) {
            return (item['title'] ?? item['name'] ?? item['label'] ?? '')
                .toString()
                .trim();
          }
          return item.toString().trim();
        })
        .where((item) => item.isNotEmpty)
        .toList(growable: false);
    return result.isEmpty ? null : result;
  }

  int? _progressInt(Map<String, dynamic> source) {
    final direct = source['percentage'] ?? source['progress'];
    if (direct is num) return direct.round().clamp(0, 100);
    if (direct is Map) {
      final percentage = direct['percentage'];
      if (percentage is num) return percentage.round().clamp(0, 100);
      final current = direct['current'];
      final total = direct['total'];
      if (current is num && total is num && total > 0) {
        return (current.toDouble() / total.toDouble() * 100).round().clamp(
          0,
          100,
        );
      }
    }
    return null;
  }

  MessageType _typeForDto(MessageDto dto, ChatMessage? existing) {
    final rawType = dto.msgType.trim().toLowerCase().replaceAll('-', '_');
    switch (rawType) {
      case 'image':
        return MessageType.image;
      case 'video':
        return MessageType.video;
      case 'audio':
      case 'voice':
        return MessageType.audio;
      case 'emote':
      case 'emoji':
        return MessageType.emote;
      case 'code':
        return MessageType.code;
      case 'file':
        return MessageType.file;
      case 'agent_task':
      case 'agenttask':
        return MessageType.agentTask;
      case 'tool_call':
      case 'toolcall':
      case 'tool':
        return MessageType.toolCall;
      case 'system_notice':
      case 'systemnotice':
        return MessageType.systemNotice;
    }
    if (existing != null && existing.type != MessageType.text) {
      return existing.type;
    }
    if (dto.imageUrl.isNotEmpty) return MessageType.image;
    if (dto.videoUrl.isNotEmpty) return MessageType.video;
    if (dto.audioUrl.isNotEmpty) return MessageType.audio;
    if ((dto.emoteId ?? '').isNotEmpty) return MessageType.emote;
    if (dto.content.startsWith('```') && dto.content.endsWith('```')) {
      return MessageType.code;
    }
    return MessageType.text;
  }

  String? _resourceForDto(MessageDto dto, ChatMessage? existing) {
    if (dto.resourceUri.isNotEmpty) return dto.resourceUri;
    if (dto.imageUrl.isNotEmpty) return dto.imageUrl;
    if (dto.videoUrl.isNotEmpty) return dto.videoUrl;
    if (dto.audioUrl.isNotEmpty) return dto.audioUrl;
    if (dto.originalAssetReference.isNotEmpty) {
      return dto.originalAssetReference;
    }
    if (dto.fallbackAssetReference.isNotEmpty) {
      return dto.fallbackAssetReference;
    }
    final contentUri = RegExp(
      r'amitia://artifacts/[A-Za-z0-9._-]+',
    ).firstMatch(dto.content)?.group(0);
    if (contentUri != null && contentUri.isNotEmpty) return contentUri;
    return existing?.resourceUri;
  }

  String? _fileNameFromContent(String content) {
    final firstLine = content
        .split('\n')
        .firstWhere((line) => line.trim().isNotEmpty, orElse: () => '')
        .trim();
    if (!firstLine.startsWith('[文件]')) return null;
    final value = firstLine.substring('[文件]'.length).trim();
    return value.isEmpty ? null : value;
  }

  MessageRole _roleFor(String role) {
    switch (role) {
      case 'user':
        return MessageRole.user;
      case 'system':
        return MessageRole.system;
      default:
        return MessageRole.assistant;
    }
  }

  MessageStatus _statusForDto(String value) {
    final status = value.trim().toLowerCase();
    return switch (status) {
      'queued' || 'pending' => MessageStatus.queued,
      'sending' => MessageStatus.sending,
      'streaming' ||
      'generating' ||
      'collecting' ||
      'processing' => MessageStatus.streaming,
      'interrupted' || 'paused' => MessageStatus.interrupted,
      'failed' || 'error' => MessageStatus.error,
      _ => MessageStatus.delivered,
    };
  }

  bool _isTerminalTurn(String status) {
    final value = status.trim().toLowerCase();
    return value == 'completed' || value == 'failed' || value == 'interrupted';
  }

  String _localId(String prefix) =>
      '$prefix${DateTime.now().microsecondsSinceEpoch}';

  int _nextLocalSequence() {
    var sequence = 0;
    for (final message in _messages.messages) {
      final current = message.sequence ?? 0;
      if (current > sequence) sequence = current;
    }
    return sequence + 1;
  }

  String _assistantRenderId(String requestId) {
    final value = requestId.trim();
    return value.isEmpty ? '' : 'request:$value';
  }

  String _assistantRenderIdForTurn(AssistantTurnDto turn) {
    final renderId = _assistantRenderId(turn.requestId);
    return renderId.isEmpty ? 'turn:${turn.id}' : renderId;
  }

  bool _isLiveAssistantMessage(ChatMessage message) {
    if (message.role != MessageRole.assistant) return false;
    return message.id.startsWith('request:') || message.id.startsWith('turn:');
  }

  Map<String, dynamic> serializeMessage(ChatMessage message) =>
      <String, dynamic>{
        'id': message.id,
        'renderId': message.renderId,
        'characterId': message.characterId,
        'role': message.role.name,
        'type': message.type.name,
        'content': message.content,
        'reasoningContent': message.reasoningContent,
        'reasoningComplete': message.reasoningComplete,
        'outputComplete': message.outputComplete,
        if (message.reasoningDurationMs > 0)
          'reasoningDurationMs': message.reasoningDurationMs,
        'time': message.time.toIso8601String(),
        if (message.sequence != null) 'sequence': message.sequence,
        'status': message.status.name,
        if ((message.agentTaskId ?? '').isNotEmpty)
          'agentTaskId': message.agentTaskId,
        if (message.fileName != null) 'fileName': message.fileName,
        if (message.fileSizeKB != null) 'fileSizeKB': message.fileSizeKB,
        if (message.resourceUri != null) 'resourceUri': message.resourceUri,
        if (message.mediaUrl != null) 'mediaUrl': message.mediaUrl,
        if (message.mimeType != null) 'mimeType': message.mimeType,
        if (message.durationMs != null) 'durationMs': message.durationMs,
        if (message.toolName != null) 'toolName': message.toolName,
        if (message.toolResult != null) 'toolResult': message.toolResult,
        if ((message.replyToMessageId ?? '').isNotEmpty)
          'replyToMessageId': message.replyToMessageId,
        if ((message.replyToExcerpt ?? '').isNotEmpty)
          'replyToExcerpt': message.replyToExcerpt,
      };

  @override
  void dispose() {
    _ownedPoll?.cancel();
    _disposed = true;
    _disconnectRuntime();
    _streamScheduler.dispose();
    super.dispose();
  }
}

final conversationRuntimeControllerProvider =
    ChangeNotifierProvider<ConversationRuntimeController>((ref) {
      final controller = ConversationRuntimeController(
        ref.read(chatServiceProvider),
        appearancePreferences: ref.read(
          chatAppearancePreferencesProvider.notifier,
        ),
        permissionPreferences: ref.read(
          chatPermissionPreferencesProvider.notifier,
        ),
      );
      ref.onDispose(controller.dispose);
      return controller;
    });
