import 'dart:async';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_motion.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_message.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/artifact/artifact_model.dart';
import '../../../../core/artifact/artifact_providers.dart';
import '../../../../core/artifact/artifact_service.dart';
import '../../../../core/ui_runtime/ui_provider.dart';
import '../../../../core/ui_runtime/ui_provider_host.dart';
import '../../../../core/ui_runtime/ui_runtime_controller.dart';
import '../../../../core/ui_runtime/ui_message_renderer_registry.dart';
import '../../../../core/ui_runtime/conversation_ui_contract.dart';
import '../../../../core/ui_runtime/mobile_extension_slot.dart';
import '../../../../core/ui_runtime/mobile_conversation_projection.dart';
import '../../../../core/ui_runtime/mobile_dynamic_runtime.dart';
import '../../runtime/conversation_runtime_controller.dart';
import '../../../../shared/models/models.dart';
import 'realtime_voice_call_sheet.dart';
import 'call_placeholder_page.dart';

class ChatPage extends ConsumerStatefulWidget {
  const ChatPage({super.key, this.initialConversationId, this.initialCharacterId});

  final String? initialConversationId;
  final String? initialCharacterId;

  @override
  ConsumerState<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends ConsumerState<ChatPage> {
  final _scrollController = ScrollController();
  late final ConversationRuntimeController _runtime;
  Map<String, dynamic>? _cachedProviderContext;
  List<ChatMessage>? _cachedMessagesForContext;
  Map<String, FutureOr<dynamic> Function(dynamic)>? _cachedProviderActions;
  Timer? _conversationEventRefreshTimer;

  @override
  void initState() {
    super.initState();
    _runtime = ConversationRuntimeController(ref.read(chatServiceProvider), ref.read(emoteServiceProvider));
    _runtime.addListener(_onRuntimeChanged);
    WidgetsBinding.instance.addPostFrameCallback((_) => _openInitialConversation());
  }

  Future<void> _openInitialConversation() async {
    final conversationId = widget.initialConversationId?.trim() ?? '';
    if (!mounted || conversationId.isEmpty) return;
    final characterId = widget.initialCharacterId?.trim() ?? '';
    if (characterId.isNotEmpty) {
      ref.read(currentCharacterIdProvider.notifier).state = characterId;
    }
    await _runtime.openConversation(
      conversationId,
      characterId: characterId.isEmpty ? null : characterId,
    );
  }

  void _onRuntimeChanged() {
    if (!mounted) return;
    _cachedProviderContext = null;
    _cachedMessagesForContext = null;
    _cachedProviderActions = null;
    final conversationId = _runtime.conversationId?.trim() ?? '';
    _conversationEventRefreshTimer?.cancel();
    if (conversationId.isNotEmpty) {
      _conversationEventRefreshTimer = Timer(const Duration(milliseconds: 350), () {
        if (!mounted) return;
        ref.invalidate(conversationUIEventWindowProvider(conversationId));
      });
    }
    setState(() {});
    _scrollToBottom();
  }

  Map<String, dynamic> _buildProviderContext(String characterId, String characterName, String avatarInitial, String avatarColor) {
    final currentMessages = _runtime.messages;
    if (_cachedProviderContext != null && _cachedMessagesForContext != null && _cachedMessagesForContext!.length == currentMessages.length) {
      bool same = true;
      for (int i = 0; i < currentMessages.length; i++) {
        if (_cachedMessagesForContext![i].id != currentMessages[i].id ||
            _cachedMessagesForContext![i].status != currentMessages[i].status) {
          same = false;
          break;
        }
      }
      if (same) return _cachedProviderContext!;
    }
    final messagesMap = currentMessages.map(_providerMessage).toList(growable: false);
    _cachedMessagesForContext = List<ChatMessage>.from(currentMessages);
    _cachedProviderContext = <String, dynamic>{
      'route': '/chat',
      'character': {
        'id': characterId,
        'name': characterName,
        'avatarInitial': avatarInitial,
        'avatarColor': avatarColor,
      },
      'messages': messagesMap,
    };
    return _cachedProviderContext!;
  }

  @override
  void dispose() {
    _conversationEventRefreshTimer?.cancel();
    _runtime.removeListener(_onRuntimeChanged);
    _runtime.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: AppMotion.extended,
          curve: AppMotion.standardCurve,
        );
      }
    });
  }

  void _jumpToMessage(int index) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) return;
      final target = (index * 76.0).clamp(
        0.0,
        _scrollController.position.maxScrollExtent,
      ).toDouble();
      _scrollController.animateTo(
        target,
        duration: AppMotion.extended,
        curve: AppMotion.standardCurve,
      );
    });
  }

  void _retryMessage(int index) {
    _runtime.retryMessage(index);
  }





  void _onSend(String text) {
    _runtime.sendText(text);
  }

  Future<void> _pickAndSendFile() async {
    await _withArtifactUpload((service) async {
      final artifact = await service.pickAndUploadFile();
      switch (artifact.kind) {
        case ArtifactKind.image:
          await _sendImageArtifact(service, artifact);
          return;
        case ArtifactKind.video:
          await _sendVideoArtifact(service, artifact);
          return;
        case ArtifactKind.audio:
          await _sendAudioArtifact(service, artifact);
          return;
        case ArtifactKind.file:
          await _runtime.sendFile(
            resourceUri: artifact.resourceUri,
            fileName: artifact.filename,
            sizeBytes: artifact.sizeBytes,
            mimeType: artifact.mimeType,
          );
          return;
      }
    });
  }

  Future<void> _pickAndSendImage(bool camera) async {
    await _withArtifactUpload((service) async {
      final artifact = await service.pickAndUploadImage(
        source: camera ? ImageSource.camera : ImageSource.gallery,
      );
      await _sendImageArtifact(service, artifact);
    });
  }

  Future<void> _pickAndSendVideo(bool camera) async {
    await _withArtifactUpload((service) async {
      final artifact = await service.pickAndUploadVideo(
        source: camera ? ImageSource.camera : ImageSource.gallery,
      );
      await _sendVideoArtifact(service, artifact);
    });
  }

  Future<void> _pickAndSendAudio() async {
    await _withArtifactUpload((service) async {
      final artifact = await service.pickAndUploadAudio();
      await _sendAudioArtifact(service, artifact);
    });
  }

  Future<void> _sendImageArtifact(
    ArtifactService service,
    ArtifactMetadata artifact,
  ) {
    return _runtime.sendImage(
      resourceUri: artifact.resourceUri,
      displayUrl: service.contentUrl(artifact.id),
      fileName: artifact.filename,
      mimeType: artifact.mimeType,
    );
  }

  Future<void> _sendVideoArtifact(
    ArtifactService service,
    ArtifactMetadata artifact,
  ) {
    return _runtime.sendVideo(
      resourceUri: artifact.resourceUri,
      displayUrl: service.contentUrl(artifact.id),
      fileName: artifact.filename,
      mimeType: artifact.mimeType,
      durationMs: artifact.durationMs,
    );
  }

  Future<void> _sendAudioArtifact(
    ArtifactService service,
    ArtifactMetadata artifact,
  ) {
    return _runtime.sendVoice(
      resourceUri: artifact.resourceUri,
      displayUrl: service.contentUrl(artifact.id),
      fileName: artifact.filename,
      mimeType: artifact.mimeType,
      durationMs: artifact.durationMs,
    );
  }

  Future<void> _withArtifactUpload(
    Future<void> Function(ArtifactService service) action,
  ) async {
    try {
      final service = await ref.read(artifactServiceProvider.future);
      await action(service);
    } on ArtifactServiceException catch (error) {
      if (error.message != 'user_cancelled' && mounted) {
        amitiaSnackBar(context, '附件处理失败：${error.message}');
      }
    } catch (error) {
      if (mounted) {
        amitiaSnackBar(
          context,
          '附件处理失败：${error.toString().replaceFirst('Exception: ', '')}',
        );
      }
    }
  }

  void _onSendCode(String lang, String code) {
    _runtime.sendCode(lang, code);
  }

  void _onSendEmote(String emoteId, String displayText) {
    _runtime.sendEmote(emoteId, displayText);
  }

  bool _shouldShowAvatar(int index) {
    if (index < 0 || index >= _runtime.messages.length) return false;
    return _runtime.messages[index].type != MessageType.systemNotice;
  }

  AmitiaAgentActivity? _toolActivityForEvent(MobileConversationEvent event) {
    if (event.eventType != 'tool.invocation_completed') return null;
    final toolName = (event.payload['toolName'] ?? '').toString().trim();
    if (toolName.isEmpty) return null;
    return AmitiaAgentActivity(
      id: event.id,
      title: toolName,
      status: (event.payload['status'] ?? 'completed').toString(),
      errorCode: (event.payload['errorCode'] ?? '').toString().trim().isEmpty
          ? null
          : event.payload['errorCode'].toString().trim(),
      time: event.timestamp,
    );
  }

  ({Map<String, List<AmitiaAgentActivity>> byMessageId, List<AmitiaAgentActivity> unpaired})
      _projectAgentActivities(
    List<ChatMessage> messages,
    List<MobileConversationEvent> events,
  ) {
    final byMessageId = <String, List<AmitiaAgentActivity>>{};
    final unpaired = <AmitiaAgentActivity>[];
    for (final event in events) {
      final activity = _toolActivityForEvent(event);
      if (activity == null) continue;

      var latestUserIndex = -1;
      for (var i = 0; i < messages.length; i++) {
        final message = messages[i];
        if (message.role == MessageRole.user &&
            !message.time.isAfter(event.timestamp.add(const Duration(seconds: 2)))) {
          latestUserIndex = i;
        }
      }

      var nextUserIndex = messages.length;
      for (var i = latestUserIndex + 1; i < messages.length; i++) {
        if (messages[i].role == MessageRole.user) {
          nextUserIndex = i;
          break;
        }
      }

      ChatMessage? target;
      for (var i = latestUserIndex + 1; i < nextUserIndex; i++) {
        final message = messages[i];
        if (message.role != MessageRole.assistant ||
            message.type == MessageType.systemNotice) {
          continue;
        }
        if (!message.time.isBefore(event.timestamp.subtract(const Duration(seconds: 2)))) {
          target = message;
          break;
        }
      }

      if (target == null) {
        unpaired.add(activity);
      } else {
        byMessageId.putIfAbsent(target.id, () => <AmitiaAgentActivity>[]).add(activity);
      }
    }
    for (final activities in byMessageId.values) {
      activities.sort((a, b) => a.time.compareTo(b.time));
    }
    unpaired.sort((a, b) => a.time.compareTo(b.time));
    return (byMessageId: byMessageId, unpaired: unpaired);
  }

  Future<void> _showCallOptions(String characterId, String characterName) async {
    final mode = await showAmitiaActionSheet<String>(
      context,
      title: '发起通话',
      actions: const [
        AmitiaActionSheetItem(
          icon: Icons.videocam_outlined,
          label: '视频通话',
          value: 'video',
        ),
        AmitiaActionSheetItem(
          icon: Icons.call_outlined,
          label: '语音通话',
          value: 'voice',
        ),
        AmitiaActionSheetItem(
          icon: Icons.screen_share_outlined,
          label: '屏幕通话',
          value: 'screen',
        ),
      ],
    );
    if (!mounted || mode == null) return;
    switch (mode) {
      case 'voice':
        await _startRealtimeCall(characterId, characterName);
      case 'video':
        await showPlaceholderCallPage(
          context,
          mode: PlaceholderCallMode.video,
          characterName: characterName,
        );
      case 'screen':
        await showPlaceholderCallPage(
          context,
          mode: PlaceholderCallMode.screen,
          characterName: characterName,
        );
    }
  }

  Future<void> _startRealtimeCall(String characterId, String characterName) async {
    var conversationId = _runtime.conversationId;
    if (conversationId == null || conversationId.isEmpty) {
      try {
        final created = await _runtime.createConversation(characterId);
        if (!created) {
          if (mounted) amitiaSnackBar(context, '无法创建语音通话会话');
          return;
        }
        conversationId = _runtime.conversationId;
      } catch (error) {
        if (mounted) amitiaSnackBar(context, '创建会话失败：$error');
        return;
      }
    }
    if (!mounted || conversationId == null || conversationId.isEmpty) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      useSafeArea: false,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
      ),
      builder: (_) => RealtimeVoiceCallSheet(
        conversationId: conversationId!,
        characterName: characterName,
      ),
    );
  }

  void _showMessageSearch(BuildContext context) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) {
        return _MessageSearchSheet(
          messages: _runtime.messages,
          onJump: (index) {
            Navigator.pop(sheetCtx);
            _jumpToMessage(index);
          },
        );
      },
    );
  }

  Future<void> _clearCurrentConversation() async {
    final conversationId = _runtime.conversationId?.trim() ?? '';
    if (conversationId.isEmpty) {
      amitiaSnackBar(context, '当前还没有可清空的会话');
      return;
    }
    try {
      await ref.read(chatServiceProvider).deleteMessages(conversationId);
      _runtime.clear();
      ref.invalidate(conversationListProvider);
      if (mounted) amitiaSnackBar(context, '聊天记录已清空');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '清空失败：$error');
    }
  }

  Future<void> _exportCurrentConversation(String format) async {
    final conversationId = _runtime.conversationId?.trim() ?? '';
    if (conversationId.isEmpty) {
      amitiaSnackBar(context, '当前还没有可导出的会话');
      return;
    }
    try {
      final url = await ref.read(chatServiceProvider).exportConversation(
            conversationId,
            format: format,
          );
      if (url.isNotEmpty) {
        await Clipboard.setData(ClipboardData(text: url));
        if (mounted) amitiaSnackBar(context, '导出完成，资源地址已复制');
      } else if (mounted) {
        amitiaSnackBar(context, '导出完成');
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '导出失败：$error');
    }
  }

  void _showExportSheet(BuildContext context) {
    showAmitiaActionSheet<String>(
      context,
      title: '导出聊天记录',
      actions: const [
        AmitiaActionSheetItem(
          icon: Icons.description_outlined,
          label: 'Markdown',
          value: 'markdown',
        ),
        AmitiaActionSheetItem(
          icon: Icons.data_object_outlined,
          label: 'JSON',
          value: 'json',
        ),
      ],
    ).then((format) {
      if (format == null || !mounted) return;
      _exportCurrentConversation(format);
    });
  }

  void _showChatActionsSheet(BuildContext context) {
    showAmitiaActionSheet<int>(
      context,
      title: '当前对话',
      actions: const [
        AmitiaActionSheetItem(
          icon: Icons.person_outline,
          label: '查看角色详情',
          value: 0,
        ),
        AmitiaActionSheetItem(
          icon: Icons.search,
          label: '搜索当前会话',
          value: 1,
        ),
        AmitiaActionSheetItem(
          icon: Icons.file_download_outlined,
          label: '导出聊天记录',
          value: 2,
        ),
        AmitiaActionSheetItem(
          icon: Icons.cleaning_services_outlined,
          label: '清空聊天记录',
          value: 3,
          isDestructive: true,
        ),
      ],
    ).then((result) {
      if (result == null || !mounted) return;
      switch (result) {
        case 0:
          context.push(AppRoutes.character(ref.read(currentCharacterIdProvider)));
        case 1:
          _showMessageSearch(context);
        case 2:
          _showExportSheet(context);
        case 3:
          showAmitiaConfirmDialog(
            context,
            title: '清空聊天记录',
            message: '确定要清空当前聊天记录吗？此操作不可撤销。',
            confirmLabel: '清空',
            isDestructive: true,
          ).then((confirmed) {
            if (confirmed == true && mounted) {
              _clearCurrentConversation();
            }
          });
      }
    });
  }

  void _openDrawer(BuildContext context) {
    final scope = ShellDrawerScope.of(context);
    if (scope != null) {
      scope.openDrawer();
      return;
    }
    Scaffold.of(context).openDrawer();
  }

  Map<String, dynamic> _providerMessage(ChatMessage message) =>
      _runtime.serializeMessage(message);

  Map<String, FutureOr<dynamic> Function(dynamic)> _providerActions(String characterId) {
    return _cachedProviderActions ??= <String, FutureOr<dynamic> Function(dynamic)>{
      ConversationUIAction.send: (input) {
        final value = input is Map ? input['text'] : input;
        final text = value?.toString() ?? '';
        if (text.trim().isNotEmpty) _onSend(text);
        return null;
      },
      ConversationUIAction.retry: (input) {
        final id = input is Map ? input['messageId']?.toString() : input?.toString();
        final index = _runtime.messages.indexWhere((message) => message.id == id);
        if (index >= 0) _retryMessage(index);
        return null;
      },
      ConversationUIAction.regenerate: (input) {
        final id = input is Map ? input['messageId']?.toString() : input?.toString();
        return _runtime.regenerate(messageId: id);
      },
      ConversationUIAction.stop: (_) => _runtime.stop(),
      ConversationUIAction.delete: (input) {
        final id = input is Map ? input['messageId']?.toString() : input?.toString();
        if (id != null && id.isNotEmpty) _runtime.deleteMessage(id);
        return null;
      },
      ConversationUIAction.newConversation: (_) => _runtime.createConversation(characterId),
      ConversationUIAction.openDrawer: (_) => _openDrawer(context),
      ConversationUIAction.sendFile: (_) => _pickAndSendFile(),
      ConversationUIAction.sendImage: (input) {
        final camera = input is Map && input['source']?.toString() == 'camera';
        return _pickAndSendImage(camera);
      },
      ConversationUIAction.sendCode: (input) {
        final row = input is Map ? input : const <String, dynamic>{};
        final language = row['language']?.toString() ?? 'text';
        final code = row['code']?.toString() ?? '';
        if (code.isNotEmpty) _runtime.sendCode(language, code);
        return null;
      },
      ConversationUIAction.sendVoice: (_) => _pickAndSendAudio(),
      ConversationUIAction.sendEmote: (input) {
        final row = input is Map ? input : const <String, dynamic>{};
        final emoteId = row['emoteId']?.toString() ?? '';
        final displayText = row['displayText']?.toString() ?? row['name']?.toString() ?? '';
        if (emoteId.isNotEmpty) _runtime.sendEmote(emoteId, displayText);
        return null;
      },
    };
  }

  @override
  Widget build(BuildContext context) {
    final isAgentMode = ref.watch(isAgentModeProvider);
    final characterId = ref.watch(currentCharacterIdProvider);
    _runtime.setCharacterId(characterId);
    final charactersAsync = ref.watch(characterListProvider);

    final character = charactersAsync.when(
      loading: () => null,
      error: (_, __) => null,
      data: (characters) {
        return characters.where((c) => c.id == characterId).firstOrNull;
      },
    );

    final characterName = (character?.name ?? '').trim();
    final avatarInitial = characterName.isNotEmpty ? characterName.characters.first : 'A';
    const avatarColor = '#8A5728';
    final currentUser = ref.watch(currentUserProvider).valueOrNull;
    final userName = (currentUser?.username ?? '').trim().isEmpty
        ? '我'
        : currentUser!.username.trim();
    final userInitial = userName.characters.first;
    const userAvatarColor = '#5F6872';

    final providerContext = _buildProviderContext(characterId, characterName, avatarInitial, avatarColor);
    providerContext['user'] = {
      'name': userName,
      'avatarInitial': userInitial,
      'avatarColor': userAvatarColor,
    };
    providerContext['agentMode'] = isAgentMode;
    providerContext['conversationState'] = _runtime.state;
    providerContext['sending'] = _runtime.sending;
    providerContext['conversationId'] = _runtime.conversationId;
    final providerActions = _providerActions(characterId);

    final uiSnapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    bool externalProvider(String capability) {
      final provider = uiSnapshot?.resolve(capability);
      return provider != null && provider.enabled && !provider.builtin;
    }
    final hasSidebarProvider = externalProvider(UICapability.conversationSidebar);
    final hasOverlayProvider = externalProvider(UICapability.conversationOverlay);

    final conversationId = _runtime.conversationId?.trim() ?? '';
    final serializedMessages = _runtime.messages.map(_providerMessage).toList(growable: false);
    final durableConversationRecords = ref
            .watch(conversationUIEventWindowProvider(conversationId))
            .valueOrNull ??
        const <Map<String, dynamic>>[];
    final durableEvents = MobileConversationProjection.durableEvents(
      conversationId: conversationId,
      records: durableConversationRecords,
    );
    final agentActivityProjection = _projectAgentActivities(
      _runtime.messages,
      durableEvents,
    );
    DateTime? lastUserTime;
    for (final message in _runtime.messages) {
      if (message.role == MessageRole.user) lastUserTime = message.time;
    }
    final liveAgentActivities = lastUserTime == null
        ? const <AmitiaAgentActivity>[]
        : agentActivityProjection.unpaired
            .where((activity) => !activity.time.isBefore(lastUserTime!.subtract(const Duration(seconds: 2))))
            .toList(growable: false);
    final hasAssistantAfterLastUser = lastUserTime != null && _runtime.messages.any(
      (message) => message.role == MessageRole.assistant &&
          !message.time.isBefore(lastUserTime!),
    );
    final showLiveAgentProcess = _runtime.sending && !hasAssistantAfterLastUser;

    final runtimeSessionState = conversationId.isEmpty
        ? null
        : ref.watch(clientRuntimeSessionStateProvider(conversationId)).valueOrNull;
    final projectionContributions = uiSnapshot == null
        ? const <UIContributionSnapshotEntry>[]
        : <UIContributionSnapshotEntry>[
            ...uiSnapshot.contributionsForSlot('chat.conversation.node'),
            ...MobileDynamicRuntime.conversationNodeContributions(
              snapshot: uiSnapshot,
              sessionState: runtimeSessionState,
            ),
          ];
    final conversationNodes = MobileConversationProjection.assemble(
      events: MobileConversationProjection.mergeEvents([
        MobileConversationProjection.messageEvents(
          conversationId: conversationId,
          messages: serializedMessages,
        ),
        durableEvents,
      ]),
      contributions: projectionContributions,
    );
    final flowItems = <_MobileChatFlowItem>[
      for (var index = 0; index < _runtime.messages.length; index++)
        _MobileChatFlowItem.message(
          message: _runtime.messages[index],
          messageIndex: index,
          timestamp: _runtime.messages[index].time,
        ),
      for (final node in conversationNodes)
        _MobileChatFlowItem.node(node),
    ]..sort((left, right) {
      final order = MobileConversationProjection.compareTimeline(
        left.sequence, left.timestamp, right.sequence, right.timestamp,
      );
      if (order != 0) return order;
      if (left.isMessage != right.isMessage) return left.isMessage ? -1 : 1;
      return left.key.compareTo(right.key);
    });

    final builtinConversation = AmitiaScaffold(
      resizeToAvoidBottomInset: false,
      body: Stack(
        key: const ValueKey('ime-single-scaffold-20260805-0325'),
        children: [
          Positioned.fill(
            child: SafeArea(
              bottom: false,
              child: Column(
                children: [
                  Expanded(
                    child: UIProviderHost(
                      capability: UICapability.conversationMessages,
                      context: providerContext,
                      actions: providerActions,
                      fallback: Stack(
                        children: [
                          ListView.builder(
                          controller: _scrollController,
                          padding: const EdgeInsets.symmetric(vertical: 32),
                          itemCount: flowItems.length + (showLiveAgentProcess ? 1 : 0),
                          itemBuilder: (context, flowIndex) {
                            if (flowIndex >= flowItems.length) {
                              return RepaintBoundary(
                                child: AmitiaMessageBubble(
                                  message: ChatMessage(
                                    id: '__live_agent_process__',
                                    role: MessageRole.assistant,
                                    type: MessageType.text,
                                    content: '',
                                    time: DateTime.now(),
                                  ),
                                  showAvatar: true,
                                  avatarInitial: avatarInitial,
                                  avatarColor: avatarColor,
                                  characterName: characterName,
                                  userInitial: userInitial,
                                  userAvatarColor: userAvatarColor,
                                  userName: userName,
                                  agentActivities: liveAgentActivities,
                                  showThinking: true,
                                ),
                              );
                            }
                            final item = flowItems[flowIndex];
                            if (!item.isMessage) {
                              final node = item.node!;
                              return MobileExtensionSlot(
                                slotId: 'chat.conversation.node',
                                contributionId: node.contributionId,
                                context: {
                                  ...providerContext,
                                  'conversationNode': node.toJson(),
                                  'eventType': node.eventType,
                                },
                                actions: providerActions,
                              );
                            }

                            final index = item.messageIndex!;
                            final message = item.message!;
                            final isAgentTask = message.type == MessageType.agentTask;
                            final builtinMessage = RepaintBoundary(
                              child: AmitiaMessageBubble(
                                message: message,
                                showAvatar: _shouldShowAvatar(index),
                                avatarInitial: avatarInitial,
                                avatarColor: avatarColor,
                                characterName: characterName,
                                userInitial: userInitial,
                                userAvatarColor: userAvatarColor,
                                userName: userName,
                                agentActivities: agentActivityProjection.byMessageId[message.id] ??
                                    const <AmitiaAgentActivity>[],
                                onRetry: message.status == MessageStatus.error
                                    ? () => _retryMessage(index)
                                    : null,
                                onAgentTaskTap: isAgentTask
                                    ? () => context.push(AppRoutes.agent)
                                    : null,
                              ),
                            );
                            final messageRenderer = UIMessageRendererRegistry.resolve(
                              uiSnapshot,
                              messageType: message.type.name,
                              role: message.role.name,
                            );
                            final providerMessage = UIProviderHost(
                              capability: UICapability.conversationMessageRenderer,
                              providerId: messageRenderer?.providerId,
                              fallback: builtinMessage,
                              context: { ...providerContext, 'message': _providerMessage(message), 'messageIndex': index },
                              actions: providerActions,
                            );
                            return MobileExtensionSlot(
                              slotId: 'chat.message.renderer',
                              context: {
                                ...providerContext,
                                'messageId': message.id,
                                'messageType': message.type.name,
                                'message': _providerMessage(message),
                                'messageIndex': index,
                              },
                              actions: providerActions,
                              fallback: providerMessage,
                            );
                          },
                        ),
                        _ChatScrollFade(
                          alignment: Alignment.topCenter,
                          begin: Alignment.topCenter,
                          end: Alignment.bottomCenter,
                          color: context.backgroundPrimary,
                          height: _chatTopBarHeight,
                        ),
                        _ChatScrollFade(
                          alignment: Alignment.bottomCenter,
                          begin: Alignment.bottomCenter,
                          end: Alignment.topCenter,
                          color: context.backgroundPrimary,
                        ),
                        ],
                      ),
                    ),
                  ),
                  UIProviderHost(
                    capability: UICapability.conversationComposer,
                    context: providerContext,
                    actions: providerActions,
                    fallback: AmitiaChatInput(
                    onSend: _onSend,
                    recipientName: characterName,
                    isAgentMode: isAgentMode,
                    onAgentModeChanged: (value) {
                      ref.read(isAgentModeProvider.notifier).state = value;
                    },
                    onPickFile: _pickAndSendFile,
                    onPickImage: _pickAndSendImage,
                    onPickVideo: _pickAndSendVideo,
                    onPickAudio: _pickAndSendAudio,
                    onSendCode: _onSendCode,
                    onLoadEmotes: () => ref.read(emoteServiceProvider).listEmotes(),
                    onSendEmote: _onSendEmote,
                    ),
                  ),
                ],
              ),
            ),
          ),
          Positioned(
            top: 0,
            left: 0,
            right: 0,
            child: UIProviderHost(
              capability: UICapability.conversationHeader,
              context: providerContext,
              actions: providerActions,
              fallback: _ChatTopBar(
                onOpenDrawer: () => _openDrawer(context),
                onCall: () => _showCallOptions(characterId, characterName),
                onMore: () => _showChatActionsSheet(context),
              ),
            ),
          ),
          if (hasSidebarProvider)
            Positioned(
              top: _chatTopBarHeight,
              right: 0,
              bottom: 0,
              child: SizedBox(
                width: MediaQuery.sizeOf(context).width.clamp(240.0, 360.0).toDouble(),
                child: UIProviderHost(
                  capability: UICapability.conversationSidebar,
                  context: {...providerContext, 'surface': 'sidebar'},
                  actions: providerActions,
                  fallback: const SizedBox.shrink(),
                ),
              ),
            ),
          if (hasOverlayProvider)
            Positioned.fill(
              child: UIProviderHost(
                capability: UICapability.conversationOverlay,
                context: {...providerContext, 'surface': 'overlay'},
                actions: providerActions,
                fallback: const SizedBox.shrink(),
              ),
            ),
        ],
      ),
    );



    return UIProviderHost(
      capability: UICapability.conversationShell,
      fallback: builtinConversation,
      context: providerContext,
      actions: providerActions,
    );
  }
}

const double _chatTopBarHeight = 68;

class _ChatScrollFade extends StatelessWidget {
  final Alignment alignment;
  final Alignment begin;
  final Alignment end;
  final Color color;
  final double height;

  const _ChatScrollFade({
    required this.alignment,
    required this.begin,
    required this.end,
    required this.color,
    this.height = 32,
  });

  @override
  Widget build(BuildContext context) {
    return IgnorePointer(
      child: Align(
        alignment: alignment,
        child: Container(
          width: double.infinity,
          height: height,
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: begin,
              end: end,
              colors: [color, color.withValues(alpha: 0)],
            ),
          ),
        ),
      ),
    );
  }
}

class _ChatTopBar extends StatelessWidget implements PreferredSizeWidget {
  final VoidCallback onOpenDrawer;
  final VoidCallback onCall;
  final VoidCallback onMore;

  const _ChatTopBar({
    required this.onOpenDrawer,
    required this.onCall,
    required this.onMore,
  });

  @override
  Size get preferredSize => const Size.fromHeight(_chatTopBarHeight);

  @override
  Widget build(BuildContext context) {
    final platform = Theme.of(context).platform;
    final isApplePlatform =
        platform == TargetPlatform.iOS || platform == TargetPlatform.macOS;

    return SafeArea(
      bottom: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(12, 8, 12, 8),
        child: Row(
          children: [
            _ChatTopBarButton(
              icon: isApplePlatform
                  ? CupertinoIcons.line_horizontal_3
                  : Icons.menu_rounded,
              size: 44,
              iconSize: 20,
              tooltip: '打开侧边栏',
              onTap: onOpenDrawer,
            ),
            const Spacer(),
            _ChatTopBarButton(
              icon: Icons.call_outlined,
              size: 44,
              iconSize: 22,
              tooltip: '发起通话',
              onTap: onCall,
            ),
            const SizedBox(width: 8),
            _ChatTopBarButton(
              icon: isApplePlatform ? CupertinoIcons.ellipsis : Icons.more_horiz,
              size: 44,
              iconSize: 22,
              tooltip: '当前对话详情',
              onTap: onMore,
            ),
          ],
        ),
      ),
    );
  }
}

class _ChatTopBarButton extends StatelessWidget {
  final IconData icon;
  final double size;
  final double iconSize;
  final String tooltip;
  final VoidCallback onTap;

  const _ChatTopBarButton({
    required this.icon,
    required this.size,
    required this.iconSize,
    required this.tooltip,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: onTap,
        child: Container(
          width: size,
          height: size,
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: BorderRadius.circular(13),
            border: Border.all(color: context.borderPrimary),
          ),
          alignment: Alignment.center,
          child: Icon(icon, size: iconSize, color: context.textPrimary),
        ),
      ),
    );
  }
}

class _MessageSearchSheet extends StatefulWidget {
  final List<ChatMessage> messages;
  final ValueChanged<int> onJump;

  const _MessageSearchSheet({required this.messages, required this.onJump});

  @override
  State<_MessageSearchSheet> createState() => _MessageSearchSheetState();
}

class _MessageSearchSheetState extends State<_MessageSearchSheet> {
  final _controller = TextEditingController();
  String _query = '';

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  List<(int, ChatMessage)> get _results {
    if (_query.trim().isEmpty) return const [];
    final q = _query.trim().toLowerCase();
    final list = <(int, ChatMessage)>[];
    for (var i = 0; i < widget.messages.length; i++) {
      final m = widget.messages[i];
      if (m.type == MessageType.systemNotice) continue;
      if (m.content.toLowerCase().contains(q)) {
        list.add((i, m));
      }
    }
    return list;
  }

  String _preview(ChatMessage m) {
    if (m.type == MessageType.file) return '[文件] ${m.fileName ?? m.content}';
    if (m.type == MessageType.image) return '[图片] ${m.fileName ?? ''}';
    if (m.type == MessageType.video) return '[视频] ${m.fileName ?? ''}';
    if (m.type == MessageType.audio) return '[语音] ${m.fileName ?? ''}';
    if (m.type == MessageType.emote) return '[表情] ${m.content}';
    if (m.type == MessageType.code) return '[代码] ${m.content}';
    return m.content;
  }

  @override
  Widget build(BuildContext context) {
    final results = _results;
    return SafeArea(
      child: SizedBox(
        height: MediaQuery.sizeOf(context).height * 0.65,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
          child: Column(
            children: [
              const SizedBox(height: 8),
              Center(
                child: Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: context.borderPrimary,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 16),
              Text('搜索聊天消息', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 12),
              AmitiaSearchField(
                hintText: '输入关键词搜索消息',
                controller: _controller,
                onChanged: (v) => setState(() => _query = v),
              ),
              const SizedBox(height: 8),
              Expanded(
                child: _query.trim().isEmpty
                    ? Center(
                        child: Text(
                          '输入关键词后在当前会话中搜索消息',
                          style: AppTypography.caption(context),
                          textAlign: TextAlign.center,
                        ),
                      )
                    : results.isEmpty
                    ? Center(
                        child: Text(
                          '没有找到匹配「$_query」的消息',
                          style: AppTypography.caption(context),
                          textAlign: TextAlign.center,
                        ),
                      )
                    : ListView.separated(
                        itemCount: results.length,
                        separatorBuilder: (_, _) =>
                            Divider(height: 1, color: context.borderSecondary),
                        itemBuilder: (ctx, i) {
                          final item = results[i];
                          final msg = item.$2;
                          return ListTile(
                            leading: Icon(
                              msg.role == MessageRole.user
                                  ? Icons.person_outline
                                  : Icons.smart_toy_outlined,
                              size: 20,
                              color: context.textTertiary,
                            ),
                            title: Text(
                              _preview(msg),
                              style: AppTypography.bodySmall(context),
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                            ),
                            subtitle: Text(
                              '${msg.time.hour.toString().padLeft(2, '0')}:${msg.time.minute.toString().padLeft(2, '0')}',
                              style: AppTypography.label(context),
                            ),
                            onTap: () => widget.onJump(item.$1),
                          );
                        },
                      ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}


class _MobileChatFlowItem {
  const _MobileChatFlowItem._({
    required this.key,
    required this.timestamp,
    this.sequence,
    this.message,
    this.messageIndex,
    this.node,
  });

  factory _MobileChatFlowItem.message({
    required ChatMessage message,
    required int messageIndex,
    required DateTime timestamp,
  }) => _MobileChatFlowItem._(
    key: 'message:${message.id}',
    timestamp: timestamp,
    message: message,
    messageIndex: messageIndex,
  );

  factory _MobileChatFlowItem.node(MobileConversationNode node) => _MobileChatFlowItem._(
    key: 'node:${node.nodeId}',
    timestamp: node.anchorTimestamp,
    sequence: node.anchorSeq,
    node: node,
  );

  final String key;
  final DateTime timestamp;
  final int? sequence;
  final ChatMessage? message;
  final int? messageIndex;
  final MobileConversationNode? node;

  bool get isMessage => message != null;
}
