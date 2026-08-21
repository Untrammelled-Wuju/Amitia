import 'dart:async';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
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
import '../../runtime/conversation_runtime_controller.dart';
import '../../../../shared/models/models.dart';
import 'realtime_voice_call_sheet.dart';

class ChatPage extends ConsumerStatefulWidget {
  const ChatPage({super.key});

  @override
  ConsumerState<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends ConsumerState<ChatPage> {
  final _scrollController = ScrollController();
  late final ConversationRuntimeController _runtime;

  @override
  void initState() {
    super.initState();
    _runtime = ConversationRuntimeController(ref.read(chatServiceProvider), ref.read(emoteServiceProvider));
    _runtime.addListener(_onRuntimeChanged);
  }

  void _onRuntimeChanged() {
    if (!mounted) return;
    setState(() {});
    _scrollToBottom();
  }

  @override
  void dispose() {
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
    final current = _runtime.messages[index];
    if (current.role != MessageRole.assistant) return false;
    if (current.type == MessageType.agentTask ||
        current.type == MessageType.toolCall)
      return false;
    if (index == 0) return true;
    final previous = _runtime.messages[index - 1];
    if (previous.role != MessageRole.assistant) return true;
    return false;
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

  void _showChatActionsSheet(BuildContext context) {
    showAmitiaActionSheet<int>(
      context,
      title: '聊天操作',
      actions: const [
        AmitiaActionSheetItem(
          icon: Icons.person_outline,
          label: '查看角色详情',
          value: 0,
        ),
        AmitiaActionSheetItem(
          icon: Icons.cleaning_services_outlined,
          label: '清空聊天记录',
          value: 1,
          isDestructive: true,
        ),
        AmitiaActionSheetItem(
          icon: Icons.file_download_outlined,
          label: '导出聊天记录',
          value: 2,
        ),
        AmitiaActionSheetItem(
          icon: Icons.share_outlined,
          label: '分享对话',
          value: 3,
        ),
        AmitiaActionSheetItem(icon: Icons.search, label: '搜索当前会话', value: 4),
      ],
    ).then((result) {
      if (result == null || !mounted) return;
      switch (result) {
        case 0:
          context.push(
            AppRoutes.character(ref.read(currentCharacterIdProvider)),
          );
        case 1:
          showAmitiaConfirmDialog(
            context,
            title: '清空聊天记录',
            message: '确定要清空当前聊天记录吗？此操作不可撤销。',
            confirmLabel: '清空',
            isDestructive: true,
          ).then((confirmed) {
            if (confirmed == true && mounted) {
              _runtime.clear(addSystemNotice: true);
              amitiaSnackBar(context, '聊天记录已清空');
            }
          });
        case 2:
          amitiaSnackBar(context, '聊天记录已导出到本地');
        case 3:
          amitiaSnackBar(context, '对话链接已复制');
        case 4:
          _showMessageSearch(context);
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

    final avatarInitial = character?.name.isNotEmpty == true ? character!.name[0] : '?';
    final avatarColor =
        '#${(((character?.name.hashCode ?? 0) & 0xFFFFFF) | 0xFF000000).toRadixString(16).padLeft(8, '0')}';
    final characterName = character?.name ?? '';

    final providerContext = <String, dynamic>{
      'route': '/chat',
      'character': {
        'id': characterId,
        'name': characterName,
        'avatarInitial': avatarInitial,
        'avatarColor': avatarColor,
      },
      'messages': _runtime.messages.map(_providerMessage).toList(growable: false),
      'agentMode': isAgentMode,
      'conversationState': _runtime.state,
      'sending': _runtime.sending,
      'conversationId': _runtime.conversationId,
    };
    final providerActions = <String, FutureOr<dynamic> Function(dynamic)>{
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
      ConversationUIAction.newConversation: (_) =>
          _runtime.createConversation(characterId),
      ConversationUIAction.openDrawer: (_) {
        _openDrawer(context);
        return null;
      },
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

    final uiSnapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    bool externalProvider(String capability) {
      final provider = uiSnapshot?.resolve(capability);
      return provider != null && provider.enabled && !provider.builtin;
    }
    final hasSidebarProvider = externalProvider(UICapability.conversationSidebar);
    final hasOverlayProvider = externalProvider(UICapability.conversationOverlay);

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
                          itemCount: _runtime.messages.length,
                          itemBuilder: (context, index) {
                            final message = _runtime.messages[index];
                            final isAgentTask =
                                message.type == MessageType.agentTask;
                            final builtinMessage = AmitiaMessageBubble(
                              message: message,
                              showAvatar: _shouldShowAvatar(index),
                              avatarInitial: avatarInitial,
                              avatarColor: avatarColor,
                              characterName: characterName,
                              onRetry: message.status == MessageStatus.error
                                  ? () => _retryMessage(index)
                                  : null,
                              onAgentTaskTap: isAgentTask
                                  ? () => context.push(AppRoutes.agent)
                                  : null,
                            );
                            final messageRenderer = UIMessageRendererRegistry.resolve(
                              uiSnapshot,
                              messageType: message.type.name,
                              role: message.role.name,
                            );
                            return UIProviderHost(
                              capability: UICapability.conversationMessageRenderer,
                              providerId: messageRenderer?.providerId,
                              fallback: builtinMessage,
                              context: { ...providerContext, 'message': _providerMessage(message), 'messageIndex': index },
                              actions: providerActions,
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
              onNewConversation: () {
                _runtime.createConversation(characterId);
              },
              onCall: () => _startRealtimeCall(characterId, characterName),
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
  final VoidCallback onNewConversation;
  final VoidCallback onCall;
  final VoidCallback onMore;

  const _ChatTopBar({
    required this.onOpenDrawer,
    required this.onNewConversation,
    required this.onCall,
    required this.onMore,
  });

  @override
  Size get preferredSize => const Size.fromHeight(_chatTopBarHeight);

  @override
  Widget build(BuildContext context) {
    final platform = Theme.of(context).platform;
    final isApplePlatform =
        platform == TargetPlatform.iOS ||
        platform == TargetPlatform.macOS;

    return SafeArea(
      bottom: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(12, 8, 12, 8),
        child: Row(
          children: [
            ConduitStyleToolbarButton(
              icon: isApplePlatform
                  ? CupertinoIcons.line_horizontal_3
                  : Icons.menu_rounded,
              iconSize: 20,
              tooltip: '打开侧边栏',
              onPressed: onOpenDrawer,
            ),
            const Spacer(),
            Material(
              color: context.surfaceSecondary,
              shape: RoundedRectangleBorder(
                side: BorderSide(color: context.borderPrimary),
                borderRadius: BorderRadius.circular(20),
              ),
              child: SizedBox(
                height: 36,
                child: Row(
                  children: [
                    Tooltip(
                      message: '实时语音通话',
                      child: InkWell(
                        borderRadius: const BorderRadius.horizontal(
                          left: Radius.circular(20),
                        ),
                        onTap: onCall,
                        child: Padding(
                          padding: const EdgeInsets.fromLTRB(10, 0, 7, 0),
                          child: Icon(
                            Icons.call_outlined,
                            size: 19,
                            color: context.textPrimary,
                          ),
                        ),
                      ),
                    ),
                    Container(width: 1, height: 18, color: context.borderPrimary),
                    Tooltip(
                      message: '新建聊天',
                      child: InkWell(
                        borderRadius: const BorderRadius.horizontal(
                          right: Radius.circular(20),
                        ),
                        onTap: onNewConversation,
                        child: Padding(
                          padding: const EdgeInsets.fromLTRB(7, 0, 10, 0),
                          child: Icon(
                            Icons.edit_square,
                            size: 20,
                            color: context.textPrimary,
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(width: 8),
            ConduitStyleToolbarButton(
              icon: isApplePlatform
                  ? CupertinoIcons.ellipsis
                  : Icons.more_vert,
              iconSize: 22,
              tooltip: '更多',
              onPressed: onMore,
            ),
          ],
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
