import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_icons.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_popup_menu.dart';
import '../../../../core/widgets/compact_empty_state.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/models/project.dart';
import '../../../../core/services/chat_service.dart';
import '../../../chat/presentation/widgets/group_chat_invite_sheet.dart';
import '../../../chat/runtime/conversation_runtime_controller.dart';
import '../widgets/conversation_list_tile.dart';
import '../widgets/conversation_swipe_tile.dart';
import '../widgets/project_dialogs.dart';
import '../widgets/project_group_tile.dart';
import '../widgets/project_workspace_picker.dart';

class ConversationListPage extends ConsumerStatefulWidget {
  const ConversationListPage({super.key});

  @override
  ConsumerState<ConversationListPage> createState() =>
      _ConversationListPageState();
}

class _ConversationListPageState extends ConsumerState<ConversationListPage> {
  static const _projectModePreferenceKey = 'conversations_project_mode';
  bool _projectMode = false;
  bool _projectBusy = false;
  final Set<String> _expandedProjects = <String>{};
  Timer? _activityPollTimer;
  late final ConversationRuntimeRegistry _runtimeRegistry;

  @override
  void initState() {
    super.initState();
    _loadViewMode();
    _runtimeRegistry = ref.read(conversationRuntimeRegistryProvider);
    _activityPollTimer = Timer.periodic(
      const Duration(seconds: 2),
      (_) => _pollLiveActivity(),
    );
  }

  @override
  void dispose() {
    _activityPollTimer?.cancel();
    super.dispose();
  }

  void _pollLiveActivity() {
    if (!mounted) return;
    final conversations =
        ref.read(conversationListProvider).valueOrNull ??
        const <ConversationDto>[];
    final sidebar = ref.read(conversationSidebarProvider).valueOrNull;
    final backendActive =
        conversations.any((item) => item.isGenerating) ||
        (sidebar?.projects.any(
              (project) =>
                  project.conversations.any((item) => item.isGenerating),
            ) ??
            false);
    final runtimeActive = _runtimeRegistry.sessions.any(
      (controller) => controller.isGenerating,
    );
    if (!backendActive && !runtimeActive) return;
    ref.invalidate(conversationListProvider);
    ref.invalidate(conversationSidebarProvider);
  }

  Future<void> _loadViewMode() async {
    final preferences = await SharedPreferences.getInstance();
    final projectMode = preferences.getBool(_projectModePreferenceKey) ?? false;
    if (!mounted) return;
    setState(() => _projectMode = projectMode);
  }

  Future<void> _setProjectMode(bool enabled) async {
    if (_projectMode == enabled) return;
    if (enabled) {
      final confirmed = await showAmitiaConfirmDialog(
        context,
        title: '切换为项目模式',
        message: '消息列表将按项目折叠展示，新建对话、新群聊和新项目会使用项目上下文。',
        confirmLabel: '切换',
      );
      if (confirmed != true || !mounted) return;
      ref.invalidate(conversationSidebarProvider);
    }
    setState(() => _projectMode = enabled);
    final preferences = await SharedPreferences.getInstance();
    await preferences.setBool(_projectModePreferenceKey, enabled);
  }

  Future<void> _startNewChat() async {
    var projectId = '';
    if (_projectMode) {
      final sidebar = await ref.read(conversationSidebarProvider.future);
      if (!mounted) return;
      final selection = await showProjectPickerSheet(
        context,
        projects: sidebar.projects,
        title: '选择新对话的项目',
      );
      if (selection == null || !mounted) return;
      projectId = selection.project?.id ?? '';
    }
    var workspace = projectId.isEmpty
        ? null
        : _workspaceBindingForProject(projectId);
    final draftId = ref
        .read(conversationRuntimeRegistryProvider)
        .startDraft(workspace: workspace);
    ref.read(activeConversationIdProvider.notifier).state = '';
    await context.push(
      Uri(
        path: AppRoutes.chat,
        queryParameters: {
          'draftId': draftId,
          if (projectId.isNotEmpty) 'projectId': projectId,
        },
      ).toString(),
    );
  }

  Future<void> _startNewGroupChat() async {
    late final List<CharacterDto> characters;
    try {
      characters = await ref.read(characterListProvider.future);
    } catch (error) {
      if (mounted) {
        amitiaSnackBar(context, '角色加载失败：$error');
      }
      return;
    }
    if (!mounted) return;
    var projects = const <ProjectDto>[];
    if (_projectMode) {
      final sidebar = await ref.read(conversationSidebarProvider.future);
      projects = sidebar.projects;
    }
    if (!mounted) return;
    final result = await createGroupChatWithInvites(
      context,
      ref,
      characters: characters,
      projects: projects,
      showProjectSelector: _projectMode,
    );
    if (result == null || !mounted) return;
    ref.invalidate(conversationListProvider);
    ref.invalidate(conversationSidebarProvider);
    ref.read(conversationCollectionRevisionProvider.notifier).state++;
    await context.push(
      '${AppRoutes.chat}?conversationId=${Uri.encodeQueryComponent(result.group.conversationId)}&characterId=${Uri.encodeQueryComponent(result.baseRoleId)}',
    );
  }

  @override
  Widget build(BuildContext context) {
    final conversationsAsync = ref.watch(conversationListProvider);
    final sidebarAsync = _projectMode
        ? ref.watch(conversationSidebarProvider)
        : null;
    final charactersById = <String, CharacterDto>{
      for (final character
          in ref.watch(characterListProvider).valueOrNull ??
              const <CharacterDto>[])
        if (character.id.isNotEmpty) character.id: character,
    };

    return AmitiaScaffold(
      handlesSystemBack: false,
      appBar: AmitiaAppBar(
        titleWidget: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              _projectMode ? '项目' : '消息',
              style: AppTypography.pageTitle(context),
            ),
            const SizedBox(width: 2),
            IconButton(
              key: const ValueKey('conversation-view-switch'),
              tooltip: _projectMode ? '切换为消息模式' : '切换为项目模式',
              visualDensity: VisualDensity.compact,
              onPressed: () => _setProjectMode(!_projectMode),
              icon: Icon(
                Icons.swap_horiz_rounded,
                size: 21,
                color: _projectMode
                    ? context.accentPrimary
                    : context.textSecondary,
              ),
            ),
          ],
        ),
        backgroundColor: context.surfacePrimary,
        toolbarHeight: AmitiaAppBar.rootPageToolbarHeight,
        leading: const SizedBox.shrink(),
        leadingWidth: 0,
        actions: [
          AmitiaPopupMenuButton<_MessageCreateAction>(
            key: const ValueKey('message-create-button'),
            tooltip: '新建',
            icon: const AmitiaIcon(AmitiaIconGlyph.add, size: 21),
            menuWidth: 140,
            itemVerticalPadding: 0,
            itemFontSize: 14.5,
            itemIconSize: 18,
            itemHorizontalPadding: 14,
            itemHorizontalMargin: 0,
            itemMinHeight: 46,
            onSelected: (action) {
              switch (action) {
                case _MessageCreateAction.chat:
                  _startNewChat();
                case _MessageCreateAction.group:
                  _startNewGroupChat();
                case _MessageCreateAction.project:
                  _showCreateProject();
              }
            },
            itemBuilder: (context) => [
              const PopupMenuItem(
                value: _MessageCreateAction.chat,
                child: AmitiaPopupMenuLabel(
                  icon: Icons.add_comment_outlined,
                  label: '新对话',
                ),
              ),
              const PopupMenuItem(
                value: _MessageCreateAction.group,
                child: AmitiaPopupMenuLabel(
                  icon: Icons.group_add_outlined,
                  label: '新群聊',
                ),
              ),
              if (_projectMode)
                const PopupMenuItem(
                  value: _MessageCreateAction.project,
                  child: AmitiaPopupMenuLabel(
                    icon: Icons.create_new_folder_outlined,
                    label: '新项目',
                  ),
                ),
            ],
          ),
          const SizedBox(width: 4),
        ],
      ),
      body: Column(
        children: [
          Padding(
            key: const ValueKey('message-search-region'),
            padding: EdgeInsets.fromLTRB(
              AppSpacing.pagePadding,
              AppSpacing.md,
              AppSpacing.pagePadding,
              AppSpacing.xs,
            ),
            child: Row(
              children: [
                Expanded(
                  child: Material(
                    key: const ValueKey('message-search-field'),
                    color: Colors.transparent,
                    child: InkWell(
                      onTap: () => context.push(AppRoutes.conversationSearch),
                      borderRadius: BorderRadius.circular(20),
                      overlayColor: const WidgetStatePropertyAll(
                        Colors.transparent,
                      ),
                      splashFactory: NoSplash.splashFactory,
                      child: Ink(
                        height: 40,
                        decoration: BoxDecoration(
                          color: context.surfacePrimary,
                          borderRadius: BorderRadius.circular(20),
                        ),
                        child: Row(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            AmitiaIcon(
                              AmitiaIconGlyph.search,
                              size: 17,
                              color: context.textTertiary,
                            ),
                            const SizedBox(width: 8),
                            Text(
                              '搜索',
                              style: AppTypography.bodySmall(
                                context,
                              ).copyWith(color: context.textTertiary),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
          Expanded(
            child: _projectMode
                ? _buildProjectList(context, sidebarAsync!, charactersById)
                : _buildConversationList(
                    context,
                    conversationsAsync,
                    charactersById,
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildConversationList(
    BuildContext context,
    AsyncValue<List<ConversationDto>> conversationsAsync,
    Map<String, CharacterDto> charactersById,
  ) {
    return conversationsAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => _buildError(err, () {
        ref.invalidate(conversationListProvider);
      }),
      data: (conversations) {
        final filtered = conversations
            .where((conversation) => !_isChannelConversation(conversation))
            .toList();
        filtered.sort(_sortConversations);
        if (filtered.isEmpty) {
          return const AmitiaEmptyState(
            icon: Icons.chat_bubble_outline,
            title: '没有找到对话',
            subtitle: '试试其他关键词，或者新建一个对话',
          );
        }
        return ListView.builder(
          padding: EdgeInsets.only(bottom: AppSpacing.lg),
          itemCount: filtered.length,
          itemBuilder: (context, index) {
            final conversation = filtered[index];
            return ConversationSwipeTile(
              key: ValueKey('conversation-${conversation.id}'),
              pinned: conversation.pinnedAt.trim().isNotEmpty,
              readable: true,
              onPin: () => _setConversationPinned(conversation),
              onArchive: () => _archiveConversation(conversation),
              onMarkRead: () => _markConversationRead(conversation),
              child: ConversationListTile(
                conversation: conversation,
                character: charactersById[conversation.characterId],
                timeText: formatConversationTime(conversation.updatedAt),
                showDivider: index < filtered.length - 1,
                onTap: () => _openConversation(conversation),
                onLongPress: () => _showActionsSheet(conversation),
              ),
            );
          },
        );
      },
    );
  }

  Widget _buildProjectList(
    BuildContext context,
    AsyncValue<ConversationSidebarDto> sidebarAsync,
    Map<String, CharacterDto> charactersById,
  ) {
    return sidebarAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => _buildError(err, () {
        ref.invalidate(conversationSidebarProvider);
      }),
      data: (sidebar) {
        final projects = sidebar.projects
            .where((project) => project.hiddenAt.isEmpty)
            .toList(growable: false);
        projects.sort((a, b) {
          final aPinned = a.pinnedAt.trim().isNotEmpty;
          final bPinned = b.pinnedAt.trim().isNotEmpty;
          if (aPinned != bPinned) return aPinned ? -1 : 1;
          return a.name.compareTo(b.name);
        });
        if (projects.isEmpty) {
          return CompactEmptyState(
            icon: Icons.folder_outlined,
            title: '还没有项目',
            message: '新建项目后，可以把对话和群聊集中到同一个工作区。',
            actionLabel: '新建项目',
            onAction: _showCreateProject,
          );
        }
        return ListView.builder(
          key: const ValueKey('project-conversation-list'),
          padding: EdgeInsets.only(bottom: AppSpacing.lg),
          itemCount: projects.length,
          itemBuilder: (context, index) {
            final project = projects[index];
            return ProjectGroupTile(
              key: ValueKey('project-${project.id}'),
              project: project,
              expanded: _expandedProjects.contains(project.id),
              busy: _projectBusy,
              onToggle: () => setState(() {
                if (!_expandedProjects.remove(project.id)) {
                  _expandedProjects.add(project.id);
                }
              }),
              onOpenConversation: _openConversation,
              onConversationLongPress: _showActionsSheet,
              onConversationPin: _setConversationPinned,
              onConversationArchive: _archiveConversation,
              onConversationMarkRead: _markConversationRead,
              onCreateConversation: () => _startProjectConversation(project),
              onAction: _handleProjectAction,
            );
          },
        );
      },
    );
  }

  Widget _buildError(Object error, VoidCallback retry) {
    if (error.toString().contains('没有可用角色')) {
      return ListView(
        padding: EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          AppSpacing.xs,
          AppSpacing.pagePadding,
          AppSpacing.lg,
        ),
        children: [
          CompactEmptyState(
            icon: Icons.person_add_alt_1_outlined,
            title: '还没有可用角色',
            message: '创建角色后即可开始聊天和创建群聊。',
            actionLabel: '创建角色',
            onAction: () => context.push(AppRoutes.charactersCreate),
          ),
        ],
      );
    }
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            AmitiaIcon(
              AmitiaIconGlyph.warning,
              size: 48,
              color: context.textSecondary,
            ),
            const SizedBox(height: 16),
            Text(
              '加载失败: ${error.toString().replaceFirst('Exception: ', '')}',
              style: AppTypography.body(context).copyWith(color: context.error),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            AmitiaButton(label: '重试', onPressed: retry),
          ],
        ),
      ),
    );
  }

  int _sortConversations(ConversationDto a, ConversationDto b) {
    final aPinned = a.pinnedAt.trim().isNotEmpty;
    final bPinned = b.pinnedAt.trim().isNotEmpty;
    if (aPinned != bPinned) return aPinned ? -1 : 1;
    final aTime =
        DateTime.tryParse(a.updatedAt) ??
        DateTime.fromMillisecondsSinceEpoch(0);
    final bTime =
        DateTime.tryParse(b.updatedAt) ??
        DateTime.fromMillisecondsSinceEpoch(0);
    return bTime.compareTo(aTime);
  }

  void _openConversation(ConversationDto conversation) {
    context.push(
      '${AppRoutes.chat}?conversationId=${Uri.encodeQueryComponent(conversation.id)}',
    );
  }

  ProjectDto? _projectById(String id) {
    final sidebar = ref.read(conversationSidebarProvider).valueOrNull;
    if (sidebar == null) return null;
    for (final project in <ProjectDto>[
      ...sidebar.projects,
      ...sidebar.removedProjects,
    ]) {
      if (project.id == id) return project;
    }
    return null;
  }

  ConversationWorkspaceDto? _workspaceBindingForProject(String projectId) {
    final project = _projectById(projectId);
    if (project == null ||
        project.readOnly ||
        !project.available ||
        project.hiddenAt.isNotEmpty ||
        project.workspaces.isEmpty) {
      return null;
    }
    final primary = project.workspaces
        .where((workspace) => workspace.id == project.primaryWorkspaceId)
        .firstOrNull;
    if (primary == null) return null;
    return ConversationWorkspaceDto(
      projectId: project.id,
      workspaceId: primary.id,
      deviceId: primary.deviceId,
      workspaceName: project.name,
      workspaceKind: primary.kind,
      rootUri: primary.rootUri,
      workspaces: project.workspaces,
      primaryWorkspaceId: primary.id,
    );
  }

  Future<void> _startProjectConversation(ProjectDto project) async {
    if (project.readOnly ||
        !project.available ||
        project.hiddenAt.isNotEmpty ||
        project.workspaces.isEmpty) {
      amitiaSnackBar(context, '该项目当前不可用于新对话');
      return;
    }
    final workspace = _workspaceBindingForProject(project.id);
    if (workspace == null) {
      amitiaSnackBar(context, '项目主工作区不可用');
      return;
    }
    final draftId = ref
        .read(conversationRuntimeRegistryProvider)
        .startDraft(workspace: workspace);
    ref.read(activeConversationIdProvider.notifier).state = '';
    await context.push(
      Uri(
        path: AppRoutes.chat,
        queryParameters: {'draftId': draftId, 'projectId': project.id},
      ).toString(),
    );
  }

  Future<void> _showCreateProject() async {
    if (_projectBusy) return;
    setState(() => _projectBusy = true);
    try {
      final project = await showCreateProjectSheet(context, ref);
      if (project == null || !mounted) return;
      await _refreshProjectCollections();
      if (!mounted) return;
      setState(() => _expandedProjects.add(project.id));
      amitiaSnackBar(context, '项目已创建');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '创建项目失败：$error');
    } finally {
      if (mounted) setState(() => _projectBusy = false);
    }
  }

  Future<void> _handleProjectAction(ProjectDto project, String action) async {
    if (_projectBusy) return;
    try {
      if (action == 'newChat') {
        await _startProjectConversation(project);
        return;
      }
      if (action == 'rename') {
        await _renameProject(project);
        return;
      }
      if (action == 'addWorkspace') {
        await _addProjectWorkspace(project);
        return;
      }
      if (action == 'pin') {
        setState(() => _projectBusy = true);
        await ref
            .read(chatServiceProvider)
            .updateProject(
              project.id,
              pinned: project.pinnedAt.isEmpty,
              project: project,
            );
        await _refreshProjectCollections();
        return;
      }
      if (action == 'browse') {
        final primary = project.workspaces
            .where((item) => item.id == project.primaryWorkspaceId)
            .firstOrNull;
        if (primary == null) {
          amitiaSnackBar(context, '项目没有可用主工作区');
          return;
        }
        await context.push(
          '${AppRoutes.toolboxFileBrowser}?rootUri=${Uri.encodeQueryComponent(primary.rootUri)}',
        );
        return;
      }
      if (action == 'remove') {
        final confirmed = await showAmitiaConfirmDialog(
          context,
          title: '从消息列表移除',
          message: '项目和其中的对话会被保留，只从项目模式列表中隐藏。',
          confirmLabel: '移除',
          isDestructive: true,
        );
        if (confirmed != true || !mounted) return;
        setState(() => _projectBusy = true);
        await ref
            .read(chatServiceProvider)
            .hideProject(project.id, project: project);
        _expandedProjects.remove(project.id);
        await _refreshProjectCollections();
        return;
      }
      if (action.startsWith('primary:')) {
        final workspaceId = action.substring('primary:'.length);
        if (!project.workspaces.any((item) => item.id == workspaceId)) return;
        setState(() => _projectBusy = true);
        await ref
            .read(chatServiceProvider)
            .updateProject(
              project.id,
              primaryWorkspaceId: workspaceId,
              project: project,
            );
        await _refreshProjectCollections();
        return;
      }
      if (action.startsWith('removeWorkspace:')) {
        final workspaceId = action.substring('removeWorkspace:'.length);
        final workspace = project.workspaces
            .where((item) => item.id == workspaceId)
            .firstOrNull;
        if (workspace == null) return;
        final confirmed = await showAmitiaConfirmDialog(
          context,
          title: '移除工作区',
          message: '确定从项目「${project.name}」移除工作区「${workspace.name}」吗？',
          confirmLabel: '移除',
          isDestructive: true,
        );
        if (confirmed != true || !mounted) return;
        final remaining = project.workspaces
            .where((item) => item.id != workspaceId)
            .toList(growable: false);
        setState(() => _projectBusy = true);
        await ref
            .read(chatServiceProvider)
            .updateProject(
              project.id,
              workspaces: remaining,
              primaryWorkspaceId: project.primaryWorkspaceId == workspaceId
                  ? (remaining.firstOrNull?.id ?? '')
                  : project.primaryWorkspaceId,
              project: project,
            );
        await _refreshProjectCollections();
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '项目操作失败：$error');
    } finally {
      if (mounted && _projectBusy) setState(() => _projectBusy = false);
    }
  }

  Future<void> _renameProject(ProjectDto project) async {
    final controller = TextEditingController(text: project.name);
    final name = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: dialogContext.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('重命名项目', style: AppTypography.cardTitle(dialogContext)),
        content: AmitiaTextField(controller: controller, hintText: '输入新的项目名称'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () {
              final value = controller.text.trim();
              if (value.isNotEmpty) Navigator.of(dialogContext).pop(value);
            },
            child: const Text('保存'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (name == null || name == project.name || !mounted) return;
    setState(() => _projectBusy = true);
    try {
      await ref
          .read(chatServiceProvider)
          .updateProject(project.id, name: name, project: project);
      await _refreshProjectCollections();
      if (mounted) amitiaSnackBar(context, '项目已重命名');
    } finally {
      if (mounted) setState(() => _projectBusy = false);
    }
  }

  Future<void> _addProjectWorkspace(ProjectDto project) async {
    setState(() => _projectBusy = true);
    try {
      final mount = await pickAndRegisterProjectWorkspace(ref);
      if (mount == null || !mounted) return;
      if (project.workspaces.any((item) => item.id == mount.id)) {
        throw StateError('该工作区已属于当前项目');
      }
      final expected = await ref
          .read(chatServiceProvider)
          .owned
          .data('memory', characterId: project.roleId);
      final scope = Map<String, dynamic>.from(
        expected['executionScope'] as Map,
      );
      final deviceId = (scope['targetDeviceId'] ?? '').toString().trim();
      if (deviceId.isEmpty) throw StateError('无法确认工作区所属设备');
      final workspace = ProjectWorkspaceDto(
        id: mount.id,
        deviceId: deviceId,
        name: mount.name.trim().isEmpty ? '工作目录' : mount.name.trim(),
        kind: mount.kind,
        rootUri: mount.rootUri,
        readOnly: mount.readOnly,
      );
      await ref
          .read(chatServiceProvider)
          .updateProject(
            project.id,
            workspaces: <ProjectWorkspaceDto>[...project.workspaces, workspace],
            primaryWorkspaceId: project.primaryWorkspaceId.isEmpty
                ? workspace.id
                : project.primaryWorkspaceId,
            project: project,
          );
      await _refreshProjectCollections();
      if (mounted) amitiaSnackBar(context, '副工作区已添加');
    } finally {
      if (mounted) setState(() => _projectBusy = false);
    }
  }

  Future<void> _refreshProjectCollections() async {
    ref.invalidate(conversationListProvider);
    ref.invalidate(conversationSidebarProvider);
    ref.read(conversationCollectionRevisionProvider.notifier).state++;
    await ref.read(conversationSidebarProvider.future);
  }

  void _showActionsSheet(ConversationDto conv) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.large),
        ),
      ),
      builder: (sheetContext) {
        return SafeArea(
          child: Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.lg,
              0,
              AppSpacing.lg,
              AppSpacing.lg,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _SheetActionItem(
                  icon: Icons.edit_outlined,
                  label: '重命名',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _renameConversation(conv);
                  },
                ),
                _SheetActionItem(
                  icon: Icons.drive_file_move_outline,
                  label: conv.projectId.isEmpty ? '移动到项目' : '调整所属项目',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showMoveConversationSheet(conv);
                  },
                ),
                _SheetActionItem(
                  icon: Icons.delete_outline,
                  label: '删除',
                  iconColor: context.error,
                  textColor: context.error,
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _deleteConversation(conv);
                  },
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Future<void> _showMoveConversationSheet(ConversationDto conversation) async {
    final sidebar = await ref.read(conversationSidebarProvider.future);
    if (!mounted) return;
    final projects = sidebar.projects
        .where(
          (project) =>
              !project.readOnly &&
              project.hiddenAt.isEmpty &&
              project.roleId == conversation.characterId &&
              project.id != conversation.projectId,
        )
        .toList(growable: false);
    final target = await showModalBottomSheet<String>(
      context: context,
      useSafeArea: true,
      showDragHandle: true,
      backgroundColor: context.surfacePrimary,
      builder: (sheetContext) => ConstrainedBox(
        constraints: BoxConstraints(
          maxHeight: MediaQuery.sizeOf(sheetContext).height * 0.7,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.lg,
                0,
                AppSpacing.lg,
                AppSpacing.sm,
              ),
              child: Text(
                '选择所属项目',
                style: AppTypography.pageTitle(sheetContext),
              ),
            ),
            if (conversation.projectId.isNotEmpty)
              ListTile(
                leading: const Icon(Icons.chat_bubble_outline),
                title: const Text('移出项目'),
                subtitle: const Text('保留对话并移回普通消息列表'),
                onTap: () => Navigator.of(sheetContext).pop(''),
              ),
            if (projects.isEmpty)
              Padding(
                padding: EdgeInsets.fromLTRB(
                  AppSpacing.lg,
                  AppSpacing.md,
                  AppSpacing.lg,
                  AppSpacing.xl,
                ),
                child: Text('暂无可移动的其他项目'),
              )
            else
              Flexible(
                child: ListView.builder(
                  shrinkWrap: true,
                  itemCount: projects.length,
                  itemBuilder: (context, index) {
                    final project = projects[index];
                    return ListTile(
                      leading: const Icon(Icons.folder_outlined),
                      title: Text(project.name),
                      subtitle: Text(
                        project.workspaces.isEmpty
                            ? '未设置工作区'
                            : '${project.workspaces.length} 个工作区',
                      ),
                      onTap: () => Navigator.of(sheetContext).pop(project.id),
                    );
                  },
                ),
              ),
          ],
        ),
      ),
    );
    if (target == null || !mounted) return;
    if (target == conversation.projectId) return;
    try {
      await ref
          .read(chatServiceProvider)
          .moveConversationToProject(conversation.id, target);
      await _refreshProjectCollections();
      if (mounted) {
        amitiaSnackBar(context, target.isEmpty ? '对话已移出项目' : '对话已移入项目');
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '移动对话失败：$error');
    }
  }

  Future<void> _setConversationPinned(ConversationDto conversation) async {
    await _runConversationAction(
      () => ref
          .read(chatServiceProvider)
          .setConversationPinned(
            conversation.id,
            conversation.pinnedAt.trim().isEmpty,
          ),
      success: conversation.pinnedAt.trim().isEmpty ? '对话已置顶' : '已取消置顶',
    );
  }

  Future<void> _archiveConversation(ConversationDto conversation) async {
    await _runConversationAction(
      () => ref.read(chatServiceProvider).archiveConversation(conversation.id),
      success: '对话已归档',
    );
  }

  Future<void> _markConversationRead(ConversationDto conversation) async {
    await _runConversationAction(
      () => ref
          .read(chatServiceProvider)
          .markConversationRead(
            conversation.id,
            lastTerminalTurnSequence: conversation.lastTerminalTurnSequence,
            characterId: conversation.characterId,
          ),
      success: '已标为已读',
      clearRuntimeUnread: conversation.id,
    );
  }

  Future<void> _runConversationAction(
    Future<void> Function() action, {
    required String success,
    String clearRuntimeUnread = '',
  }) async {
    try {
      await action();
      if (clearRuntimeUnread.isNotEmpty) {
        ref
            .read(conversationRuntimeRegistryProvider)
            .clearBackgroundCompletions(clearRuntimeUnread);
      }
      ref.invalidate(conversationListProvider);
      ref.invalidate(conversationSidebarProvider);
      ref.read(conversationCollectionRevisionProvider.notifier).state++;
      if (mounted) amitiaSnackBar(context, success);
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '操作失败：$error');
      rethrow;
    }
  }

  Future<void> _renameConversation(ConversationDto conv) async {
    final controller = TextEditingController(text: conv.title);
    final title = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: dialogContext.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('重命名对话', style: AppTypography.cardTitle(dialogContext)),
        content: AmitiaTextField(controller: controller, hintText: '输入新的对话标题'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () {
              final value = controller.text.trim();
              if (value.isNotEmpty) Navigator.pop(dialogContext, value);
            },
            child: const Text('保存'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (title == null || title == conv.title) return;
    try {
      await ref.read(chatServiceProvider).renameConversation(conv.id, title);
      ref.invalidate(conversationListProvider);
      ref.invalidate(conversationSidebarProvider);
      if (mounted) amitiaSnackBar(context, '对话已重命名');
    } catch (e) {
      if (mounted) amitiaSnackBar(context, '重命名失败：$e');
    }
  }

  Future<void> _deleteConversation(ConversationDto conv) async {
    final confirmed = await showAmitiaConfirmDialog(
      context,
      title: '删除对话',
      message: '确定删除「${conv.title}」及其聊天记录吗？此操作不可撤销。',
      confirmLabel: '删除',
      isDestructive: true,
    );
    if (confirmed != true) return;
    try {
      await ref.read(chatServiceProvider).deleteConversation(conv.id);
      ref.invalidate(conversationListProvider);
      ref.invalidate(conversationSidebarProvider);
      if (mounted) amitiaSnackBar(context, '对话已删除');
    } catch (e) {
      if (mounted) amitiaSnackBar(context, '删除失败：$e');
    }
  }
}

enum _MessageCreateAction { chat, group, project }

class _SheetActionItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color? iconColor;
  final Color? textColor;
  final VoidCallback onTap;

  const _SheetActionItem({
    required this.icon,
    required this.label,
    required this.onTap,
    this.iconColor,
    this.textColor,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 14),
        child: Row(
          children: [
            AmitiaIcon.adaptive(
              icon,
              size: 22,
              color: iconColor ?? context.textSecondary,
            ),
            const SizedBox(width: 16),
            Text(
              label,
              style: AppTypography.body(context).copyWith(color: textColor),
            ),
          ],
        ),
      ),
    );
  }
}

bool _isChannelConversation(ConversationDto conversation) {
  final channel = conversation.channel.trim().toLowerCase();
  if (channel.isNotEmpty && channel != 'web') return true;
  final id = conversation.sourceResourceId.trim().isNotEmpty
      ? conversation.sourceResourceId.trim()
      : conversation.id;
  return id.startsWith('conv-qq-personal-') ||
      id.startsWith('conv-group-') ||
      id.startsWith('wechat-ipad860-') ||
      id.contains('@im.wechat') ||
      id.contains('@chatroom');
}
