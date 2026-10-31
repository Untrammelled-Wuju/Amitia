import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/models/project.dart';
import '../../../../core/widgets/amitia_icons.dart';
import '../../../../core/widgets/amitia_popup_menu.dart';
import '../../../chat/runtime/conversation_runtime_controller.dart';
import 'conversation_list_tile.dart';
import 'conversation_swipe_tile.dart';
import 'project_dialogs.dart';

class ProjectGroupTile extends ConsumerWidget {
  const ProjectGroupTile({
    super.key,
    required this.project,
    required this.expanded,
    required this.onToggle,
    required this.onOpenConversation,
    required this.onConversationLongPress,
    required this.onConversationPin,
    required this.onConversationArchive,
    required this.onConversationMarkRead,
    required this.onCreateConversation,
    required this.onAction,
    this.busy = false,
  });

  final ProjectDto project;
  final bool expanded;
  final bool busy;
  final VoidCallback onToggle;
  final ValueChanged<ConversationDto> onOpenConversation;
  final ValueChanged<ConversationDto> onConversationLongPress;
  final Future<void> Function(ConversationDto) onConversationPin;
  final Future<void> Function(ConversationDto) onConversationArchive;
  final Future<void> Function(ConversationDto) onConversationMarkRead;
  final VoidCallback onCreateConversation;
  final void Function(ProjectDto project, String action) onAction;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final registry = ref.watch(conversationRuntimeRegistryProvider);
    var runningCount = 0;
    var unreadCount = 0;
    for (final conversation in project.conversations) {
      final live = registry.activityFor(conversation.id);
      if (live?.isGenerating == true || conversation.isGenerating) {
        runningCount++;
      }
      final count = [
        conversation.unreadCompletedCount,
        live?.unreadCompletedCount ?? 0,
      ].reduce((left, right) => left > right ? left : right);
      unreadCount += count;
    }
    final primary = project.workspaces
        .where((workspace) => workspace.id == project.primaryWorkspaceId)
        .firstOrNull;
    final projectMeta = [
      primary == null ? '未设置主工作区' : '主工作区：${primary.name}',
      if (runningCount > 0) '$runningCount 个进行中',
      if (unreadCount > 0) '$unreadCount 条未读',
    ].join(' · ');
    final conversations = project.conversations.toList(growable: false)
      ..sort((a, b) {
        final aTime = DateTime.tryParse(a.updatedAt);
        final bTime = DateTime.tryParse(b.updatedAt);
        return (bTime ?? DateTime.fromMillisecondsSinceEpoch(0)).compareTo(
          aTime ?? DateTime.fromMillisecondsSinceEpoch(0),
        );
      });
    final canCreate =
        !project.readOnly &&
        !busy &&
        project.available &&
        project.workspaces.isNotEmpty;

    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 3, 12, 3),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Material(
            color: Colors.transparent,
            child: InkWell(
              borderRadius: BorderRadius.circular(12),
              onTap: onToggle,
              child: Padding(
                padding: const EdgeInsets.fromLTRB(12, 9, 6, 9),
                child: Row(
                  children: [
                    Container(
                      width: 38,
                      height: 38,
                      decoration: BoxDecoration(
                        color: context.accentPrimary.withValues(alpha: 0.10),
                        borderRadius: BorderRadius.circular(11),
                      ),
                      child: Icon(
                        primary == null
                            ? Icons.folder_outlined
                            : projectWorkspaceIcon(primary.kind),
                        size: 21,
                        color: context.accentPrimary,
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            project.name,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTypography.body(context).copyWith(
                              fontSize: 15.5,
                              fontWeight: FontWeight.w500,
                            ),
                          ),
                          const SizedBox(height: 3),
                          Text(
                            projectMeta,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTypography.bodySmall(context).copyWith(
                              color: project.available
                                  ? context.textSecondary
                                  : context.warning,
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: 6),
                    if (runningCount > 0) ...[
                      SizedBox(
                        width: 13,
                        height: 13,
                        child: CircularProgressIndicator(
                          strokeWidth: 1.5,
                          color: context.accentPrimary,
                        ),
                      ),
                      const SizedBox(width: 5),
                    ],
                    if (unreadCount > 0)
                      Container(
                        constraints: const BoxConstraints(
                          minWidth: 18,
                          minHeight: 18,
                        ),
                        padding: const EdgeInsets.symmetric(horizontal: 4),
                        decoration: BoxDecoration(
                          color: context.error,
                          borderRadius: BorderRadius.circular(9),
                        ),
                        alignment: Alignment.center,
                        child: Text(
                          unreadCount > 99 ? '99+' : '$unreadCount',
                          style: AppTypography.label(context).copyWith(
                            color: Colors.white,
                            fontSize: 9,
                            height: 1,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                    AnimatedRotation(
                      turns: expanded ? 0.5 : 0,
                      duration: const Duration(milliseconds: 180),
                      child: const Padding(
                        padding: EdgeInsets.symmetric(horizontal: 7),
                        child: Icon(
                          Icons.keyboard_arrow_down_rounded,
                          size: 18,
                        ),
                      ),
                    ),
                    IconButton(
                      tooltip: '在项目中新建对话',
                      onPressed: canCreate ? onCreateConversation : null,
                      icon: const AmitiaIcon(AmitiaIconGlyph.add, size: 20),
                    ),
                    AmitiaPopupMenuButton<String>(
                      tooltip: '项目操作',
                      menuWidth: 250,
                      itemVerticalPadding: 0,
                      itemFontSize: 14,
                      itemIconSize: 18,
                      itemHorizontalPadding: 14,
                      itemHorizontalMargin: 0,
                      itemMinHeight: 44,
                      onSelected: (action) => onAction(project, action),
                      itemBuilder: (context) => [
                        PopupMenuItem(
                          value: 'newChat',
                          enabled: canCreate,
                          child: const AmitiaPopupMenuLabel(
                            icon: Icons.add_comment_outlined,
                            label: '新建对话',
                          ),
                        ),
                        if (!project.readOnly)
                          const PopupMenuItem(
                            value: 'rename',
                            child: AmitiaPopupMenuLabel(
                              icon: Icons.edit_outlined,
                              label: '重命名项目',
                            ),
                          ),
                        if (!project.readOnly)
                          const PopupMenuItem(
                            value: 'addWorkspace',
                            child: AmitiaPopupMenuLabel(
                              icon: Icons.create_new_folder_outlined,
                              label: '添加副工作区',
                            ),
                          ),
                        if (project.workspaces.isNotEmpty)
                          const PopupMenuDivider(),
                        for (final workspace in project.workspaces)
                          PopupMenuItem(
                            value: 'primary:${workspace.id}',
                            enabled:
                                !project.readOnly &&
                                workspace.id != project.primaryWorkspaceId,
                            child: AmitiaPopupMenuLabel(
                              icon: workspace.id == project.primaryWorkspaceId
                                  ? Icons.star_rounded
                                  : Icons.star_border_rounded,
                              label:
                                  '${workspace.id == project.primaryWorkspaceId ? '主工作区' : '设为主工作区'}：${workspace.name}',
                            ),
                          ),
                        for (final workspace in project.workspaces)
                          PopupMenuItem(
                            value: 'removeWorkspace:${workspace.id}',
                            enabled: !project.readOnly,
                            child: AmitiaPopupMenuLabel(
                              icon: Icons.remove_circle_outline,
                              label: '移除工作区：${workspace.name}',
                            ),
                          ),
                        if (!project.readOnly) const PopupMenuDivider(),
                        if (!project.readOnly)
                          PopupMenuItem(
                            value: 'pin',
                            child: AmitiaPopupMenuLabel(
                              icon: Icons.push_pin_outlined,
                              label: project.pinnedAt.isEmpty ? '置顶项目' : '取消置顶',
                            ),
                          ),
                        PopupMenuItem(
                          value: 'browse',
                          enabled: project.workspaces.isNotEmpty,
                          child: const AmitiaPopupMenuLabel(
                            icon: Icons.folder_open_outlined,
                            label: '浏览主工作区',
                          ),
                        ),
                        if (!project.readOnly)
                          const PopupMenuItem(
                            value: 'remove',
                            child: AmitiaPopupMenuLabel(
                              icon: Icons.visibility_off_outlined,
                              label: '从消息列表移除',
                            ),
                          ),
                      ],
                      child: const SizedBox(
                        width: 36,
                        height: 36,
                        child: Icon(Icons.more_horiz_rounded, size: 20),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          ClipRect(
            child: AnimatedSize(
              duration: const Duration(milliseconds: 190),
              curve: Curves.easeOutCubic,
              child: expanded
                  ? Padding(
                      padding: const EdgeInsets.only(left: 12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: conversations.isEmpty
                            ? [
                                Padding(
                                  padding: const EdgeInsets.fromLTRB(
                                    16,
                                    8,
                                    16,
                                    12,
                                  ),
                                  child: Text(
                                    '暂无对话',
                                    style: AppTypography.caption(context),
                                  ),
                                ),
                              ]
                            : [
                                for (
                                  var index = 0;
                                  index < conversations.length;
                                  index++
                                )
                                  ConversationSwipeTile(
                                    pinned: conversations[index].pinnedAt
                                        .trim()
                                        .isNotEmpty,
                                    readable: true,
                                    onPin: () =>
                                        onConversationPin(conversations[index]),
                                    onArchive: () => onConversationArchive(
                                      conversations[index],
                                    ),
                                    onMarkRead: () => onConversationMarkRead(
                                      conversations[index],
                                    ),
                                    child: ConversationListTile(
                                      conversation: conversations[index],
                                      timeText: formatConversationTime(
                                        conversations[index].updatedAt,
                                      ),
                                      showDivider:
                                          index < conversations.length - 1,
                                      onTap: () => onOpenConversation(
                                        conversations[index],
                                      ),
                                      onLongPress: () =>
                                          onConversationLongPress(
                                            conversations[index],
                                          ),
                                    ),
                                  ),
                              ],
                      ),
                    )
                  : const SizedBox.shrink(),
            ),
          ),
          const Divider(height: 1),
        ],
      ),
    );
  }
}
