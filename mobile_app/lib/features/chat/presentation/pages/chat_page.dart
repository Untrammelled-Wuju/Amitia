import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'package:flutter/cupertino.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_motion.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_message.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/realtime/realtime_audio_bridge.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/chat_service.dart';
import '../../../../core/services/workspace_service.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/runtime/backend/mobile_deployment_mode.dart';
import '../../../../core/native_bridge/providers/native_bridge_relay_provider.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/memory.dart';
import '../../../../core/models/profile.dart';
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

class ChatPage extends ConsumerStatefulWidget {
  const ChatPage({
    super.key,
    this.initialConversationId,
    this.initialCharacterId,
    this.initialProjectId,
  });

  final String? initialConversationId;
  final String? initialCharacterId;
  final String? initialProjectId;

  @override
  ConsumerState<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends ConsumerState<ChatPage> {
  final _scrollController = ScrollController();
  final _composerController = TextEditingController();
  late final ConversationRuntimeController _runtime;
  late final StateController<String> _activeConversationIdController;
  Map<String, dynamic>? _cachedProviderContext;
  List<ChatMessage>? _cachedMessagesForContext;
  Map<String, FutureOr<dynamic> Function(dynamic)>? _cachedProviderActions;
  String _cachedProviderActionsCharacterId = '';
  Timer? _conversationEventRefreshTimer;
  ChatMessage? _replyTarget;
  List<WorkspaceMountDto> _recentWorkspaces = const <WorkspaceMountDto>[];
  bool _workspaceBusy = false;
  final RealtimeAudioBridge _realtimeAudio = RealtimeAudioBridge();
  final BytesBuilder _voicePcm = BytesBuilder(copy: false);
  StreamSubscription<Uint8List>? _voiceInputSubscription;
  bool _voiceRecording = false;
  String _routeSyncedConversationId = '';
  Timer? _composerDraftTimer;
  bool _loadingComposerDraft = false;
  String _activeComposerDraftKey = '';

  @override
  void initState() {
    super.initState();
    _activeConversationIdController = ref.read(
      activeConversationIdProvider.notifier,
    );
    _runtime = ref.read(conversationRuntimeControllerProvider);
    _runtime.addListener(_onRuntimeChanged);
    _composerController.addListener(_handleComposerChanged);
    WidgetsBinding.instance.addPostFrameCallback(
      (_) => _openInitialConversation(),
    );
  }

  @override
  void didUpdateWidget(covariant ChatPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    final oldConversationId = oldWidget.initialConversationId?.trim() ?? '';
    final newConversationId = widget.initialConversationId?.trim() ?? '';
    final oldProjectId = oldWidget.initialProjectId?.trim() ?? '';
    final newProjectId = widget.initialProjectId?.trim() ?? '';
    if (oldConversationId == newConversationId &&
        oldProjectId == newProjectId) {
      return;
    }
    final runtimeConversationId = _runtime.conversationId?.trim() ?? '';
    if (newConversationId.isNotEmpty &&
        newConversationId == _routeSyncedConversationId &&
        newConversationId == runtimeConversationId) {
      return;
    }
    _routeSyncedConversationId = '';
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      if (newConversationId.isEmpty) {
        _runtime.startDraft();
        unawaited(_openDraftWorkspace(newProjectId));
      } else {
        unawaited(_openInitialConversation());
      }
    });
  }

  Future<void> _openInitialConversation() async {
    final initialConversationId = widget.initialConversationId?.trim() ?? '';
    final conversationId = initialConversationId.isNotEmpty
        ? initialConversationId
        : _activeConversationIdController.state.trim();
    if (!mounted) return;
    if (conversationId.isEmpty) {
      _runtime.startDraft();
      await _openDraftWorkspace(widget.initialProjectId);
      await _refreshRecentWorkspaces();
      return;
    }
    final characterId = widget.initialCharacterId?.trim() ?? '';
    if (characterId.isNotEmpty) {
      ref.read(currentCharacterIdProvider.notifier).state = characterId;
    }
    await _runtime.openConversation(
      conversationId,
      characterId: characterId.isEmpty ? null : characterId,
    );
    await Future.wait<void>([
      _loadConversationWorkspace(conversationId),
      _refreshRecentWorkspaces(),
    ]);
  }

  void _onRuntimeChanged() {
    if (!mounted) return;
    _cachedProviderContext = null;
    _cachedMessagesForContext = null;
    _cachedProviderActions = null;
    _cachedProviderActionsCharacterId = '';
    final conversationId = _runtime.conversationId?.trim() ?? '';
    ref.read(activeConversationIdProvider.notifier).state = conversationId;
    unawaited(_syncComposerDraft());
    _syncCreatedConversationRoute(conversationId);
    _conversationEventRefreshTimer?.cancel();
    if (conversationId.isNotEmpty) {
      _conversationEventRefreshTimer = Timer(
        const Duration(milliseconds: 350),
        () {
          if (!mounted) return;
          ref.invalidate(conversationUIEventWindowProvider(conversationId));
        },
      );
    }
    setState(() {});
    _scrollToBottom();
  }

  Future<void> _openDraftWorkspace(String? projectId) async {
    final id = projectId?.trim() ?? '';
    if (id.isEmpty) {
      _runtime.setWorkspace(null);
      return;
    }
    try {
      final sidebar = await ref.read(chatServiceProvider).conversationSidebar();
      final project = sidebar.projects
          .where((item) => item.id == id)
          .firstOrNull;
      if (!mounted) return;
      _runtime.setWorkspace(
        project == null
            ? null
            : ConversationWorkspaceDto(
                projectId: project.id,
                workspaceId: project.workspaceId,
                deviceId: project.deviceId,
                workspaceName: project.name,
                workspaceKind: project.rootUri.startsWith('content://')
                    ? 'saf'
                    : 'local',
                rootUri: project.rootUri,
              ),
      );
    } catch (_) {
      if (mounted) _runtime.setWorkspace(null);
    }
  }

  void _syncCreatedConversationRoute(String conversationId) {
    if (conversationId.isEmpty ||
        _routeSyncedConversationId == conversationId ||
        (widget.initialConversationId ?? '').trim().isNotEmpty) {
      return;
    }
    _routeSyncedConversationId = conversationId;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      context.go(
        '${AppRoutes.chat}?conversationId=${Uri.encodeQueryComponent(conversationId)}',
      );
    });
  }

  Map<String, dynamic> _buildProviderContext(
    String characterId,
    String characterName,
    String avatarInitial,
    String avatarColor,
  ) {
    final currentMessages = _runtime.messages;
    if (_cachedProviderContext != null &&
        _cachedMessagesForContext != null &&
        _cachedMessagesForContext!.length == currentMessages.length) {
      bool same = true;
      for (int i = 0; i < currentMessages.length; i++) {
        if (_cachedMessagesForContext![i].id != currentMessages[i].id ||
            _cachedMessagesForContext![i].renderId !=
                currentMessages[i].renderId ||
            _cachedMessagesForContext![i].type != currentMessages[i].type ||
            _cachedMessagesForContext![i].content !=
                currentMessages[i].content ||
            _cachedMessagesForContext![i].status != currentMessages[i].status) {
          same = false;
          break;
        }
      }
      if (same) return _cachedProviderContext!;
    }
    final messagesMap = currentMessages
        .map(_providerMessage)
        .toList(growable: false);
    final workspace = _runtime.workspace;
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
      'workspace': workspace == null
          ? null
          : <String, dynamic>{
              'workspaceId': workspace.workspaceId,
              'deviceId': workspace.deviceId,
              'name': workspace.workspaceName,
              'kind': workspace.workspaceKind,
              'rootUri': workspace.rootUri,
            },
      'recentWorkspaces': _recentWorkspaces
          .map(
            (mount) => <String, dynamic>{
              'workspaceId': mount.id,
              'name': mount.name,
              'kind': mount.kind,
              'rootUri': mount.rootUri,
              'available': mount.available,
              'status': mount.status,
              'statusReason': mount.statusReason,
            },
          )
          .toList(growable: false),
    };
    return _cachedProviderContext!;
  }

  Widget _buildEmptyChatState(
    BuildContext context,
    String characterName,
    String workspaceName,
  ) {
    final resolvedCharacterName = characterName.trim().isEmpty
        ? 'Amitia'
        : characterName.trim();
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 32),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            '你好，我是 $resolvedCharacterName',
            textAlign: TextAlign.center,
            style: AppTypography.pageTitle(
              context,
            ).copyWith(fontSize: 20, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 8),
          Text(
            workspaceName.isEmpty
                ? '随时可以和我聊聊天，或者做你想做的事。'
                : '你想在 $workspaceName 中构建什么？',
            textAlign: TextAlign.center,
            style: AppTypography.body(
              context,
            ).copyWith(color: context.textSecondary),
          ),
        ],
      ),
    );
  }

  @override
  void dispose() {
    _composerDraftTimer?.cancel();
    unawaited(_persistComposerDraftNow(_activeComposerDraftKey));
    _composerController.removeListener(_handleComposerChanged);
    _conversationEventRefreshTimer?.cancel();
    unawaited(_voiceInputSubscription?.cancel());
    unawaited(_realtimeAudio.stopCapture());
    _runtime.removeListener(_onRuntimeChanged);
    _scrollController.dispose();
    _composerController.dispose();
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

  void _retryMessage(int index) {
    _runtime.retryMessage(index);
  }

  Future<void> _refreshRecentWorkspaces() async {
    try {
      final sidebar = await ref.read(chatServiceProvider).conversationSidebar();
      if (!mounted) return;
      _cachedProviderContext = null;
      setState(() {
        _recentWorkspaces = sidebar.projects
            .map(
              (project) => WorkspaceMountDto(
                id: project.workspaceId,
                projectId: project.id,
                name: project.name,
                kind: project.rootUri.startsWith('content://')
                    ? 'saf'
                    : 'local',
                rootUri: project.rootUri,
                readOnly: project.status == 'read_only',
                available: project.available,
                status: project.status,
                statusReason: project.statusReason,
              ),
            )
            .take(20)
            .toList(growable: false);
      });
    } catch (_) {
      if (!mounted) return;
      _cachedProviderContext = null;
      setState(() => _recentWorkspaces = const <WorkspaceMountDto>[]);
    }
  }

  Future<void> _loadConversationWorkspace(String conversationId) async {
    final id = conversationId.trim();
    if (id.isEmpty) {
      _runtime.setWorkspace(null);
      return;
    }
    try {
      final sidebar = await ref.read(chatServiceProvider).conversationSidebar();
      final conversation = <dynamic>[
        ...sidebar.pinned,
        ...sidebar.recent,
        ...sidebar.projects.expand((project) => project.conversations),
      ].where((item) => item.id == id).firstOrNull;
      if (!mounted || _runtime.conversationId?.trim() != id) return;
      final projectId = conversation?.projectId?.toString() ?? '';
      final project = projectId.isEmpty
          ? null
          : sidebar.projects.where((item) => item.id == projectId).firstOrNull;
      _runtime.setWorkspace(
        project == null
            ? null
            : ConversationWorkspaceDto(
                conversationId: id,
                projectId: project.id,
                workspaceId: project.workspaceId,
                deviceId: project.deviceId,
                workspaceName: project.name,
                workspaceKind: project.rootUri.startsWith('content://')
                    ? 'saf'
                    : 'local',
                rootUri: project.rootUri,
              ),
      );
    } catch (_) {
      if (!mounted || _runtime.conversationId?.trim() != id) return;
      _runtime.setWorkspace(null);
    }
  }

  Future<ConversationWorkspaceDto> _workspaceBindingForMount(
    WorkspaceMountDto mount,
  ) async {
    var deviceId = '';
    final deployment = ref.read(mobileDeploymentConfigProvider);
    if (deployment.mode == MobileDeploymentMode.cloud) {
      final localMesh = ref.read(deviceMeshLocalServiceProvider);
      if (localMesh == null) {
        throw StateError('本机 Device Agent 尚未就绪，云端模式无法绑定本地工作目录');
      }
      final identity = await localMesh.identity();
      deviceId = (identity['deviceId'] ?? '').toString().trim();
      if (deviceId.isEmpty) {
        throw StateError('无法读取本机 Device Mesh 身份');
      }
    }
    return ConversationWorkspaceDto(
      conversationId: _runtime.conversationId?.trim() ?? '',
      projectId: mount.projectId,
      workspaceId: mount.id,
      deviceId: deviceId,
      workspaceName: mount.name,
      workspaceKind: mount.kind,
      rootUri: mount.rootUri.isEmpty
          ? 'amitia://workspace/@${mount.id}/'
          : mount.rootUri,
    );
  }

  Future<void> _selectWorkspaceMount(WorkspaceMountDto mount) async {
    if (_workspaceBusy) return;
    if (!mount.available) {
      if (mounted) {
        amitiaSnackBar(
          context,
          mount.statusReason.isNotEmpty ? mount.statusReason : '该工作目录当前不可用',
        );
      }
      return;
    }
    setState(() => _workspaceBusy = true);
    try {
      final binding = await _workspaceBindingForMount(mount);
      final conversationId = _runtime.conversationId?.trim() ?? '';
      if (conversationId.isNotEmpty && mount.projectId.isNotEmpty) {
        await ref
            .read(chatServiceProvider)
            .moveConversationToProject(conversationId, mount.projectId);
      }
      _runtime.setWorkspace(binding);
      try {
        await ref.read(workspaceServiceProvider).touchLocal(mount.id);
      } catch (_) {}
      await _refreshRecentWorkspaces();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '切换工作目录失败：$error');
    } finally {
      if (mounted) setState(() => _workspaceBusy = false);
    }
  }

  Future<WorkspaceMountDto?> _pickWorkspaceMount() async {
    if (defaultTargetPlatform == TargetPlatform.android) {
      final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
      final response = await dispatcher.execute(<String, dynamic>{
        'protocolVersion': 1,
        'requestId':
            'workspace-picker-${DateTime.now().microsecondsSinceEpoch}',
        'platform': 'android',
        'operation': 'workspace.saf.pick_tree',
        'payload': const <String, dynamic>{},
      });
      if ((response['status'] ?? '').toString() != 'success') {
        final rawError = response['error'];
        final error = rawError is Map
            ? Map<String, dynamic>.from(rawError)
            : const <String, dynamic>{};
        throw StateError((error['message'] ?? '系统目录授权失败').toString());
      }
      final rawResult = response['result'];
      final result = rawResult is Map
          ? Map<String, dynamic>.from(rawResult)
          : const <String, dynamic>{};
      if (result['cancelled'] == true) return null;
      final grantId = (result['grantId'] ?? '').toString().trim();
      if (grantId.isEmpty) throw StateError('系统目录授权未返回 grantId');
      return ref
          .read(workspaceServiceProvider)
          .registerSaf(
            name: (result['name'] ?? '工作目录').toString(),
            grantId: grantId,
            readOnly: result['readOnly'] == true,
          );
    }

    final path = await FilePicker.platform.getDirectoryPath(
      dialogTitle: '选择工作目录',
    );
    final localRoot = path?.trim() ?? '';
    if (localRoot.isEmpty) return null;
    final normalized = localRoot.replaceAll('\\', '/');
    final segments = normalized
        .split('/')
        .where((segment) => segment.trim().isNotEmpty)
        .toList(growable: false);
    final name = segments.isEmpty ? '工作目录' : segments.last;
    return ref
        .read(workspaceServiceProvider)
        .registerLocal(name: name, localRoot: localRoot);
  }

  Future<void> _chooseWorkspaceDirectory() async {
    if (_workspaceBusy) return;
    setState(() => _workspaceBusy = true);
    try {
      final mount = await _pickWorkspaceMount();
      if (mount == null) return;
      final sidebar = await ref.read(chatServiceProvider).conversationSidebar();
      final existing = sidebar.projects
          .where((project) => project.workspaceId == mount.id)
          .firstOrNull;
      final project =
          existing ??
          await ref
              .read(chatServiceProvider)
              .createProject(
                name: mount.name.trim().isEmpty ? '项目' : mount.name,
                workspaceId: mount.id,
                rootUri: mount.rootUri,
              );
      final projectMount = WorkspaceMountDto(
        id: mount.id,
        projectId: project.id,
        name: project.name,
        kind: mount.kind,
        rootUri: mount.rootUri,
        readOnly: mount.readOnly,
        available: project.available,
        status: project.status,
        statusReason: project.statusReason,
      );
      final binding = await _workspaceBindingForMount(projectMount);
      final conversationId = _runtime.conversationId?.trim() ?? '';
      if (conversationId.isNotEmpty) {
        await ref
            .read(chatServiceProvider)
            .moveConversationToProject(conversationId, project.id);
      }
      _runtime.setWorkspace(binding);
      await _refreshRecentWorkspaces();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '选择工作目录失败：$error');
    } finally {
      if (mounted) setState(() => _workspaceBusy = false);
    }
  }

  Future<void> _clearWorkspace() async {
    if (_workspaceBusy) return;
    setState(() => _workspaceBusy = true);
    try {
      final conversationId = _runtime.conversationId?.trim() ?? '';
      if (conversationId.isNotEmpty) {
        await ref
            .read(chatServiceProvider)
            .moveConversationToProject(conversationId, '');
      }
      _runtime.setWorkspace(null);
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '清除工作目录失败：$error');
    } finally {
      if (mounted) setState(() => _workspaceBusy = false);
    }
  }

  Future<void> _showWorkspacePicker() async {
    await _refreshRecentWorkspaces();
    if (!mounted) return;
    final selectedId = _runtime.workspace?.workspaceId ?? '';
    await showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      backgroundColor: context.surfacePrimary,
      builder: (sheetContext) => SafeArea(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxHeight: 520),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Padding(
                padding: EdgeInsets.fromLTRB(20, 2, 20, 10),
                child: Text(
                  '项目',
                  style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
                ),
              ),
              if (_recentWorkspaces.isEmpty)
                const Padding(
                  padding: EdgeInsets.symmetric(horizontal: 20, vertical: 18),
                  child: Text('暂无项目'),
                )
              else
                Flexible(
                  child: ListView.builder(
                    shrinkWrap: true,
                    itemCount: _recentWorkspaces.length,
                    itemBuilder: (_, index) {
                      final mount = _recentWorkspaces[index];
                      return ListTile(
                        leading: const Icon(Icons.folder_outlined),
                        title: Text(
                          mount.name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        subtitle: Text(
                          mount.available
                              ? (mount.kind == 'saf' ? '系统授权目录' : '本机目录')
                              : (mount.statusReason.isNotEmpty
                                    ? mount.statusReason
                                    : '目录不可用'),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        enabled: mount.available && !_workspaceBusy,
                        trailing: mount.id == selectedId
                            ? const Icon(Icons.check_rounded)
                            : null,
                        onTap: () {
                          Navigator.of(sheetContext).pop();
                          _selectWorkspaceMount(mount);
                        },
                      );
                    },
                  ),
                ),
              const Divider(height: 1),
              ListTile(
                leading: const Icon(Icons.create_new_folder_outlined),
                title: const Text('添加文件夹为项目…'),
                enabled: !_workspaceBusy,
                onTap: () {
                  Navigator.of(sheetContext).pop();
                  _chooseWorkspaceDirectory();
                },
              ),
              if (_runtime.workspace != null)
                ListTile(
                  leading: const Icon(Icons.close_rounded),
                  title: const Text('移出项目'),
                  enabled: !_workspaceBusy,
                  onTap: () {
                    Navigator.of(sheetContext).pop();
                    _clearWorkspace();
                  },
                ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildWorkspaceBar(BuildContext context) {
    final workspace = _runtime.workspace;
    final label = workspace?.workspaceName.trim().isNotEmpty == true
        ? workspace!.workspaceName.trim()
        : '选择项目';
    return ConstrainedBox(
      constraints: const BoxConstraints(maxWidth: 180),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          borderRadius: BorderRadius.circular(8),
          onTap: _workspaceBusy ? null : _showWorkspacePicker,
          child: Container(
            height: 31,
            padding: const EdgeInsets.symmetric(horizontal: 7),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.folder_outlined, size: 16),
                const SizedBox(width: 5),
                Flexible(
                  child: Text(
                    label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 12),
                  ),
                ),
                const SizedBox(width: 3),
                if (_workspaceBusy)
                  const SizedBox(
                    width: 12,
                    height: 12,
                    child: CircularProgressIndicator(strokeWidth: 1.5),
                  )
                else
                  const Icon(Icons.keyboard_arrow_down_rounded, size: 16),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _onSend(String text) {
    _composerDraftTimer?.cancel();
    unawaited(_persistComposerDraftNow(_activeComposerDraftKey, value: ''));
    final reply = _replyTarget;
    if (reply != null) {
      setState(() => _replyTarget = null);
    }
    _runtime.sendText(
      text,
      replyToMessageId: reply?.id,
      replyToExcerpt: reply == null ? null : _replyExcerpt(reply),
    );
  }

  String _composerDraftKey() {
    final conversationId = _runtime.conversationId?.trim() ?? '';
    if (conversationId.isNotEmpty) return 'conversation:$conversationId';
    final projectId = _runtime.workspace?.projectId.trim() ?? '';
    return projectId.isEmpty ? 'new:recent' : 'new:project:$projectId';
  }

  void _handleComposerChanged() {
    if (_loadingComposerDraft || _activeComposerDraftKey.isEmpty) return;
    _composerDraftTimer?.cancel();
    _composerDraftTimer = Timer(const Duration(milliseconds: 280), () {
      unawaited(_persistComposerDraftNow(_activeComposerDraftKey));
    });
  }

  Future<void> _persistComposerDraftNow(String key, {String? value}) async {
    final normalizedKey = key.trim();
    if (normalizedKey.isEmpty) return;
    final storageKey = 'webchat_draft:$normalizedKey';
    final draft = value ?? _composerController.text;
    final preferences = await SharedPreferences.getInstance();
    if (draft.trim().isEmpty) {
      await preferences.remove(storageKey);
    } else {
      await preferences.setString(storageKey, draft);
    }
  }

  Future<void> _syncComposerDraft() async {
    final nextKey = _composerDraftKey();
    if (nextKey == _activeComposerDraftKey) return;
    final previousKey = _activeComposerDraftKey;
    if (previousKey.isNotEmpty) {
      await _persistComposerDraftNow(previousKey);
    }
    _activeComposerDraftKey = nextKey;
    final preferences = await SharedPreferences.getInstance();
    final draft = preferences.getString('webchat_draft:$nextKey') ?? '';
    if (!mounted) return;
    _loadingComposerDraft = true;
    _composerController.value = TextEditingValue(
      text: draft,
      selection: TextSelection.collapsed(offset: draft.length),
    );
    _loadingComposerDraft = false;
  }

  Future<void> _startRecordedVoice() async {
    if (_voiceRecording) return;
    await _voiceInputSubscription?.cancel();
    _voicePcm.clear();
    _voiceInputSubscription = _realtimeAudio.inputPcm.listen(_voicePcm.add);
    try {
      await _realtimeAudio.startCapture();
      _voiceRecording = true;
    } catch (error) {
      await _voiceInputSubscription?.cancel();
      _voiceInputSubscription = null;
      if (mounted) amitiaSnackBar(context, '无法开始录音：$error');
    }
  }

  Future<Uint8List?> _stopRecordedVoice() async {
    if (!_voiceRecording) return null;
    _voiceRecording = false;
    try {
      await _realtimeAudio.stopCapture();
    } finally {
      await _voiceInputSubscription?.cancel();
      _voiceInputSubscription = null;
    }
    final pcm = _voicePcm.takeBytes();
    if (pcm.isEmpty) return null;
    return _encodeWavPcm16(pcm);
  }

  Future<void> _cancelRecordedVoice() async {
    await _stopRecordedVoice();
    _voicePcm.clear();
    if (mounted) amitiaSnackBar(context, '已取消录音');
  }

  Future<void> _finishRecordedVoice({required bool transcribe}) async {
    final wav = await _stopRecordedVoice();
    if (!mounted || wav == null) return;
    if (transcribe) {
      await _transcribeRecordedVoice(wav);
    } else {
      await _sendRecordedVoice(wav);
    }
  }

  Future<void> _sendRecordedVoice(Uint8List wav) async {
    try {
      final service = await ref.read(artifactServiceProvider.future);
      final artifact = await service.uploadBytes(
        bytes: wav,
        kind: ArtifactKind.audio,
        fileName: 'voice-${DateTime.now().millisecondsSinceEpoch}.wav',
        mimeType: 'audio/wav',
        source: 'chat_composer',
      );
      if (!mounted) return;
      await _runtime.sendVoice(
        resourceUri: artifact.resourceUri,
        displayUrl: service.contentUrl(artifact.id),
        fileName: artifact.filename,
        mimeType: artifact.mimeType,
        durationMs: _pcmDurationMs(wav),
      );
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '发送语音失败：$error');
    }
  }

  Future<void> _transcribeRecordedVoice(Uint8List wav) async {
    final connection = ref.read(backendConnectionProvider).asData?.value;
    if (connection is! BackendConnectionAvailable) {
      if (mounted) amitiaSnackBar(context, '后端未连接，无法转文字');
      return;
    }
    final dio = createAuthenticatedDio(connection.config);
    try {
      final uploadResponse = await dio.post(
        '/api/asr/upload',
        data: FormData.fromMap({
          'audio': MultipartFile.fromBytes(
            wav,
            filename: 'voice.wav',
            contentType: DioMediaType.parse('audio/wav'),
          ),
        }),
      );
      final uploadBody = uploadResponse.data;
      final uploadPayload = uploadBody is Map ? uploadBody['data'] : null;
      final audioUrl = uploadPayload is Map
          ? (uploadPayload['url'] ?? '').toString().trim()
          : '';
      if (audioUrl.isEmpty) throw StateError('后端未返回音频地址');

      final submitResponse = await dio.post(
        '/api/asr/submit',
        data: FormData.fromMap({'audioUrl': audioUrl}),
      );
      final submitBody = submitResponse.data;
      final submitPayload = submitBody is Map ? submitBody['data'] : null;
      final taskId = submitPayload is Map
          ? (submitPayload['taskId'] ?? '').toString().trim()
          : '';
      if (taskId.isEmpty) throw StateError('后端未返回识别任务');

      for (var attempt = 0; attempt < 24; attempt++) {
        await Future<void>.delayed(const Duration(milliseconds: 1200));
        if (!mounted) return;
        final queryResponse = await dio.get(
          '/api/asr/query',
          queryParameters: <String, dynamic>{'taskId': taskId},
        );
        final queryBody = queryResponse.data;
        final queryPayload = queryBody is Map ? queryBody['data'] : null;
        if (queryPayload is! Map) continue;
        final status = (queryPayload['status'] ?? '').toString().toLowerCase();
        final text = (queryPayload['result'] ?? queryPayload['text'] ?? '')
            .toString()
            .trim();
        if (text.isNotEmpty &&
            (status == 'success' ||
                status == 'completed' ||
                status == 'succeeded')) {
          _composerController.text = text;
          _composerController.selection = TextSelection.collapsed(
            offset: text.length,
          );
          if (mounted) amitiaSnackBar(context, '语音已转为文字');
          return;
        }
        if (status == 'failed' || status == 'error') {
          throw StateError('语音识别失败');
        }
      }
      throw StateError('语音识别超时');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '转文字失败：$error');
    } finally {
      dio.close(force: true);
    }
  }

  Uint8List _encodeWavPcm16(Uint8List pcm, {int sampleRate = 16000}) {
    final header = ByteData(44);
    void ascii(int offset, String value) {
      for (var index = 0; index < value.length; index++) {
        header.setUint8(offset + index, value.codeUnitAt(index));
      }
    }

    ascii(0, 'RIFF');
    header.setUint32(4, 36 + pcm.length, Endian.little);
    ascii(8, 'WAVE');
    ascii(12, 'fmt ');
    header.setUint32(16, 16, Endian.little);
    header.setUint16(20, 1, Endian.little);
    header.setUint16(22, 1, Endian.little);
    header.setUint32(24, sampleRate, Endian.little);
    header.setUint32(28, sampleRate * 2, Endian.little);
    header.setUint16(32, 2, Endian.little);
    header.setUint16(34, 16, Endian.little);
    ascii(36, 'data');
    header.setUint32(40, pcm.length, Endian.little);
    final bytes = BytesBuilder(copy: false);
    bytes.add(header.buffer.asUint8List());
    bytes.add(pcm);
    return bytes.takeBytes();
  }

  int _pcmDurationMs(Uint8List wav) {
    if (wav.length <= 44) return 0;
    return ((wav.length - 44) * 1000 / (16000 * 2)).round();
  }

  String _replyExcerpt(ChatMessage message) {
    final content = message.content.trim();
    if (content.isNotEmpty) {
      return content.length <= 120 ? content : '${content.substring(0, 120)}…';
    }
    if ((message.fileName ?? '').trim().isNotEmpty) {
      return message.fileName!.trim();
    }
    return switch (message.type) {
      MessageType.image => '[图片]',
      MessageType.video => '[视频]',
      MessageType.audio => '[语音]',
      MessageType.file => '[文件]',
      MessageType.emote => '[表情]',
      MessageType.code => '[代码]',
      _ => '[消息]',
    };
  }

  void _setReplyTarget(ChatMessage? message) {
    if (!mounted) return;
    setState(() => _replyTarget = message);
  }

  Future<List<Map<String, dynamic>>> _loadAgentSkills() {
    return ref.read(extensionServiceProvider).agentSkills();
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

  Future<ArtifactMetadata> _uploadProviderBase64(
    ArtifactService service, {
    required String encoded,
    required ArtifactKind kind,
    required String fallbackMimeType,
    required String fallbackFileName,
  }) async {
    var payload = encoded.trim();
    var mimeType = fallbackMimeType;
    if (payload.startsWith('data:')) {
      final comma = payload.indexOf(',');
      if (comma <= 5) {
        throw ArtifactServiceException('invalid_data_uri');
      }
      final header = payload.substring(5, comma);
      if (!header.toLowerCase().contains(';base64')) {
        throw ArtifactServiceException('unsupported_data_uri_encoding');
      }
      final declaredMime = header.split(';').first.trim();
      if (declaredMime.isNotEmpty) mimeType = declaredMime;
      payload = payload.substring(comma + 1);
    }
    payload = payload.replaceAll(RegExp(r'\s+'), '');
    if (payload.isEmpty) throw ArtifactServiceException('empty_base64_payload');

    try {
      final bytes = base64Decode(payload);
      return service.uploadBytes(
        bytes: bytes,
        kind: kind,
        fileName: fallbackFileName,
        mimeType: mimeType,
        source: 'ui_provider',
      );
    } on FormatException {
      throw ArtifactServiceException('invalid_base64_payload');
    }
  }

  String _artifactDisplayUrl(ArtifactService service, String resourceUri) {
    final artifactId = parseArtifactUri(resourceUri);
    return artifactId == null ? resourceUri : service.contentUrl(artifactId);
  }

  Future<void> _sendProviderImage(Map<dynamic, dynamic> input) async {
    final encoded = input['imageBase64']?.toString().trim() ?? '';
    final resourceUri = input['resourceUri']?.toString().trim() ?? '';
    final text = input['text']?.toString() ?? '';
    final fileName = (input['fileName'] ?? input['filename'] ?? 'image.png')
        .toString();
    final mimeType = (input['mimeType'] ?? input['mime_type'] ?? 'image/png')
        .toString();
    if (encoded.isEmpty && resourceUri.isEmpty) {
      final camera = input['source']?.toString() == 'camera';
      await _pickAndSendImage(camera);
      return;
    }
    await _withArtifactUpload((service) async {
      if (encoded.isNotEmpty) {
        final artifact = await _uploadProviderBase64(
          service,
          encoded: encoded,
          kind: ArtifactKind.image,
          fallbackMimeType: mimeType,
          fallbackFileName: fileName,
        );
        await _runtime.sendImage(
          resourceUri: artifact.resourceUri,
          displayUrl: service.contentUrl(artifact.id),
          fileName: artifact.filename,
          mimeType: artifact.mimeType,
          text: text,
        );
        return;
      }
      await _runtime.sendImage(
        resourceUri: resourceUri,
        displayUrl: _artifactDisplayUrl(service, resourceUri),
        fileName: fileName,
        mimeType: mimeType,
        text: text,
      );
    });
  }

  Future<void> _sendProviderVideo(Map<dynamic, dynamic> input) async {
    final encoded = input['videoBase64']?.toString().trim() ?? '';
    final resourceUri = input['resourceUri']?.toString().trim() ?? '';
    final text = input['text']?.toString() ?? '';
    final fileName = (input['fileName'] ?? input['filename'] ?? 'video.mp4')
        .toString();
    final mimeType = (input['mimeType'] ?? input['mime_type'] ?? 'video/mp4')
        .toString();
    final durationMs =
        int.tryParse(
          (input['durationMs'] ?? input['duration_ms'] ?? '0').toString(),
        ) ??
        0;
    if (encoded.isEmpty && resourceUri.isEmpty) {
      final camera = input['source']?.toString() == 'camera';
      await _pickAndSendVideo(camera);
      return;
    }
    await _withArtifactUpload((service) async {
      if (encoded.isNotEmpty) {
        final artifact = await _uploadProviderBase64(
          service,
          encoded: encoded,
          kind: ArtifactKind.video,
          fallbackMimeType: mimeType,
          fallbackFileName: fileName,
        );
        await _runtime.sendVideo(
          resourceUri: artifact.resourceUri,
          displayUrl: service.contentUrl(artifact.id),
          fileName: artifact.filename,
          mimeType: artifact.mimeType,
          durationMs: artifact.durationMs > 0
              ? artifact.durationMs
              : durationMs,
          text: text,
        );
        return;
      }
      await _runtime.sendVideo(
        resourceUri: resourceUri,
        displayUrl: _artifactDisplayUrl(service, resourceUri),
        fileName: fileName,
        mimeType: mimeType,
        durationMs: durationMs,
        text: text,
      );
    });
  }

  Future<void> _sendProviderFile(Map<dynamic, dynamic> input) async {
    final resourceUri = input['resourceUri']?.toString().trim() ?? '';
    if (resourceUri.isEmpty) {
      await _pickAndSendFile();
      return;
    }
    final fileName = (input['fileName'] ?? input['filename'] ?? '文件')
        .toString()
        .trim();
    final sizeBytes =
        int.tryParse(
          (input['sizeBytes'] ?? input['size_bytes'] ?? '0').toString(),
        ) ??
        0;
    final mimeType =
        (input['mimeType'] ?? input['mime_type'] ?? 'application/octet-stream')
            .toString();
    await _runtime.sendFile(
      resourceUri: resourceUri,
      fileName: fileName.isEmpty ? '文件' : fileName,
      sizeBytes: sizeBytes,
      mimeType: mimeType,
    );
  }

  Future<void> _sendProviderVoice(Map<dynamic, dynamic> input) async {
    final encoded =
        (input['audioBase64'] ?? input['voiceBase64'])?.toString().trim() ?? '';
    final resourceUri = input['resourceUri']?.toString().trim() ?? '';
    final text = input['text']?.toString() ?? '';
    if (encoded.isEmpty && resourceUri.isEmpty) {
      if (text.trim().isNotEmpty) {
        _composerController.text = text;
        _composerController.selection = TextSelection.collapsed(
          offset: _composerController.text.length,
        );
        return;
      }
      await _pickAndSendAudio();
      return;
    }
    final fileName = (input['fileName'] ?? input['filename'] ?? 'voice.webm')
        .toString();
    final mimeType = (input['mimeType'] ?? input['mime_type'] ?? 'audio/webm')
        .toString();
    final durationMs =
        int.tryParse(
          (input['durationMs'] ?? input['duration_ms'] ?? '0').toString(),
        ) ??
        0;
    await _withArtifactUpload((service) async {
      if (encoded.isNotEmpty) {
        final artifact = await _uploadProviderBase64(
          service,
          encoded: encoded,
          kind: ArtifactKind.audio,
          fallbackMimeType: mimeType,
          fallbackFileName: fileName,
        );
        await _runtime.sendVoice(
          resourceUri: artifact.resourceUri,
          displayUrl: service.contentUrl(artifact.id),
          fileName: artifact.filename,
          mimeType: artifact.mimeType,
          durationMs: artifact.durationMs > 0
              ? artifact.durationMs
              : durationMs,
          text: text,
        );
        return;
      }
      await _runtime.sendVoice(
        resourceUri: resourceUri,
        displayUrl: _artifactDisplayUrl(service, resourceUri),
        fileName: fileName,
        mimeType: mimeType,
        durationMs: durationMs,
        text: text,
      );
    });
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

  ({
    Map<String, List<AmitiaAgentActivity>> byMessageId,
    List<AmitiaAgentActivity> unpaired,
  })
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
            !message.time.isAfter(
              event.timestamp.add(const Duration(seconds: 2)),
            )) {
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
        if (!message.time.isBefore(
          event.timestamp.subtract(const Duration(seconds: 2)),
        )) {
          target = message;
          break;
        }
      }

      if (target == null) {
        unpaired.add(activity);
      } else {
        byMessageId
            .putIfAbsent(target.id, () => <AmitiaAgentActivity>[])
            .add(activity);
      }
    }
    for (final activities in byMessageId.values) {
      activities.sort((a, b) => a.time.compareTo(b.time));
    }
    unpaired.sort((a, b) => a.time.compareTo(b.time));
    return (byMessageId: byMessageId, unpaired: unpaired);
  }

  Future<void> _handleCallOption(
    String characterId,
    String characterName,
    String mode,
  ) async {
    switch (mode) {
      case 'voice':
        await _startRealtimeCall(
          characterId,
          characterName,
          mode: RealtimeCallMode.voice,
        );
      case 'video':
        await _startRealtimeCall(
          characterId,
          characterName,
          mode: RealtimeCallMode.video,
        );
      case 'screen':
        await _startRealtimeCall(
          characterId,
          characterName,
          mode: RealtimeCallMode.screen,
        );
    }
  }

  Future<void> _startRealtimeCall(
    String characterId,
    String characterName, {
    RealtimeCallMode mode = RealtimeCallMode.voice,
  }) async {
    var conversationId = _runtime.conversationId;
    if (conversationId == null || conversationId.isEmpty) {
      try {
        final created = await _runtime.createConversation(
          characterId,
          projectId: _runtime.workspace?.projectId ?? '',
        );
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
        initialMode: mode,
      ),
    );
  }

  Future<void> _copyMessage(ChatMessage message) async {
    final text = message.content.trim();
    if (text.isEmpty) return;
    await Clipboard.setData(ClipboardData(text: text));
    if (mounted) amitiaSnackBar(context, '消息已复制');
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
      final url = await ref
          .read(chatServiceProvider)
          .exportConversation(conversationId, format: format);
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

  Future<void> _showProfileSummary(BuildContext context) async {
    final characterId = ref.read(currentCharacterIdProvider).trim();
    final future = ref
        .read(profileServiceProvider)
        .list(characterId: characterId, page: 1, pageSize: 10);
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetContext) => SafeArea(
        child: FractionallySizedBox(
          heightFactor: 0.72,
          child: FutureBuilder<List<ProfileDto>>(
            future: future,
            builder: (context, snapshot) => MobileExtensionSlot(
              slotId: 'chat.profile_summary.panel',
              context: <String, dynamic>{
                'conversationId': _runtime.conversationId ?? '',
                'characterId': characterId,
                'surface': 'profile-summary',
              },
              fallback: _ChatProfileSummarySheet(
                loading: snapshot.connectionState != ConnectionState.done,
                error: snapshot.hasError ? snapshot.error : null,
                profiles: snapshot.data ?? const <ProfileDto>[],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Future<Map<String, dynamic>> _loadChatMemoryContext(
    String conversationId,
    String characterId,
  ) async {
    Future<List<MemoryDto>> loadMemories() async {
      try {
        if (characterId.isEmpty) return const <MemoryDto>[];
        return await ref
            .read(memoryServiceProvider)
            .list(characterId: characterId, page: 1, pageSize: 8);
      } catch (_) {
        return const <MemoryDto>[];
      }
    }

    Future<List<ProfileDto>> loadProfiles() async {
      try {
        return await ref
            .read(profileServiceProvider)
            .list(characterId: characterId, page: 1, pageSize: 5);
      } catch (_) {
        return const <ProfileDto>[];
      }
    }

    Future<Map<String, dynamic>> loadCompression() async {
      try {
        if (conversationId.isEmpty) return const <String, dynamic>{};
        return await ref
                .read(systemServiceProvider)
                .chatCompressionStatus(conversationId) ??
            const <String, dynamic>{};
      } catch (_) {
        return const <String, dynamic>{};
      }
    }

    Future<Map<String, dynamic>> loadPipeline() async {
      try {
        return await ref.read(systemServiceProvider).pipelineStatus() ??
            const <String, dynamic>{};
      } catch (_) {
        return const <String, dynamic>{};
      }
    }

    final values = await Future.wait<dynamic>([
      loadMemories(),
      loadProfiles(),
      loadCompression(),
      loadPipeline(),
    ]);
    return <String, dynamic>{
      'memories': values[0] as List<MemoryDto>,
      'profiles': values[1] as List<ProfileDto>,
      'compression': values[2] as Map<String, dynamic>,
      'pipeline': values[3] as Map<String, dynamic>,
    };
  }

  Future<void> _showMemoryContext(BuildContext context) async {
    final conversationId = _runtime.conversationId?.trim() ?? '';
    final characterId = ref.read(currentCharacterIdProvider).trim();
    final future = _loadChatMemoryContext(conversationId, characterId);
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetContext) => SafeArea(
        child: FractionallySizedBox(
          heightFactor: 0.78,
          child: FutureBuilder<Map<String, dynamic>>(
            future: future,
            builder: (context, snapshot) {
              final data = snapshot.data ?? const <String, dynamic>{};
              return MobileExtensionSlot(
                slotId: 'chat.memory_context.panel',
                context: <String, dynamic>{
                  'conversationId': conversationId,
                  'characterId': characterId,
                  'surface': 'memory-context',
                },
                fallback: _ChatMemoryContextSheet(
                  loading: snapshot.connectionState != ConnectionState.done,
                  error: snapshot.hasError ? snapshot.error : null,
                  memories:
                      (data['memories'] as List<MemoryDto>?) ??
                      const <MemoryDto>[],
                  profiles:
                      (data['profiles'] as List<ProfileDto>?) ??
                      const <ProfileDto>[],
                  compression:
                      (data['compression'] as Map<String, dynamic>?) ??
                      const <String, dynamic>{},
                  pipeline:
                      (data['pipeline'] as Map<String, dynamic>?) ??
                      const <String, dynamic>{},
                ),
              );
            },
          ),
        ),
      ),
    );
  }

  Future<void> _handleChatAction(int result) async {
    switch (result) {
      case 0:
        _showExportSheet(context);
      case 1:
        await _showProfileSummary(context);
      case 2:
        await _showMemoryContext(context);
      case 3:
        final confirmed = await showAmitiaConfirmDialog(
          context,
          title: '清空聊天记录',
          message: '确定要清空当前聊天记录吗？此操作不可撤销。',
          confirmLabel: '清空',
          isDestructive: true,
        );
        if (confirmed == true && mounted) {
          await _clearCurrentConversation();
        }
    }
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

  Map<String, FutureOr<dynamic> Function(dynamic)> _providerActions(
    String characterId,
  ) {
    if (_cachedProviderActions != null &&
        _cachedProviderActionsCharacterId == characterId) {
      return _cachedProviderActions!;
    }
    _cachedProviderActionsCharacterId = characterId;
    return _cachedProviderActions =
        <String, FutureOr<dynamic> Function(dynamic)>{
          ConversationUIAction.send: (input) async {
            if (input is Map) {
              final videoBase64 = input['videoBase64']?.toString().trim() ?? '';
              final imageBase64 = input['imageBase64']?.toString().trim() ?? '';
              if (videoBase64.isNotEmpty) {
                await _sendProviderVideo(input);
                return null;
              }
              if (imageBase64.isNotEmpty) {
                await _sendProviderImage(input);
                return null;
              }
              final text = input['text']?.toString() ?? '';
              if (text.trim().isNotEmpty) _onSend(text);
              return null;
            }
            final text = input?.toString() ?? '';
            if (text.trim().isNotEmpty) _onSend(text);
            return null;
          },
          ConversationUIAction.retry: (input) {
            final id = input is Map
                ? input['messageId']?.toString()
                : input?.toString();
            final index = _runtime.messages.indexWhere(
              (message) => message.id == id,
            );
            if (index >= 0) _retryMessage(index);
            return null;
          },
          ConversationUIAction.regenerate: (input) {
            final id = input is Map
                ? input['messageId']?.toString()
                : input?.toString();
            return _runtime.regenerate(messageId: id);
          },
          ConversationUIAction.stop: (_) => _runtime.stop(),
          ConversationUIAction.delete: (input) async {
            final id = input is Map
                ? input['messageId']?.toString()
                : input?.toString();
            if (id != null && id.isNotEmpty) await _runtime.deleteMessage(id);
            return null;
          },
          ConversationUIAction.newConversation: (_) async {
            _runtime.startDraft();
            _routeSyncedConversationId = '';
            context.go(AppRoutes.chat);
            return null;
          },
          ConversationUIAction.openDrawer: (_) => _openDrawer(context),
          ConversationUIAction.clear: (_) => _clearCurrentConversation(),
          ConversationUIAction.reply: (input) {
            final id = input is Map
                ? input['messageId']?.toString()
                : input?.toString();
            if (id == null || id.isEmpty) return null;
            final message = _runtime.messages
                .where((row) => row.id == id)
                .firstOrNull;
            if (message != null) _setReplyTarget(message);
            return null;
          },
          ConversationUIAction.sendFile: (input) =>
              input is Map ? _sendProviderFile(input) : _pickAndSendFile(),
          ConversationUIAction.sendImage: (input) => input is Map
              ? _sendProviderImage(input)
              : _pickAndSendImage(false),
          ConversationUIAction.sendCode: (input) {
            final row = input is Map ? input : const <String, dynamic>{};
            final language = row['language']?.toString() ?? 'text';
            final code = row['code']?.toString() ?? '';
            if (code.isNotEmpty) _runtime.sendCode(language, code);
            return null;
          },
          ConversationUIAction.sendVoice: (input) =>
              input is Map ? _sendProviderVoice(input) : _pickAndSendAudio(),
          ConversationUIAction.sendEmote: (input) {
            final row = input is Map ? input : const <String, dynamic>{};
            final emoteId = row['emoteId']?.toString() ?? '';
            final displayText =
                row['displayText']?.toString() ?? row['name']?.toString() ?? '';
            if (emoteId.isNotEmpty) _runtime.sendEmote(emoteId, displayText);
            return null;
          },
          ConversationUIAction.chooseWorkspace: (_) => _showWorkspacePicker(),
          ConversationUIAction.selectWorkspace: (input) async {
            final rawWorkspaceId = input is Map ? input['workspaceId'] : input;
            final workspaceId = rawWorkspaceId?.toString().trim() ?? '';
            if (workspaceId.isEmpty) {
              await _showWorkspacePicker();
              return null;
            }
            WorkspaceMountDto? mount = _recentWorkspaces
                .where((item) => item.id == workspaceId)
                .firstOrNull;
            if (mount == null) {
              await _refreshRecentWorkspaces();
              mount = _recentWorkspaces
                  .where((item) => item.id == workspaceId)
                  .firstOrNull;
            }
            if (mount != null) await _selectWorkspaceMount(mount);
            return null;
          },
          ConversationUIAction.clearWorkspace: (_) => _clearWorkspace(),
          ConversationUIAction.refreshWorkspaces: (_) =>
              _refreshRecentWorkspaces(),
        };
  }

  @override
  Widget build(BuildContext context) {
    final selectedCharacterId = ref.watch(currentCharacterIdProvider);
    final characters =
        ref.watch(characterListProvider).valueOrNull ?? const <CharacterDto>[];
    CharacterDto? character = characters
        .where((item) => item.id == selectedCharacterId)
        .firstOrNull;
    character ??= characters.where((item) => item.isActive == 1).firstOrNull;
    character ??= characters.where((item) => item.isDefault).firstOrNull;
    character ??= characters.firstOrNull;
    final characterId = character?.id ?? selectedCharacterId;
    if (characterId.isNotEmpty && characterId != selectedCharacterId) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && ref.read(currentCharacterIdProvider) != characterId) {
          ref.read(currentCharacterIdProvider.notifier).state = characterId;
        }
      });
    }
    if (characterId.isNotEmpty) _runtime.setCharacterId(characterId);

    final characterName = (character?.name ?? '').trim();
    final avatarInitial = characterName.isNotEmpty
        ? characterName.characters.first
        : 'A';
    const avatarColor = '#8A5728';
    final spaceProfile = ref.watch(currentSpaceProfileProvider).valueOrNull;
    final userName = (spaceProfile?.displayName ?? '').trim().isEmpty
        ? '我'
        : spaceProfile!.displayName.trim();
    final userInitial = userName.characters.first;
    const userAvatarColor = '#5F6872';

    final providerContext = _buildProviderContext(
      characterId,
      characterName,
      avatarInitial,
      avatarColor,
    );
    providerContext['user'] = {
      'name': userName,
      'avatarInitial': userInitial,
      'avatarColor': userAvatarColor,
    };
    providerContext['conversationState'] = _runtime.state;
    providerContext['sending'] = _runtime.sending;
    providerContext['conversationId'] = _runtime.conversationId;
    final providerActions = _providerActions(characterId);

    final uiSnapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    final conversationId = _runtime.conversationId?.trim() ?? '';
    final runtimeSessionState = conversationId.isEmpty
        ? null
        : ref
              .watch(clientRuntimeSessionStateProvider(conversationId))
              .valueOrNull;
    bool hasExtensionSlot(String slotId) {
      if (uiSnapshot == null) return false;
      if (uiSnapshot.contributionsForSlot(slotId).isNotEmpty) return true;
      return MobileDynamicRuntime.slotContributions(
        snapshot: uiSnapshot,
        sessionState: runtimeSessionState,
        slotId: slotId,
      ).isNotEmpty;
    }

    bool externalProvider(String capability) {
      final provider = uiSnapshot?.resolve(capability);
      return provider != null && provider.enabled && !provider.builtin;
    }

    final hasSidebarProvider = externalProvider(
      UICapability.conversationSidebar,
    );
    final hasSidebarExtensions = hasExtensionSlot('chat.sidebar.panel');
    final hasOverlayProvider = externalProvider(
      UICapability.conversationOverlay,
    );

    final serializedMessages = _runtime.messages
        .map(_providerMessage)
        .toList(growable: false);
    final durableConversationRecords =
        ref
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
              .where(
                (activity) => !activity.time.isBefore(
                  lastUserTime!.subtract(const Duration(seconds: 2)),
                ),
              )
              .toList(growable: false);
    final hasAssistantAfterLastUser =
        lastUserTime != null &&
        _runtime.messages.any(
          (message) =>
              message.role == MessageRole.assistant &&
              !message.time.isBefore(lastUserTime!),
        );
    final showLiveAgentProcess = _runtime.sending && !hasAssistantAfterLastUser;

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
    final flowItems =
        <_MobileChatFlowItem>[
          for (var index = 0; index < _runtime.messages.length; index++)
            _MobileChatFlowItem.message(
              message: _runtime.messages[index],
              messageIndex: index,
              timestamp: _runtime.messages[index].time,
            ),
          for (final node in conversationNodes) _MobileChatFlowItem.node(node),
        ]..sort((left, right) {
          final order = MobileConversationProjection.compareTimeline(
            left.sequence,
            left.timestamp,
            right.sequence,
            right.timestamp,
          );
          if (order != 0) return order;
          if (left.isMessage != right.isMessage) return left.isMessage ? -1 : 1;
          return left.key.compareTo(right.key);
        });

    final emptyStateSlotCount = flowItems.isEmpty ? 1 : 0;
    final workspaceName = _runtime.workspace?.workspaceName.trim() ?? '';

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
                    child: Stack(
                      children: [
                        ListView.builder(
                          controller: _scrollController,
                          padding: const EdgeInsets.fromLTRB(
                            0,
                            _chatTopBarHeight + 24,
                            0,
                            32,
                          ),
                          itemCount:
                              emptyStateSlotCount +
                              flowItems.length +
                              (showLiveAgentProcess ? 1 : 0),
                          itemBuilder: (context, flowIndex) {
                            if (emptyStateSlotCount == 1 && flowIndex == 0) {
                              return Padding(
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 16,
                                  vertical: 8,
                                ),
                                child: _buildEmptyChatState(
                                  context,
                                  characterName,
                                  workspaceName,
                                ),
                              );
                            }
                            final contentIndex =
                                flowIndex - emptyStateSlotCount;
                            if (contentIndex >= flowItems.length) {
                              return KeyedSubtree(
                                key: const ValueKey(
                                  'message:live-agent-process',
                                ),
                                child: RepaintBoundary(
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
                                ),
                              );
                            }
                            final item = flowItems[contentIndex];
                            if (!item.isMessage) {
                              final node = item.node!;
                              return KeyedSubtree(
                                key: ValueKey(item.key),
                                child: MobileExtensionSlot(
                                  slotId: 'chat.conversation.node',
                                  contributionId: node.contributionId,
                                  context: {
                                    ...providerContext,
                                    'conversationNode': node.toJson(),
                                    'eventType': node.eventType,
                                  },
                                  actions: providerActions,
                                ),
                              );
                            }

                            final index = item.messageIndex!;
                            final message = item.message!;
                            final isAgentTask =
                                message.type == MessageType.agentTask;
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
                                agentActivities:
                                    agentActivityProjection.byMessageId[message
                                        .id] ??
                                    const <AmitiaAgentActivity>[],
                                onRetry: _runtime.canRetryMessage(index)
                                    ? () => _retryMessage(index)
                                    : null,
                                onReply:
                                    message.type == MessageType.systemNotice
                                    ? null
                                    : () => _setReplyTarget(message),
                                onCopy:
                                    message.type == MessageType.systemNotice ||
                                        message.content.trim().isEmpty
                                    ? null
                                    : () => _copyMessage(message),
                                onAgentTaskTap: isAgentTask
                                    ? () {
                                        final taskId =
                                            message.agentTaskId?.trim() ?? '';
                                        context.push(
                                          taskId.isEmpty
                                              ? AppRoutes.agent
                                              : AppRoutes.agentTask(taskId),
                                        );
                                      }
                                    : null,
                              ),
                            );
                            final messageRenderer =
                                UIMessageRendererRegistry.resolve(
                                  uiSnapshot,
                                  messageType: message.type.name,
                                  role: message.role.name,
                                );
                            final providerMessage = UIProviderHost(
                              capability:
                                  UICapability.conversationMessageRenderer,
                              providerId: messageRenderer?.providerId,
                              fallback: builtinMessage,
                              context: {
                                ...providerContext,
                                'message': _providerMessage(message),
                                'messageIndex': index,
                              },
                              actions: providerActions,
                            );
                            final messageContext = <String, dynamic>{
                              ...providerContext,
                              'messageId': message.id,
                              'messageType': message.type.name,
                              'message': _providerMessage(message),
                              'messageIndex': index,
                            };
                            final hasAttachment =
                                (message.resourceUri ?? '').trim().isNotEmpty ||
                                (message.fileName ?? '').trim().isNotEmpty;
                            return KeyedSubtree(
                              key: ValueKey(item.key),
                              child: Column(
                                mainAxisSize: MainAxisSize.min,
                                crossAxisAlignment: CrossAxisAlignment.stretch,
                                children: [
                                  providerMessage,
                                  if (hasAttachment)
                                    Padding(
                                      padding: const EdgeInsets.fromLTRB(
                                        52,
                                        2,
                                        12,
                                        2,
                                      ),
                                      child: MobileExtensionSlot(
                                        slotId:
                                            'chat.message.attachment_renderer',
                                        context: messageContext,
                                        actions: providerActions,
                                      ),
                                    ),
                                  Padding(
                                    padding: const EdgeInsets.fromLTRB(
                                      52,
                                      0,
                                      12,
                                      2,
                                    ),
                                    child: MobileExtensionSlot(
                                      slotId: 'chat.message.badge',
                                      context: messageContext,
                                      actions: providerActions,
                                    ),
                                  ),
                                  Padding(
                                    padding: const EdgeInsets.fromLTRB(
                                      52,
                                      0,
                                      12,
                                      6,
                                    ),
                                    child: MobileExtensionSlot(
                                      slotId: 'chat.message.action',
                                      context: messageContext,
                                      actions: providerActions,
                                    ),
                                  ),
                                ],
                              ),
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
                  UIProviderHost(
                    capability: UICapability.conversationComposer,
                    context: providerContext,
                    actions: providerActions,
                    fallback: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        MobileExtensionSlot(
                          slotId: 'chat.composer.hint',
                          context: {
                            ...providerContext,
                            'surface': 'composer-hint',
                          },
                          actions: providerActions,
                        ),
                        AmitiaChatInput(
                          controller: _composerController,
                          onSend: _onSend,
                          recipientName: characterName,
                          workspaceSelector: _buildWorkspaceBar(context),
                          onPickFile: _pickAndSendFile,
                          onPickImage: _pickAndSendImage,
                          onPickVideo: _pickAndSendVideo,
                          onSendCode: _onSendCode,
                          onLoadEmotes: () =>
                              ref.read(emoteServiceProvider).listEmotes(),
                          onSendEmote: _onSendEmote,
                          onLoadAgentSkills: () => _loadAgentSkills(),
                          onStartVoiceRecording: _startRecordedVoice,
                          onFinishVoiceRecording: _finishRecordedVoice,
                          onCancelVoiceRecording: _cancelRecordedVoice,
                          replyPreview: _replyTarget == null
                              ? null
                              : _replyExcerpt(_replyTarget!),
                          onCancelReply: () => _setReplyTarget(null),
                        ),
                        Padding(
                          padding: const EdgeInsets.fromLTRB(12, 0, 12, 6),
                          child: Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Expanded(
                                child: MobileExtensionSlot(
                                  slotId: 'chat.composer.action',
                                  context: {
                                    ...providerContext,
                                    'surface': 'composer-action',
                                  },
                                  actions: providerActions,
                                ),
                              ),
                              const SizedBox(width: 8),
                              Expanded(
                                child: MobileExtensionSlot(
                                  slotId: 'chat.composer.attachment',
                                  context: {
                                    ...providerContext,
                                    'surface': 'composer-attachment',
                                  },
                                  actions: providerActions,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                  MobileExtensionSlot(
                    slotId: 'chat.status.item',
                    context: {...providerContext, 'surface': 'status'},
                    actions: providerActions,
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
                onCallSelected: (mode) =>
                    _handleCallOption(characterId, characterName, mode),
                onMoreSelected: _handleChatAction,
                extensionActions: MobileExtensionSlot(
                  slotId: 'chat.header.action',
                  context: {...providerContext, 'surface': 'header'},
                  actions: providerActions,
                ),
              ),
            ),
          ),
          if (hasSidebarProvider || hasSidebarExtensions)
            Positioned(
              top: _chatTopBarHeight,
              right: 0,
              bottom: 0,
              child: SizedBox(
                width: MediaQuery.sizeOf(
                  context,
                ).width.clamp(240.0, 360.0).toDouble(),
                child: UIProviderHost(
                  capability: UICapability.conversationSidebar,
                  context: {...providerContext, 'surface': 'sidebar'},
                  actions: providerActions,
                  fallback: MobileExtensionSlot(
                    slotId: 'chat.sidebar.panel',
                    context: {...providerContext, 'surface': 'sidebar'},
                    actions: providerActions,
                  ),
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

class _ChatProfileSummarySheet extends StatelessWidget {
  const _ChatProfileSummarySheet({
    required this.loading,
    required this.error,
    required this.profiles,
  });

  final bool loading;
  final Object? error;
  final List<ProfileDto> profiles;

  String _categoryLabel(String category) {
    const labels = <String, String>{
      'personal_info': '个人信息',
      'preference': '偏好',
      'habit': '习惯',
      'fear': '顾虑',
      'relationship': '关系',
      'health': '健康',
      'plan': '计划',
    };
    return labels[category] ?? (category.isEmpty ? '画像' : category);
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
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
          Text('用户画像摘要', style: AppTypography.pageTitle(context)),
          const SizedBox(height: 4),
          Text('当前角色可用于上下文注入的用户画像事实', style: AppTypography.caption(context)),
          const SizedBox(height: 16),
          Expanded(
            child: loading
                ? const Center(child: CircularProgressIndicator())
                : error != null
                ? Center(
                    child: Text(
                      '画像加载失败：$error',
                      style: AppTypography.bodySmall(
                        context,
                      ).copyWith(color: context.error),
                      textAlign: TextAlign.center,
                    ),
                  )
                : profiles.isEmpty
                ? Center(
                    child: Text(
                      '暂无画像数据。对话完成后系统会自动提取可用画像。',
                      style: AppTypography.caption(context),
                      textAlign: TextAlign.center,
                    ),
                  )
                : ListView.separated(
                    itemCount: profiles.length,
                    separatorBuilder: (_, _) => const SizedBox(height: 8),
                    itemBuilder: (context, index) {
                      final profile = profiles[index];
                      final confidence = profile.confidence.clamp(0, 100);
                      return AmitiaCard(
                        padding: const EdgeInsets.all(12),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Row(
                              children: [
                                Container(
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 8,
                                    vertical: 3,
                                  ),
                                  decoration: BoxDecoration(
                                    color: context.accentPrimary.withValues(
                                      alpha: 0.10,
                                    ),
                                    borderRadius: BorderRadius.circular(999),
                                  ),
                                  child: Text(
                                    _categoryLabel(profile.category),
                                    style: AppTypography.label(
                                      context,
                                    ).copyWith(color: context.accentPrimary),
                                  ),
                                ),
                                const Spacer(),
                                Text(
                                  '置信度 $confidence%',
                                  style: AppTypography.label(context).copyWith(
                                    color: confidence >= 80
                                        ? context.success
                                        : context.warning,
                                  ),
                                ),
                              ],
                            ),
                            const SizedBox(height: 8),
                            Text(
                              profile.attributeName.isEmpty
                                  ? '未命名画像'
                                  : profile.attributeName,
                              style: AppTypography.cardTitle(context),
                            ),
                            if (profile.attributeValue.isNotEmpty) ...[
                              const SizedBox(height: 4),
                              Text(
                                profile.attributeValue,
                                style: AppTypography.bodySmall(context),
                              ),
                            ],
                          ],
                        ),
                      );
                    },
                  ),
          ),
        ],
      ),
    );
  }
}

class _ChatMemoryContextSheet extends StatelessWidget {
  const _ChatMemoryContextSheet({
    required this.loading,
    required this.error,
    required this.memories,
    required this.profiles,
    required this.compression,
    required this.pipeline,
  });

  final bool loading;
  final Object? error;
  final List<MemoryDto> memories;
  final List<ProfileDto> profiles;
  final Map<String, dynamic> compression;
  final Map<String, dynamic> pipeline;

  String _memoryTypeLabel(String type) {
    const labels = <String, String>{
      'fact': '事实',
      'preference': '偏好',
      'episodic': '情景',
      'relationship': '关系',
      'custom': '记忆',
    };
    return labels[type] ?? (type.isEmpty ? '记忆' : type);
  }

  int _asInt(dynamic value) {
    if (value is int) return value;
    if (value is num) return value.round();
    return int.tryParse(value?.toString() ?? '') ?? 0;
  }

  Color _pipelineStatusColor(BuildContext context, String status) {
    switch (status) {
      case 'completed':
        return context.success;
      case 'failed':
      case 'cancelled':
        return context.error;
      case 'skipped':
        return context.textTertiary;
      default:
        return context.accentPrimary;
    }
  }

  @override
  Widget build(BuildContext context) {
    if (loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (error != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(
            '记忆上下文加载失败：$error',
            style: AppTypography.bodySmall(
              context,
            ).copyWith(color: context.error),
            textAlign: TextAlign.center,
          ),
        ),
      );
    }

    final compressedRounds = _asInt(compression['compressedRounds']);
    final totalRounds = _asInt(compression['totalRounds']);
    final lastCompressedAt =
        compression['lastCompressedAt']?.toString().trim() ?? '';
    final rawLayers = pipeline['layers'];
    final layers = rawLayers is List
        ? rawLayers
              .whereType<Map>()
              .map((row) => Map<String, dynamic>.from(row))
              .toList(growable: false)
        : const <Map<String, dynamic>>[];

    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
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
          Text('记忆上下文', style: AppTypography.pageTitle(context)),
          const SizedBox(height: 12),
          Expanded(
            child: ListView(
              children: [
                Text(
                  '相关记忆 (${memories.length})',
                  style: AppTypography.sectionTitle(context),
                ),
                const SizedBox(height: 8),
                if (memories.isEmpty)
                  Text('暂无相关记忆', style: AppTypography.caption(context))
                else
                  ...memories.map(
                    (memory) => Padding(
                      padding: const EdgeInsets.only(bottom: 8),
                      child: AmitiaCard(
                        padding: const EdgeInsets.all(12),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Row(
                              children: [
                                Text(
                                  _memoryTypeLabel(memory.memoryType),
                                  style: AppTypography.label(
                                    context,
                                  ).copyWith(color: context.accentPrimary),
                                ),
                                const Spacer(),
                                Text(
                                  '置信度 ${memory.confidence.clamp(0, 100)}%',
                                  style: AppTypography.label(context),
                                ),
                              ],
                            ),
                            if (memory.key.isNotEmpty) ...[
                              const SizedBox(height: 6),
                              Text(
                                memory.key,
                                style: AppTypography.cardTitle(context),
                              ),
                            ],
                            if (memory.value.isNotEmpty) ...[
                              const SizedBox(height: 4),
                              Text(
                                memory.value,
                                style: AppTypography.bodySmall(context),
                              ),
                            ],
                          ],
                        ),
                      ),
                    ),
                  ),
                const SizedBox(height: 12),
                Text(
                  '用户画像 (${profiles.length})',
                  style: AppTypography.sectionTitle(context),
                ),
                const SizedBox(height: 8),
                if (profiles.isEmpty)
                  Text('暂无用户画像', style: AppTypography.caption(context))
                else
                  AmitiaCard(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 6,
                    ),
                    child: Column(
                      children: profiles
                          .map(
                            (profile) => Padding(
                              padding: const EdgeInsets.symmetric(vertical: 7),
                              child: Row(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Expanded(
                                    child: Text(
                                      '${profile.attributeName}: ${profile.attributeValue}',
                                      style: AppTypography.bodySmall(context),
                                    ),
                                  ),
                                  const SizedBox(width: 8),
                                  Text(
                                    '${profile.confidence.clamp(0, 100)}%',
                                    style: AppTypography.label(context)
                                        .copyWith(
                                          color: profile.confidence >= 80
                                              ? context.success
                                              : context.warning,
                                        ),
                                  ),
                                ],
                              ),
                            ),
                          )
                          .toList(growable: false),
                    ),
                  ),
                const SizedBox(height: 18),
                Text('压缩状态', style: AppTypography.sectionTitle(context)),
                const SizedBox(height: 8),
                AmitiaCard(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '已压缩 $compressedRounds / $totalRounds 轮',
                        style: AppTypography.bodySmall(context),
                      ),
                      if (lastCompressedAt.isNotEmpty) ...[
                        const SizedBox(height: 4),
                        Text(
                          '上次压缩：$lastCompressedAt',
                          style: AppTypography.caption(context),
                        ),
                      ],
                    ],
                  ),
                ),
                const SizedBox(height: 18),
                Text('管线状态', style: AppTypography.sectionTitle(context)),
                const SizedBox(height: 8),
                if (layers.isEmpty)
                  Text('暂无管线状态', style: AppTypography.caption(context))
                else
                  AmitiaCard(
                    padding: const EdgeInsets.all(12),
                    child: Column(
                      children: layers
                          .map((layer) {
                            final status =
                                layer['status']?.toString() ?? 'unknown';
                            final name = layer['name']?.toString() ?? '未命名层';
                            final duration = _asInt(layer['durationMs']);
                            return Padding(
                              padding: const EdgeInsets.symmetric(vertical: 5),
                              child: Row(
                                children: [
                                  Container(
                                    width: 8,
                                    height: 8,
                                    decoration: BoxDecoration(
                                      color: _pipelineStatusColor(
                                        context,
                                        status,
                                      ),
                                      shape: BoxShape.circle,
                                    ),
                                  ),
                                  const SizedBox(width: 8),
                                  Expanded(
                                    child: Text(
                                      name,
                                      style: AppTypography.bodySmall(context),
                                    ),
                                  ),
                                  Text(
                                    '$status · ${duration}ms',
                                    style: AppTypography.label(context),
                                  ),
                                ],
                              ),
                            );
                          })
                          .toList(growable: false),
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

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
  final ValueChanged<String> onCallSelected;
  final ValueChanged<int> onMoreSelected;
  final Widget? extensionActions;

  const _ChatTopBar({
    required this.onOpenDrawer,
    required this.onCallSelected,
    required this.onMoreSelected,
    this.extensionActions,
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
            Tooltip(
              message: '打开侧边栏',
              child: GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTap: onOpenDrawer,
                child: _ChatTopBarButton(
                  icon: isApplePlatform
                      ? CupertinoIcons.line_horizontal_3
                      : Icons.menu_rounded,
                  size: 44,
                  iconSize: 20,
                ),
              ),
            ),
            const Spacer(),
            if (extensionActions != null) ...[
              ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 180),
                child: extensionActions!,
              ),
              const SizedBox(width: 8),
            ],
            PopupMenuButton<String>(
              tooltip: '发起通话',
              onSelected: onCallSelected,
              itemBuilder: (context) => const [
                PopupMenuItem(
                  value: 'video',
                  child: ListTile(
                    contentPadding: EdgeInsets.zero,
                    leading: Icon(Icons.videocam_outlined),
                    title: Text('视频通话'),
                  ),
                ),
                PopupMenuItem(
                  value: 'voice',
                  child: ListTile(
                    contentPadding: EdgeInsets.zero,
                    leading: Icon(Icons.phone_in_talk_outlined),
                    title: Text('语音通话'),
                  ),
                ),
                PopupMenuItem(
                  value: 'screen',
                  child: ListTile(
                    contentPadding: EdgeInsets.zero,
                    leading: Icon(Icons.screen_share_outlined),
                    title: Text('屏幕通话'),
                  ),
                ),
              ],
              child: const _ChatTopBarButton(
                icon: Icons.phone_in_talk_outlined,
                size: 44,
                iconSize: 20,
              ),
            ),
            const SizedBox(width: 8),
            PopupMenuButton<int>(
              tooltip: '当前对话详情',
              onSelected: onMoreSelected,
              itemBuilder: (context) => const [
                PopupMenuItem(
                  value: 0,
                  child: ListTile(
                    contentPadding: EdgeInsets.zero,
                    leading: Icon(Icons.file_download_outlined),
                    title: Text('导出聊天记录'),
                  ),
                ),
                PopupMenuItem(
                  value: 1,
                  child: ListTile(
                    contentPadding: EdgeInsets.zero,
                    leading: Icon(Icons.badge_outlined),
                    title: Text('用户画像摘要'),
                  ),
                ),
                PopupMenuItem(
                  value: 2,
                  child: ListTile(
                    contentPadding: EdgeInsets.zero,
                    leading: Icon(Icons.psychology_alt_outlined),
                    title: Text('记忆上下文'),
                  ),
                ),
                PopupMenuItem(
                  value: 3,
                  child: ListTile(
                    contentPadding: EdgeInsets.zero,
                    leading: Icon(Icons.cleaning_services_outlined),
                    title: Text('清空聊天记录'),
                  ),
                ),
              ],
              child: _ChatTopBarButton(
                icon: isApplePlatform
                    ? CupertinoIcons.ellipsis
                    : Icons.more_horiz,
                size: 44,
                iconSize: 20,
              ),
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

  const _ChatTopBarButton({
    required this.icon,
    required this.size,
    required this.iconSize,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: size,
      height: size,
      child: Center(
        child: Icon(icon, size: iconSize, color: context.textPrimary),
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
    key: 'message:${message.role.name}:${message.renderId}',
    timestamp: timestamp,
    sequence: message.sequence,
    message: message,
    messageIndex: messageIndex,
  );

  factory _MobileChatFlowItem.node(MobileConversationNode node) =>
      _MobileChatFlowItem._(
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
