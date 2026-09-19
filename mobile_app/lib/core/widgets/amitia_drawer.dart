import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';
import 'package:go_router/go_router.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_motion.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_typography.dart';
import '../../app/theme/design_tokens.dart';
import '../../app/app_routes.dart';
import '../../app/drawer_route_state.dart';
import '../models/character.dart';
import '../settings/appearance_preferences.dart';
import '../services/extension_service.dart';
import '../services/providers.dart';
import '../services/chat_service.dart';
import '../services/workspace_service.dart';
import '../native_bridge/providers/native_bridge_relay_provider.dart';
import '../models/project.dart';
import '../models/conversation.dart';
import '../../features/chat/runtime/conversation_runtime_controller.dart';
import '../ui_runtime/ui_navigation_registry.dart';
import '../ui_runtime/ui_runtime_controller.dart';
import 'amitia_misc.dart';
import 'profile_avatar.dart';

final currentCharacterIdProvider = StateProvider<String>((ref) => '');
final isDeveloperModeProvider = StateProvider<bool>((ref) => false);
final _installedExtensionViewProvider =
    FutureProvider.autoDispose<ExtensionCenterView>((ref) async {
      final service = ref.read(extensionServiceProvider);
      return service.getExtensionCenterView();
    });

class _CharInfo {
  final String name;
  final String avatarInitial;

  const _CharInfo(this.name, this.avatarInitial);

  factory _CharInfo.fromCharacter(CharacterDto character) {
    final name = character.name.trim().isEmpty
        ? '未命名角色'
        : character.name.trim();
    return _CharInfo(name, name.characters.first);
  }
}

class AmitiaDrawer extends ConsumerStatefulWidget {
  final String currentRoute;

  const AmitiaDrawer({super.key, required this.currentRoute});

  @override
  ConsumerState<AmitiaDrawer> createState() => _AmitiaDrawerState();
}

class _AmitiaDrawerState extends ConsumerState<AmitiaDrawer> {
  void _navigateTo(String route) {
    final router = GoRouter.of(context);
    final currentRoute = router.routerDelegate.currentConfiguration.fullPath;
    Navigator.of(context).pop();
    final action = resolveDrawerNavigationAction(
      currentLocation: currentRoute,
      targetLocation: route,
    );
    switch (action) {
      case DrawerNavigationAction.none:
        return;
      case DrawerNavigationAction.push:
        router.push(route);
        return;
      case DrawerNavigationAction.replace:
        router.go(route);
        return;
    }
  }

  Future<void> _showGlobalSearch(
    List<UINavigationItem> navigationItems,
    List<CharacterDto> characters,
  ) async {
    final pages = <_DrawerSearchPage>[
      for (final item in navigationItems)
        _DrawerSearchPage(item.label, item.route, item.icon),
      const _DrawerSearchPage(
        '会话列表',
        AppRoutes.conversations,
        Icons.forum_outlined,
      ),
      const _DrawerSearchPage(
        '日程提醒',
        AppRoutes.reminders,
        Icons.notifications_none,
      ),
      const _DrawerSearchPage(
        '记忆总览',
        AppRoutes.memory,
        Icons.psychology_outlined,
      ),
      const _DrawerSearchPage(
        '情景记忆',
        AppRoutes.memoryEpisodic,
        Icons.auto_stories_outlined,
      ),
      const _DrawerSearchPage(
        '记忆图谱',
        AppRoutes.memoryGraph,
        Icons.hub_outlined,
      ),
      const _DrawerSearchPage(
        '时间线',
        AppRoutes.memoryTimeline,
        Icons.timeline_outlined,
      ),
      const _DrawerSearchPage(
        '用户画像',
        AppRoutes.memoryProfiles,
        Icons.person_search_outlined,
      ),
      const _DrawerSearchPage(
        '世界书',
        AppRoutes.memoryWorldBook,
        Icons.menu_book_outlined,
      ),
      const _DrawerSearchPage(
        '导入记录',
        AppRoutes.chatImport,
        Icons.file_upload_outlined,
      ),
      const _DrawerSearchPage(
        '表情管理',
        AppRoutes.emotes,
        Icons.emoji_emotions_outlined,
      ),
      const _DrawerSearchPage(
        '设置',
        AppRoutes.settings,
        Icons.settings_outlined,
      ),
    ];
    final result = await showSearch<_DrawerSearchResult>(
      context: context,
      delegate: _AmitiaDrawerSearchDelegate(
        pages: pages,
        characters: characters,
      ),
    );
    if (!mounted || result == null || result.route.isEmpty) return;
    if (result.characterId != null) {
      ref.read(currentCharacterIdProvider.notifier).state = result.characterId!;
    }
    _navigateTo(result.route);
  }

  Future<void> _refreshConversationSidebar() async {
    ref.invalidate(conversationListProvider);
    ref.invalidate(conversationSidebarProvider);
    await ref.read(conversationSidebarProvider.future);
  }

  Future<ConversationSidebarDto> _loadConversationSidebar() {
    return ref.read(conversationSidebarProvider.future);
  }

  void _replaceActiveProjectWorkspace(
    ProjectDto project, {
    String? name,
    String? workspaceId,
    String? rootUri,
  }) {
    final runtime = ref.read(conversationRuntimeControllerProvider);
    final workspace = runtime.workspace;
    if (workspace == null || workspace.projectId != project.id) return;
    runtime.setWorkspace(
      ConversationWorkspaceDto(
        conversationId: workspace.conversationId,
        projectId: project.id,
        workspaceId: workspaceId ?? workspace.workspaceId,
        deviceId: workspace.deviceId,
        workspaceName: name ?? workspace.workspaceName,
        workspaceKind: workspace.workspaceKind,
        rootUri: rootUri ?? workspace.rootUri,
      ),
    );
  }

  Future<void> _createConversation({String projectId = ''}) async {
    final id = projectId.trim();
    ConversationWorkspaceDto? workspace;
    if (id.isNotEmpty) {
      try {
        final sidebar = await _loadConversationSidebar();
        final project = sidebar.projects
            .where((item) => item.id == id)
            .firstOrNull;
        if (project == null) throw StateError('项目不存在');
        if (!project.available) {
          throw StateError(
            project.statusReason.trim().isEmpty
                ? '项目目录当前不可用'
                : project.statusReason,
          );
        }
        workspace = ConversationWorkspaceDto(
          projectId: project.id,
          workspaceId: project.workspaceId,
          deviceId: project.deviceId,
          workspaceName: project.name,
          workspaceKind: project.rootUri.startsWith('content://')
              ? 'saf'
              : 'local',
          rootUri: project.rootUri,
        );
      } catch (error) {
        if (mounted) amitiaSnackBar(context, '新建项目对话失败：$error');
        return;
      }
    }
    if (!mounted) return;
    ref
        .read(conversationRuntimeControllerProvider)
        .startDraft(workspace: workspace);
    ref.read(activeConversationIdProvider.notifier).state = '';
    _navigateTo(
      id.isEmpty
          ? AppRoutes.chat
          : '${AppRoutes.chat}?projectId=${Uri.encodeQueryComponent(id)}',
    );
  }

  Future<WorkspaceMountDto?> _pickWorkspaceMount() async {
    if (defaultTargetPlatform == TargetPlatform.android) {
      final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
      final response = await dispatcher.execute(<String, dynamic>{
        'protocolVersion': 1,
        'requestId': 'project-picker-${DateTime.now().microsecondsSinceEpoch}',
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
            name: (result['name'] ?? '项目').toString(),
            grantId: grantId,
            readOnly: result['readOnly'] == true,
          );
    }
    final path = await FilePicker.platform.getDirectoryPath(
      dialogTitle: '选择项目文件夹',
    );
    final localRoot = path?.trim() ?? '';
    if (localRoot.isEmpty) return null;
    final segments = localRoot
        .replaceAll('\\', '/')
        .split('/')
        .where((segment) => segment.trim().isNotEmpty)
        .toList(growable: false);
    return ref
        .read(workspaceServiceProvider)
        .registerLocal(
          name: segments.isEmpty ? '项目' : segments.last,
          localRoot: localRoot,
        );
  }

  Future<void> _addProject() async {
    try {
      final mount = await _pickWorkspaceMount();
      if (mount == null || !mounted) return;
      final sidebar = await _loadConversationSidebar();
      if (!sidebar.projects.any((project) => project.workspaceId == mount.id)) {
        await ref
            .read(chatServiceProvider)
            .createProject(
              name: mount.name.trim().isEmpty ? '项目' : mount.name,
              workspaceId: mount.id,
              rootUri: mount.rootUri,
            );
      }
      await _refreshConversationSidebar();
      if (mounted) amitiaSnackBar(context, '项目已添加');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '添加项目失败：$error');
    }
  }

  Future<void> _removeProject(ProjectDto project) async {
    final confirmed = await showAmitiaConfirmDialog(
      context,
      title: '移除项目',
      message: '移除“${project.name}”项目？项目中的对话会移到最近，不会删除聊天记录。',
      confirmLabel: '移除',
      isDestructive: true,
    );
    if (confirmed != true) return;
    try {
      await ref.read(chatServiceProvider).deleteProject(project.id);
      final runtime = ref.read(conversationRuntimeControllerProvider);
      if (runtime.workspace?.projectId == project.id) {
        runtime.setWorkspace(null);
      }
      await _refreshConversationSidebar();
      if (mounted) amitiaSnackBar(context, '项目已移除');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '移除项目失败：$error');
    }
  }

  Future<void> _toggleProjectPin(ProjectDto project) async {
    try {
      await ref
          .read(chatServiceProvider)
          .updateProject(project.id, pinned: project.pinnedAt.isEmpty);
      await _refreshConversationSidebar();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '更新项目置顶失败：$error');
    }
  }

  Future<void> _openProject(ProjectDto project) async {
    try {
      final target = await ref
          .read(chatServiceProvider)
          .projectLocation(project.id);
      final path = (target['path'] ?? '').toString().trim();
      final uri = (target['uri'] ?? '').toString().trim();
      if (path.isEmpty && uri.isEmpty) {
        throw StateError('项目目录位置不可用');
      }
      final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
      final response = await dispatcher.execute(<String, dynamic>{
        'protocolVersion': 1,
        'requestId': 'project-open-${DateTime.now().microsecondsSinceEpoch}',
        'platform': 'android',
        'operation': 'workspace.open',
        'payload': <String, dynamic>{
          if (path.isNotEmpty) 'path': path,
          if (uri.isNotEmpty) 'uri': uri,
        },
      });
      if ((response['status'] ?? '').toString() != 'success') {
        final rawError = response['error'];
        final error = rawError is Map
            ? Map<String, dynamic>.from(rawError)
            : const <String, dynamic>{};
        throw StateError((error['message'] ?? '打开项目目录失败').toString());
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '打开项目目录失败：$error');
    }
  }

  Future<void> _renameProject(ProjectDto project) async {
    final controller = TextEditingController(text: project.name);
    final name = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('重命名项目'),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(hintText: '输入项目名称'),
          onSubmitted: (value) {
            final text = value.trim();
            if (text.isNotEmpty) Navigator.pop(dialogContext, text);
          },
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () {
              final text = controller.text.trim();
              if (text.isNotEmpty) Navigator.pop(dialogContext, text);
            },
            child: const Text('保存'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (name == null || name == project.name) return;
    try {
      await ref.read(chatServiceProvider).updateProject(project.id, name: name);
      _replaceActiveProjectWorkspace(project, name: name);
      await _refreshConversationSidebar();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '重命名项目失败：$error');
    }
  }

  Future<void> _changeProjectRoot(ProjectDto project) async {
    try {
      final mount = await _pickWorkspaceMount();
      if (mount == null || !mounted) return;
      await ref
          .read(chatServiceProvider)
          .updateProject(
            project.id,
            workspaceId: mount.id,
            rootUri: mount.rootUri,
          );
      _replaceActiveProjectWorkspace(
        project,
        workspaceId: mount.id,
        rootUri: mount.rootUri,
      );
      await _refreshConversationSidebar();
      if (mounted) amitiaSnackBar(context, '项目根目录已更新');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '更新项目根目录失败：$error');
    }
  }

  Future<void> _renameConversation(ConversationDto conversation) async {
    final controller = TextEditingController(text: conversation.title);
    final title = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('重命名对话'),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(hintText: '输入对话名称'),
          onSubmitted: (value) {
            final text = value.trim();
            if (text.isNotEmpty) Navigator.pop(dialogContext, text);
          },
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () {
              final text = controller.text.trim();
              if (text.isNotEmpty) Navigator.pop(dialogContext, text);
            },
            child: const Text('保存'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (title == null || title == conversation.title) return;
    try {
      await ref
          .read(chatServiceProvider)
          .renameConversation(conversation.id, title);
      await _refreshConversationSidebar();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '重命名失败：$error');
    }
  }

  Future<void> _toggleConversationPin(ConversationDto conversation) async {
    try {
      await ref
          .read(chatServiceProvider)
          .setConversationPinned(
            conversation.id,
            conversation.pinnedAt.isEmpty,
          );
      await _refreshConversationSidebar();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '更新置顶失败：$error');
    }
  }

  Future<void> _archiveConversation(ConversationDto conversation) async {
    try {
      final archivingActive =
          ref.read(activeConversationIdProvider).trim() == conversation.id;
      await ref.read(chatServiceProvider).archiveConversation(conversation.id);
      ref.read(conversationCollectionRevisionProvider.notifier).state++;
      await _refreshConversationSidebar();
      if (!mounted) return;
      if (archivingActive) {
        ref.read(conversationRuntimeControllerProvider).startDraft();
        ref.read(activeConversationIdProvider.notifier).state = '';
        _navigateTo(AppRoutes.chat);
      }
      amitiaSnackBar(context, '对话已归档');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '归档失败：$error');
    }
  }

  @override
  Widget build(BuildContext context) {
    final appearance = ref.watch(appearancePreferencesProvider);
    final platformBrightness = MediaQuery.platformBrightnessOf(context);
    final isDark =
        appearance.themeMode == ThemeMode.dark ||
        (appearance.themeMode == ThemeMode.system &&
            platformBrightness == Brightness.dark);
    final characterId = ref.watch(currentCharacterIdProvider);
    final characters =
        ref.watch(characterListProvider).valueOrNull ?? const <CharacterDto>[];
    CharacterDto? selectedCharacter = characters
        .where((item) => item.id == characterId)
        .firstOrNull;
    selectedCharacter ??= characters
        .where((item) => item.isActive == 1)
        .firstOrNull;
    selectedCharacter ??= characters
        .where((item) => item.isDefault)
        .firstOrNull;
    selectedCharacter ??= characters.firstOrNull;
    if (selectedCharacter != null && selectedCharacter.id != characterId) {
      final resolvedId = selectedCharacter.id;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && ref.read(currentCharacterIdProvider) != resolvedId) {
          ref.read(currentCharacterIdProvider.notifier).state = resolvedId;
        }
      });
    }
    final character = selectedCharacter == null
        ? const _CharInfo('暂无角色', '角')
        : _CharInfo.fromCharacter(selectedCharacter);
    final spaceProfile = ref.watch(currentSpaceProfileProvider).valueOrNull;
    final userName = (spaceProfile?.displayName ?? '').trim().isEmpty
        ? '我'
        : spaceProfile!.displayName.trim();
    final userInitial = userName.characters.first;
    final navigationItems = UINavigationRegistry.resolve(
      ref.watch(uiRuntimeProvider).valueOrNull,
    );
    final conversationSidebar = ref.watch(conversationSidebarProvider);
    final routeState = resolveDrawerRouteState(widget.currentRoute);
    final installedExtensions = ref.watch(_installedExtensionViewProvider);

    return Material(
      color: context.surfacePrimary,
      child: SafeArea(
        child: SizedBox(
          width:
              MediaQuery.sizeOf(context).width * 0.82 >
                  context.uiComponents.drawerMaxWidth
              ? context.uiComponents.drawerMaxWidth
              : MediaQuery.sizeOf(context).width * 0.82,
          child: _DrawerMainPanel(
            character: character,
            isDark: isDark,
            onToggleTheme: () {
              ref
                  .read(appearancePreferencesProvider.notifier)
                  .setThemeMode(isDark ? ThemeMode.light : ThemeMode.dark);
            },
            onSearchTap: () => _showGlobalSearch(navigationItems, characters),
            onNavigate: _navigateTo,
            onSettingsTap: () => _navigateTo(AppRoutes.settings),
            onNewChat: () => _createConversation(),
            onSelectConversation: (conversationId) => _navigateTo(
              '${AppRoutes.chat}?conversationId=${Uri.encodeQueryComponent(conversationId)}',
            ),
            onRenameConversation: _renameConversation,
            onToggleConversationPin: _toggleConversationPin,
            onArchiveConversation: _archiveConversation,
            onAddProject: _addProject,
            onCreateProjectConversation: (projectId) =>
                _createConversation(projectId: projectId),
            onRenameProject: _renameProject,
            onChangeProjectRoot: _changeProjectRoot,
            onToggleProjectPin: _toggleProjectPin,
            onOpenProject: _openProject,
            onRemoveProject: _removeProject,
            conversationSidebar: conversationSidebar,
            navigationItems: navigationItems,
            installedExtensions: installedExtensions,
            currentRoute: widget.currentRoute,
            settingsSelected: routeState.settingsSelected,
            userName: userName,
            userInitial: userInitial,
            userAvatar: spaceProfile?.avatar ?? '',
          ),
        ),
      ),
    );
  }
}

class _DrawerMainPanel extends StatelessWidget {
  final _CharInfo character;
  final bool isDark;
  final VoidCallback onToggleTheme;
  final VoidCallback onSearchTap;
  final ValueChanged<String> onNavigate;
  final VoidCallback onSettingsTap;
  final VoidCallback onNewChat;
  final ValueChanged<String> onSelectConversation;
  final ValueChanged<ConversationDto> onRenameConversation;
  final ValueChanged<ConversationDto> onToggleConversationPin;
  final ValueChanged<ConversationDto> onArchiveConversation;
  final VoidCallback onAddProject;
  final ValueChanged<String> onCreateProjectConversation;
  final ValueChanged<ProjectDto> onRenameProject;
  final ValueChanged<ProjectDto> onChangeProjectRoot;
  final ValueChanged<ProjectDto> onToggleProjectPin;
  final ValueChanged<ProjectDto> onOpenProject;
  final ValueChanged<ProjectDto> onRemoveProject;
  final AsyncValue<ConversationSidebarDto> conversationSidebar;
  final List<UINavigationItem> navigationItems;
  final AsyncValue<ExtensionCenterView> installedExtensions;
  final String currentRoute;
  final bool settingsSelected;
  final String userName;
  final String userInitial;
  final String userAvatar;

  const _DrawerMainPanel({
    required this.character,
    required this.isDark,
    required this.onToggleTheme,
    required this.onSearchTap,
    required this.onNavigate,
    required this.onSettingsTap,
    required this.onNewChat,
    required this.onSelectConversation,
    required this.onRenameConversation,
    required this.onToggleConversationPin,
    required this.onArchiveConversation,
    required this.onAddProject,
    required this.onCreateProjectConversation,
    required this.onRenameProject,
    required this.onChangeProjectRoot,
    required this.onToggleProjectPin,
    required this.onOpenProject,
    required this.onRemoveProject,
    required this.conversationSidebar,
    required this.navigationItems,
    required this.installedExtensions,
    required this.currentRoute,
    required this.settingsSelected,
    required this.userName,
    required this.userInitial,
    required this.userAvatar,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Expanded(
          child: ListView(
            padding: EdgeInsets.zero,
            children: [
              _DrawerHeader(
                name: character.name,
                isDark: isDark,
                onToggleTheme: onToggleTheme,
                onSearchTap: onSearchTap,
              ),
              SizedBox(height: AppSpacing.sm),
              _MainMenuItem(
                icon: Icons.add_comment_outlined,
                label: '新对话',
                isSelected: false,
                onTap: onNewChat,
              ),
              ...navigationItems
                  .where((item) => item.panel == UINavigationPanel.main)
                  .map(
                    (item) => _MainMenuItem(
                      icon: item.icon,
                      label: item.label,
                      isSelected: item.matches(currentRoute),
                      onTap: () => onNavigate(item.route),
                    ),
                  ),
              if (navigationItems.any(
                (item) => item.panel == UINavigationPanel.more,
              )) ...[
                const SizedBox(height: 14),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 20),
                  child: Text('更多', style: AppTypography.label(context)),
                ),
                const SizedBox(height: 4),
                ...navigationItems
                    .where((item) => item.panel == UINavigationPanel.more)
                    .map(
                      (item) => _MainMenuItem(
                        icon: item.icon,
                        label: item.label,
                        isSelected: item.matches(currentRoute),
                        onTap: () => onNavigate(item.route),
                      ),
                    ),
              ],
              installedExtensions.when(
                data: (view) => _buildInstalledExtensions(context, view),
                loading: () => const Padding(
                  padding: EdgeInsets.all(16),
                  child: Center(
                    child: SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                  ),
                ),
                error: (_, _) => const SizedBox.shrink(),
              ),
              conversationSidebar.when(
                loading: () => const Padding(
                  padding: EdgeInsets.all(16),
                  child: Center(
                    child: SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                  ),
                ),
                error: (_, _) => const SizedBox.shrink(),
                data: (sidebar) {
                  final pinnedProjects = sidebar.projects
                      .where((project) => project.pinnedAt.isNotEmpty)
                      .toList(growable: false);
                  final regularProjects = sidebar.projects
                      .where((project) => project.pinnedAt.isEmpty)
                      .toList(growable: false);
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (sidebar.pinned.isNotEmpty ||
                          pinnedProjects.isNotEmpty) ...[
                        Padding(
                          padding: const EdgeInsets.fromLTRB(20, 16, 12, 4),
                          child: Text(
                            '置顶',
                            style: AppTypography.label(context),
                          ),
                        ),
                        _ExpandableConversationList(
                          conversations: sidebar.pinned,
                          onOpen: onSelectConversation,
                          onRename: onRenameConversation,
                          onTogglePin: onToggleConversationPin,
                          onArchive: onArchiveConversation,
                          emptyText: '暂无置顶对话',
                        ),
                        ...pinnedProjects.map(
                          (project) => _ProjectTile(
                            project: project,
                            onOpenConversation: onSelectConversation,
                            onRenameConversation: onRenameConversation,
                            onToggleConversationPin: onToggleConversationPin,
                            onArchiveConversation: onArchiveConversation,
                            onCreateProjectConversation:
                                onCreateProjectConversation,
                            onRenameProject: onRenameProject,
                            onChangeProjectRoot: onChangeProjectRoot,
                            onToggleProjectPin: onToggleProjectPin,
                            onOpenProject: onOpenProject,
                            onRemoveProject: onRemoveProject,
                          ),
                        ),
                      ],
                      Padding(
                        padding: const EdgeInsets.fromLTRB(20, 16, 12, 4),
                        child: Text('最近', style: AppTypography.label(context)),
                      ),
                      _ExpandableRecentList(
                        conversations: sidebar.recent,
                        onOpen: onSelectConversation,
                        onRename: onRenameConversation,
                        onTogglePin: onToggleConversationPin,
                        onArchive: onArchiveConversation,
                      ),
                      Padding(
                        padding: const EdgeInsets.fromLTRB(14, 12, 8, 4),
                        child: Row(
                          children: [
                            const SizedBox(width: 6),
                            Text('项目', style: AppTypography.label(context)),
                            const Spacer(),
                            IconButton(
                              tooltip: '添加项目文件夹',
                              onPressed: onAddProject,
                              icon: const Icon(Icons.add, size: 20),
                            ),
                          ],
                        ),
                      ),
                      ...regularProjects.map(
                        (project) => _ProjectTile(
                          project: project,
                          onOpenConversation: onSelectConversation,
                          onRenameConversation: onRenameConversation,
                          onToggleConversationPin: onToggleConversationPin,
                          onArchiveConversation: onArchiveConversation,
                          onCreateProjectConversation:
                              onCreateProjectConversation,
                          onRenameProject: onRenameProject,
                          onChangeProjectRoot: onChangeProjectRoot,
                          onToggleProjectPin: onToggleProjectPin,
                          onOpenProject: onOpenProject,
                          onRemoveProject: onRemoveProject,
                        ),
                      ),
                    ],
                  );
                },
              ),
            ],
          ),
        ),
        _DrawerBottomArea(
          onSettingsTap: onSettingsTap,
          settingsSelected: settingsSelected,
          userName: userName,
          userInitial: userInitial,
          userAvatar: userAvatar,
        ),
      ],
    );
  }

  Widget _buildInstalledExtensions(
    BuildContext context,
    ExtensionCenterView view,
  ) {
    if (view.installed.isEmpty) return const SizedBox.shrink();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 16),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 20),
          child: Text('已安装扩展', style: AppTypography.label(context)),
        ),
        const SizedBox(height: 4),
        ...view.installed.map(
          (extension) => _InstalledExtensionMenuItem(
            name: extension.displayName,
            status: extension.status,
            isEnabled: extension.enabled,
            isSelected: isRouteFamily(
              currentRoute,
              AppRoutes.extensionsPackages,
            ),
            onTap: () => onNavigate(AppRoutes.extensionsPackages),
          ),
        ),
      ],
    );
  }
}

class _ConversationTile extends StatelessWidget {
  final ConversationDto conversation;
  final VoidCallback onOpen;
  final VoidCallback onRename;
  final VoidCallback onTogglePin;
  final VoidCallback onArchive;
  final bool compact;

  const _ConversationTile({
    required this.conversation,
    required this.onOpen,
    required this.onRename,
    required this.onTogglePin,
    required this.onArchive,
    this.compact = false,
  });

  @override
  Widget build(BuildContext context) {
    final title = conversation.title.trim().isEmpty
        ? '新对话'
        : conversation.title;
    return ListTile(
      dense: true,
      minTileHeight: compact ? 34 : 38,
      minVerticalPadding: 0,
      visualDensity: VisualDensity.compact,
      contentPadding: EdgeInsets.only(left: compact ? 12 : 20, right: 4),
      title: Text(
        title,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(
          color: context.textPrimary,
          fontSize: 14.5,
          fontWeight: FontWeight.w400,
        ),
      ),
      onTap: onOpen,
      onLongPress: () => _showActions(context),
    );
  }

  Future<void> _showActions(BuildContext context) async {
    final tileBox = context.findRenderObject();
    final overlayBox = Overlay.of(context).context.findRenderObject();
    if (tileBox is! RenderBox || overlayBox is! RenderBox) return;
    final origin = tileBox.localToGlobal(Offset.zero);
    const menuWidth = 190.0;
    const estimatedMenuHeight = 160.0;
    final anchorTop = (origin.dy - estimatedMenuHeight).clamp(
      8.0,
      overlayBox.size.height - 8,
    );
    final action = await showGeneralDialog<_ConversationAction>(
      context: context,
      barrierDismissible: true,
      barrierLabel: '对话操作',
      barrierColor: Colors.transparent,
      transitionDuration: const Duration(milliseconds: 180),
      pageBuilder: (dialogContext, _, _) => _ConversationActionMenu(
        pinned: conversation.pinnedAt.isNotEmpty,
        onSelected: (action) => Navigator.of(dialogContext).pop(action),
      ),
      transitionBuilder: (context, animation, _, child) {
        final curved = CurvedAnimation(
          parent: animation,
          curve: Curves.easeOutCubic,
          reverseCurve: Curves.easeInCubic,
        );
        return FadeTransition(
          opacity: curved,
          child: Align(
            alignment: Alignment.topLeft,
            child: Padding(
              padding: EdgeInsets.only(
                top: anchorTop,
                left: origin.dx + 12 + menuWidth / 2,
              ),
              child: ScaleTransition(
                alignment: Alignment.bottomRight,
                scale: Tween<double>(begin: 0.9, end: 1).animate(curved),
                child: child,
              ),
            ),
          ),
        );
      },
    );
    switch (action) {
      case _ConversationAction.rename:
        onRename();
        return;
      case _ConversationAction.archive:
        onArchive();
        return;
      case _ConversationAction.pin:
        onTogglePin();
        return;
      case null:
        return;
    }
  }
}

enum _ConversationAction { rename, archive, pin }

class _ConversationActionMenu extends StatelessWidget {
  final bool pinned;
  final ValueChanged<_ConversationAction> onSelected;

  const _ConversationActionMenu({
    required this.pinned,
    required this.onSelected,
  });

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(14),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.12),
            blurRadius: 14,
            spreadRadius: 0,
            offset: Offset.zero,
          ),
        ],
      ),
      child: Material(
        color: context.surfacePrimary,
        borderRadius: BorderRadius.circular(14),
        clipBehavior: Clip.antiAlias,
        child: SizedBox(
          width: 190,
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: 6),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                _item(
                  context,
                  icon: Icons.drive_file_rename_outline,
                  label: '重命名',
                  value: _ConversationAction.rename,
                ),
                _divider(context),
                _item(
                  context,
                  icon: Icons.archive_outlined,
                  label: '归档',
                  value: _ConversationAction.archive,
                ),
                _divider(context),
                _item(
                  context,
                  icon: pinned ? Icons.push_pin : Icons.push_pin_outlined,
                  label: pinned ? '取消置顶' : '置顶',
                  value: _ConversationAction.pin,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _divider(BuildContext context) {
    return Divider(
      height: 1,
      thickness: 0.6,
      indent: 14,
      endIndent: 14,
      color: context.borderSecondary,
    );
  }

  Widget _item(
    BuildContext context, {
    required IconData icon,
    required String label,
    required _ConversationAction value,
  }) {
    return InkWell(
      onTap: () => onSelected(value),
      child: SizedBox(
        height: 46,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 14),
          child: Row(
            children: [
              Icon(icon, size: 18, color: context.textSecondary),
              const SizedBox(width: 12),
              Text(
                label,
                style: TextStyle(color: context.textPrimary, fontSize: 14.5),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ExpandableRecentList extends StatelessWidget {
  final List<ConversationDto> conversations;
  final ValueChanged<String> onOpen;
  final ValueChanged<ConversationDto> onRename;
  final ValueChanged<ConversationDto> onTogglePin;
  final ValueChanged<ConversationDto> onArchive;

  const _ExpandableRecentList({
    required this.conversations,
    required this.onOpen,
    required this.onRename,
    required this.onTogglePin,
    required this.onArchive,
  });

  @override
  Widget build(BuildContext context) {
    return _ExpandableConversationList(
      conversations: conversations,
      onOpen: onOpen,
      onRename: onRename,
      onTogglePin: onTogglePin,
      onArchive: onArchive,
      emptyText: '暂无对话',
    );
  }
}

class _ProjectConversationList extends StatelessWidget {
  final List<ConversationDto> conversations;
  final ValueChanged<String> onOpen;
  final ValueChanged<ConversationDto> onRename;
  final ValueChanged<ConversationDto> onTogglePin;
  final ValueChanged<ConversationDto> onArchive;

  const _ProjectConversationList({
    required this.conversations,
    required this.onOpen,
    required this.onRename,
    required this.onTogglePin,
    required this.onArchive,
  });

  @override
  Widget build(BuildContext context) {
    return _ExpandableConversationList(
      conversations: conversations,
      onOpen: onOpen,
      onRename: onRename,
      onTogglePin: onTogglePin,
      onArchive: onArchive,
      compact: true,
      emptyText: '暂无对话',
    );
  }
}

enum _ProjectAction { newChat, rename, changeRoot, pin, open, remove }

class _ProjectTile extends StatelessWidget {
  final ProjectDto project;
  final ValueChanged<String> onOpenConversation;
  final ValueChanged<ConversationDto> onRenameConversation;
  final ValueChanged<ConversationDto> onToggleConversationPin;
  final ValueChanged<ConversationDto> onArchiveConversation;
  final ValueChanged<String> onCreateProjectConversation;
  final ValueChanged<ProjectDto> onRenameProject;
  final ValueChanged<ProjectDto> onChangeProjectRoot;
  final ValueChanged<ProjectDto> onToggleProjectPin;
  final ValueChanged<ProjectDto> onOpenProject;
  final ValueChanged<ProjectDto> onRemoveProject;

  const _ProjectTile({
    required this.project,
    required this.onOpenConversation,
    required this.onRenameConversation,
    required this.onToggleConversationPin,
    required this.onArchiveConversation,
    required this.onCreateProjectConversation,
    required this.onRenameProject,
    required this.onChangeProjectRoot,
    required this.onToggleProjectPin,
    required this.onOpenProject,
    required this.onRemoveProject,
  });

  @override
  Widget build(BuildContext context) {
    return ExpansionTile(
      tilePadding: const EdgeInsets.symmetric(horizontal: 18),
      childrenPadding: const EdgeInsets.only(left: 18, right: 8),
      leading: Icon(
        project.available ? Icons.folder_outlined : Icons.folder_off_outlined,
      ),
      title: Row(
        children: [
          Expanded(
            child: Text(
              project.name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          PopupMenuButton<_ProjectAction>(
            tooltip: '项目操作',
            onSelected: _handleAction,
            itemBuilder: (context) => [
              PopupMenuItem(
                value: _ProjectAction.newChat,
                enabled: project.available,
                child: const _ProjectMenuItem(
                  icon: Icons.add_comment_outlined,
                  label: '新建对话',
                ),
              ),
              const PopupMenuItem(
                value: _ProjectAction.rename,
                child: _ProjectMenuItem(
                  icon: Icons.drive_file_rename_outline,
                  label: '重命名项目',
                ),
              ),
              const PopupMenuItem(
                value: _ProjectAction.changeRoot,
                child: _ProjectMenuItem(
                  icon: Icons.drive_file_move_outline,
                  label: '更换根目录',
                ),
              ),
              PopupMenuItem(
                value: _ProjectAction.pin,
                child: _ProjectMenuItem(
                  icon: project.pinnedAt.isEmpty
                      ? Icons.push_pin_outlined
                      : Icons.push_pin,
                  label: project.pinnedAt.isEmpty ? '置顶' : '取消置顶',
                ),
              ),
              PopupMenuItem(
                value: _ProjectAction.open,
                enabled: project.available,
                child: const _ProjectMenuItem(
                  icon: Icons.folder_open_outlined,
                  label: '在资源管理器中打开',
                ),
              ),
              PopupMenuItem(
                value: _ProjectAction.remove,
                child: _ProjectMenuItem(
                  icon: Icons.remove_circle_outline,
                  label: '移除项目',
                  isDestructive: true,
                ),
              ),
            ],
          ),
        ],
      ),
      subtitle: project.available
          ? null
          : Text(project.statusReason.isEmpty ? '目录不可用' : project.statusReason),
      children: [
        _ProjectConversationList(
          conversations: project.conversations,
          onOpen: onOpenConversation,
          onRename: onRenameConversation,
          onTogglePin: onToggleConversationPin,
          onArchive: onArchiveConversation,
        ),
      ],
    );
  }

  void _handleAction(_ProjectAction action) {
    switch (action) {
      case _ProjectAction.newChat:
        onCreateProjectConversation(project.id);
        return;
      case _ProjectAction.rename:
        onRenameProject(project);
        return;
      case _ProjectAction.changeRoot:
        onChangeProjectRoot(project);
        return;
      case _ProjectAction.pin:
        onToggleProjectPin(project);
        return;
      case _ProjectAction.open:
        onOpenProject(project);
        return;
      case _ProjectAction.remove:
        onRemoveProject(project);
        return;
    }
  }
}

class _ProjectMenuItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final bool isDestructive;

  const _ProjectMenuItem({
    required this.icon,
    required this.label,
    this.isDestructive = false,
  });

  @override
  Widget build(BuildContext context) {
    final color = isDestructive ? context.error : context.textPrimary;
    return Row(
      children: [
        Icon(icon, size: 20, color: color),
        const SizedBox(width: 12),
        Text(label, style: TextStyle(color: color)),
      ],
    );
  }
}

class _ExpandableConversationList extends StatefulWidget {
  final List<ConversationDto> conversations;
  final ValueChanged<String> onOpen;
  final ValueChanged<ConversationDto> onRename;
  final ValueChanged<ConversationDto> onTogglePin;
  final ValueChanged<ConversationDto> onArchive;
  final bool compact;
  final String emptyText;

  const _ExpandableConversationList({
    required this.conversations,
    required this.onOpen,
    required this.onRename,
    required this.onTogglePin,
    required this.onArchive,
    required this.emptyText,
    this.compact = false,
  });

  @override
  State<_ExpandableConversationList> createState() =>
      _ExpandableConversationListState();
}

class _ExpandableConversationListState
    extends State<_ExpandableConversationList> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    final visible = _expanded
        ? widget.conversations
        : widget.conversations.take(5).toList(growable: false);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ...visible.map(
          (conversation) => _ConversationTile(
            conversation: conversation,
            compact: widget.compact,
            onOpen: () => widget.onOpen(conversation.id),
            onRename: () => widget.onRename(conversation),
            onTogglePin: () => widget.onTogglePin(conversation),
            onArchive: () => widget.onArchive(conversation),
          ),
        ),
        if (widget.conversations.isEmpty)
          Padding(
            padding: EdgeInsets.symmetric(
              horizontal: widget.compact ? 12 : 20,
              vertical: 6,
            ),
            child: Text(widget.emptyText),
          ),
        if (widget.conversations.length > 5)
          Padding(
            padding: EdgeInsets.only(left: widget.compact ? 12 : 20, bottom: 4),
            child: TextButton(
              onPressed: () => setState(() => _expanded = !_expanded),
              child: Text(_expanded ? '收起' : '展开显示'),
            ),
          ),
      ],
    );
  }
}

class _InstalledExtensionMenuItem extends StatelessWidget {
  final String name;
  final String status;
  final bool isEnabled;
  final bool isSelected;
  final VoidCallback onTap;

  const _InstalledExtensionMenuItem({
    required this.name,
    required this.status,
    required this.isEnabled,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 1),
      child: Material(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(13),
        child: InkWell(
          borderRadius: BorderRadius.circular(13),
          onTap: onTap,
          child: Container(
            constraints: const BoxConstraints(minHeight: 44),
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
            decoration: BoxDecoration(
              color: isSelected ? context.accentSoft : Colors.transparent,
              borderRadius: BorderRadius.circular(13),
            ),
            child: Row(
              children: [
                Container(
                  width: 32,
                  height: 32,
                  decoration: BoxDecoration(
                    color: isEnabled
                        ? context.accentSoft
                        : context.surfaceSecondary,
                    borderRadius: BorderRadius.circular(9),
                  ),
                  child: Icon(
                    Icons.extension_outlined,
                    size: 17,
                    color: isEnabled
                        ? context.accentPrimary
                        : context.textTertiary,
                  ),
                ),
                const SizedBox(width: 11),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        name,
                        style: TextStyle(
                          fontSize: 14,
                          color: context.textPrimary,
                          fontWeight: FontWeight.w500,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      Text(
                        status,
                        style: TextStyle(
                          fontSize: 11,
                          color: context.textTertiary,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ),
                ),
                Container(
                  width: 8,
                  height: 8,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: isEnabled ? context.success : context.borderPrimary,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _DrawerBottomArea extends StatelessWidget {
  final VoidCallback onSettingsTap;
  final bool settingsSelected;
  final String userName;
  final String userInitial;
  final String userAvatar;

  const _DrawerBottomArea({
    required this.onSettingsTap,
    required this.settingsSelected,
    required this.userName,
    required this.userInitial,
    required this.userAvatar,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        border: Border(top: BorderSide(color: context.borderPrimary)),
      ),
      padding: const EdgeInsets.only(top: 7, bottom: 8),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8),
        child: Row(
          children: [
            Expanded(
              child: Material(
                color: Colors.transparent,
                borderRadius: BorderRadius.circular(7),
                child: InkWell(
                  borderRadius: BorderRadius.circular(7),
                  onTap: onSettingsTap,
                  child: Container(
                    constraints: const BoxConstraints(minHeight: 48),
                    padding: const EdgeInsets.symmetric(horizontal: 4),
                    child: Row(
                      children: [
                        SizedBox(
                          width: 44,
                          height: 44,
                          child: Center(
                            child: ProfileAvatar(
                              avatar: userAvatar,
                              initial: userInitial,
                              size: 30,
                            ),
                          ),
                        ),
                        const SizedBox(width: 7),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Text(
                                userName,
                                style: TextStyle(
                                  color: context.textPrimary,
                                  fontSize: 12,
                                  fontWeight: FontWeight.w600,
                                ),
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                              ),
                              const SizedBox(height: 1),
                              Text(
                                '个人空间',
                                style: TextStyle(
                                  color: context.textTertiary,
                                  fontSize: 10,
                                ),
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
            const SizedBox(width: 4),
            Tooltip(
              message: '设置',
              child: Semantics(
                button: true,
                label: '设置',
                child: SizedBox(
                  width: 44,
                  height: 44,
                  child: Center(
                    child: Material(
                      color: settingsSelected
                          ? context.accentSoft
                          : Colors.transparent,
                      borderRadius: BorderRadius.circular(12),
                      child: InkWell(
                        borderRadius: BorderRadius.circular(12),
                        onTap: onSettingsTap,
                        child: SizedBox(
                          width: 36,
                          height: 36,
                          child: Icon(
                            Icons.settings_outlined,
                            size: 17,
                            color: settingsSelected
                                ? context.accentPrimary
                                : context.textSecondary,
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _DrawerHeader extends StatelessWidget {
  final String name;
  final bool isDark;
  final VoidCallback onToggleTheme;
  final VoidCallback onSearchTap;

  const _DrawerHeader({
    required this.name,
    required this.isDark,
    required this.onToggleTheme,
    required this.onSearchTap,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 12, 12, 8),
      child: Row(
        children: [
          Expanded(
            child: Text(
              name,
              style: AppTypography.pageTitle(context).copyWith(fontSize: 17),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          IconButton(
            icon: const Icon(Icons.search, size: 22),
            onPressed: onSearchTap,
            visualDensity: VisualDensity.compact,
          ),
          IconButton(
            icon: Icon(
              isDark ? Icons.light_mode_outlined : Icons.dark_mode_outlined,
              size: 22,
            ),
            onPressed: onToggleTheme,
            visualDensity: VisualDensity.compact,
          ),
        ],
      ),
    );
  }
}

class _MainMenuItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final bool isSelected;
  final VoidCallback onTap;

  const _MainMenuItem({
    required this.icon,
    required this.label,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final variant = context.uiComponentVariant('navigationItem');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: EdgeInsets.symmetric(
          horizontal: number('outerPaddingX', 12),
          vertical: number('outerPaddingY', 0),
        ),
        child: AnimatedContainer(
          duration: AppMotion.standard,
          curve: AppMotion.standardCurve,
          constraints: BoxConstraints(minHeight: number('minHeight', 44)),
          padding: EdgeInsets.symmetric(
            horizontal: number('paddingX', 12),
            vertical: number('paddingY', 8),
          ),
          decoration: BoxDecoration(
            color: isSelected ? context.accentSoft : Colors.transparent,
            borderRadius: BorderRadius.circular(
              number('radius', AppRadius.small),
            ),
          ),
          child: Row(
            children: [
              Icon(
                icon,
                size: number('iconSize', 19),
                color: isSelected
                    ? context.accentPrimary
                    : context.textSecondary,
              ),
              const SizedBox(width: 14),
              Text(
                label,
                style: TextStyle(
                  fontSize: 14.5,
                  fontWeight: isSelected ? FontWeight.w500 : FontWeight.w400,
                  color: isSelected
                      ? context.accentPrimary
                      : context.textPrimary,
                ),
              ),
              const Spacer(),
            ],
          ),
        ),
      ),
    );
  }
}

class _DrawerSearchPage {
  final String label;
  final String route;
  final IconData icon;

  const _DrawerSearchPage(this.label, this.route, this.icon);
}

class _DrawerSearchResult {
  final String route;
  final String? characterId;

  const _DrawerSearchResult(this.route, {this.characterId});
}

class _AmitiaDrawerSearchDelegate extends SearchDelegate<_DrawerSearchResult> {
  final List<_DrawerSearchPage> pages;
  final List<CharacterDto> characters;

  _AmitiaDrawerSearchDelegate({required this.pages, required this.characters});

  @override
  String get searchFieldLabel => '搜索页面、角色';

  @override
  List<Widget>? buildActions(BuildContext context) => [
    if (query.isNotEmpty)
      IconButton(icon: const Icon(Icons.clear), onPressed: () => query = ''),
  ];

  @override
  Widget? buildLeading(BuildContext context) => IconButton(
    icon: const Icon(Icons.arrow_back),
    onPressed: () => Navigator.of(context).maybePop(),
  );

  @override
  Widget buildResults(BuildContext context) => _buildList(context);

  @override
  Widget buildSuggestions(BuildContext context) => _buildList(context);

  Widget _buildList(BuildContext context) {
    final keyword = query.trim().toLowerCase();
    final uniquePages = <String, _DrawerSearchPage>{};
    for (final page in pages) {
      uniquePages.putIfAbsent(page.route, () => page);
    }
    final matchedPages = uniquePages.values
        .where(
          (page) =>
              keyword.isEmpty ||
              page.label.toLowerCase().contains(keyword) ||
              page.route.toLowerCase().contains(keyword),
        )
        .toList(growable: false);
    final matchedCharacters = characters
        .where((character) {
          if (keyword.isEmpty) return true;
          return character.name.toLowerCase().contains(keyword) ||
              character.identity.toLowerCase().contains(keyword) ||
              character.description.toLowerCase().contains(keyword);
        })
        .toList(growable: false);

    if (matchedPages.isEmpty && matchedCharacters.isEmpty) {
      return const Center(child: Text('没有匹配结果'));
    }
    return ListView(
      children: [
        if (matchedPages.isNotEmpty) ...[
          const Padding(
            padding: EdgeInsets.fromLTRB(16, 16, 16, 6),
            child: Text(
              '页面',
              style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600),
            ),
          ),
          ...matchedPages.map(
            (page) => ListTile(
              leading: Icon(page.icon),
              title: Text(page.label),
              subtitle: Text(page.route),
              onTap: () => close(context, _DrawerSearchResult(page.route)),
            ),
          ),
        ],
        if (matchedCharacters.isNotEmpty) ...[
          const Padding(
            padding: EdgeInsets.fromLTRB(16, 16, 16, 6),
            child: Text(
              '角色',
              style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600),
            ),
          ),
          ...matchedCharacters.map(
            (character) => ListTile(
              leading: const Icon(Icons.person_outline),
              title: Text(character.name),
              subtitle: Text(
                character.identity.isNotEmpty
                    ? character.identity
                    : character.description,
              ),
              onTap: () => close(
                context,
                _DrawerSearchResult(
                  AppRoutes.character(character.id),
                  characterId: character.id,
                ),
              ),
            ),
          ),
        ],
      ],
    );
  }
}

class AmitiaCharacterCard extends StatelessWidget {
  final String name;
  final String status;
  final String identity;
  final String avatarInitial;
  final String avatarColor;
  final String avatarUrl;
  final String mood;
  final String lastActive;
  final VoidCallback? onTap;

  const AmitiaCharacterCard({
    super.key,
    required this.name,
    required this.status,
    required this.identity,
    required this.avatarInitial,
    required this.avatarColor,
    this.avatarUrl = '',
    required this.mood,
    required this.lastActive,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final color = Color(
      int.parse('FF${avatarColor.replaceAll('#', '')}', radix: 16),
    );
    final normalizedStatus = status.toLowerCase();
    final isOnline =
        normalizedStatus == '在线' ||
        normalizedStatus == 'enabled' ||
        normalizedStatus == 'online';
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Stack(
              children: [
                Container(
                  width: 52,
                  height: 52,
                  clipBehavior: Clip.antiAlias,
                  decoration: BoxDecoration(
                    color: color,
                    shape: BoxShape.circle,
                  ),
                  child: avatarUrl.trim().isNotEmpty
                      ? Image.network(
                          avatarUrl,
                          fit: BoxFit.cover,
                          errorBuilder: (_, __, ___) => _initialAvatar(),
                        )
                      : _initialAvatar(),
                ),
                if (isOnline)
                  Positioned(
                    right: 0,
                    bottom: 0,
                    child: Container(
                      width: 14,
                      height: 14,
                      decoration: BoxDecoration(
                        color: context.success,
                        shape: BoxShape.circle,
                        border: Border.all(
                          color: context.surfacePrimary,
                          width: 2,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(name, style: AppTypography.cardTitle(context)),
                      const SizedBox(width: 8),
                      AmitiaStatusBadge(
                        label: status,
                        type: isOnline ? BadgeType.success : BadgeType.neutral,
                      ),
                    ],
                  ),
                  const SizedBox(height: 3),
                  Text(identity, style: AppTypography.caption(context)),
                  const SizedBox(height: 3),
                  Row(
                    children: [
                      Text('心情：$mood', style: AppTypography.label(context)),
                      const SizedBox(width: 12),
                      Text(lastActive, style: AppTypography.label(context)),
                    ],
                  ),
                ],
              ),
            ),
            Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
          ],
        ),
      ),
    );
  }

  Widget _initialAvatar() {
    return Center(
      child: Text(
        avatarInitial.isEmpty ? '?' : avatarInitial,
        style: const TextStyle(
          color: Colors.white,
          fontSize: 22,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}

class AmitiaExtensionCard extends StatelessWidget {
  final String name;
  final String description;
  final IconData icon;
  final String typeLabel;
  final bool isInstalled;
  final bool isEnabled;
  final bool isRecommended;
  final VoidCallback? onAction;
  final ValueChanged<bool>? onToggle;

  const AmitiaExtensionCard({
    super.key,
    required this.name,
    required this.description,
    required this.icon,
    required this.typeLabel,
    required this.isInstalled,
    required this.isEnabled,
    this.isRecommended = false,
    this.onAction,
    this.onToggle,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(icon, size: 22, color: context.accentPrimary),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(name, style: AppTypography.cardTitle(context)),
                    const SizedBox(width: 8),
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 6,
                        vertical: 2,
                      ),
                      decoration: BoxDecoration(
                        color: context.borderSecondary,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(
                        typeLabel,
                        style: TextStyle(
                          fontSize: 10,
                          color: context.textTertiary,
                        ),
                      ),
                    ),
                    if (isRecommended) ...[
                      const SizedBox(width: 4),
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: context.accentSoft,
                          borderRadius: AppRadius.brTag,
                        ),
                        child: Text(
                          '推荐',
                          style: TextStyle(
                            fontSize: 10,
                            color: context.accentPrimary,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  description,
                  style: AppTypography.caption(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          if (isInstalled)
            SizedBox(
              width: 44,
              child: Switch(
                value: isEnabled,
                onChanged: onToggle,
                materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
              ),
            )
          else
            GestureDetector(
              onTap: onAction,
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 8,
                ),
                decoration: BoxDecoration(
                  color: context.accentPrimary,
                  borderRadius: AppRadius.brTag,
                ),
                child: Text(
                  '安装',
                  style: TextStyle(
                    fontSize: 13,
                    color: Colors.white,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
