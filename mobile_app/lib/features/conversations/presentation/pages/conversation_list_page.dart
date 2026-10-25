import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
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
import '../../../chat/presentation/widgets/group_chat_invite_sheet.dart';
import '../../../chat/runtime/conversation_runtime_controller.dart';
import '../widgets/conversation_list_tile.dart';

class ConversationListPage extends ConsumerStatefulWidget {
  const ConversationListPage({super.key});

  @override
  ConsumerState<ConversationListPage> createState() =>
      _ConversationListPageState();
}

class _ConversationListPageState extends ConsumerState<ConversationListPage> {
  void _startNewChat() {
    ref.read(conversationRuntimeControllerProvider).startDraft();
    ref.read(activeConversationIdProvider.notifier).state = '';
    context.push(AppRoutes.chat);
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
    final result = await createGroupChatWithInvites(
      context,
      ref,
      characters: characters,
    );
    if (result == null || !mounted) return;
    await context.push(
      '${AppRoutes.chat}?conversationId=${Uri.encodeQueryComponent(result.group.conversationId)}&characterId=${Uri.encodeQueryComponent(result.baseRoleId)}',
    );
  }

  @override
  Widget build(BuildContext context) {
    final conversationsAsync = ref.watch(conversationListProvider);
    final charactersById = <String, CharacterDto>{
      for (final character
          in ref.watch(characterListProvider).valueOrNull ??
              const <CharacterDto>[])
        if (character.id.isNotEmpty) character.id: character,
    };

    return AmitiaScaffold(
      handlesSystemBack: false,
      appBar: AmitiaAppBar(
        title: '消息',
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
              }
            },
            itemBuilder: (context) => const [
              PopupMenuItem(
                value: _MessageCreateAction.chat,
                child: AmitiaPopupMenuLabel(
                  icon: Icons.add_comment_outlined,
                  label: '新对话',
                ),
              ),
              PopupMenuItem(
                value: _MessageCreateAction.group,
                child: AmitiaPopupMenuLabel(
                  icon: Icons.group_add_outlined,
                  label: '新群聊',
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
            child: conversationsAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (err, _) {
                if (err.toString().contains('没有可用角色')) {
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
                        onAction: () =>
                            context.push(AppRoutes.charactersCreate),
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
                          '加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                          style: AppTypography.body(
                            context,
                          ).copyWith(color: context.error),
                          textAlign: TextAlign.center,
                        ),
                        const SizedBox(height: 16),
                        AmitiaButton(
                          label: '重试',
                          onPressed: () =>
                              ref.invalidate(conversationListProvider),
                        ),
                      ],
                    ),
                  ),
                );
              },
              data: (conversations) {
                final filtered = List<ConversationDto>.of(conversations);
                filtered.sort((a, b) {
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
                });
                if (filtered.isEmpty) {
                  return AmitiaEmptyState(
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
                    return ConversationListTile(
                      key: ValueKey('conversation-${conversation.id}'),
                      conversation: conversation,
                      character: charactersById[conversation.characterId],
                      timeText: formatConversationTime(conversation.updatedAt),
                      showDivider: index < filtered.length - 1,
                      onTap: () {
                        final uri =
                            '${AppRoutes.chat}?conversationId=${Uri.encodeQueryComponent(conversation.id)}';
                        context.push(uri);
                      },
                      onLongPress: () => _showActionsSheet(conversation),
                    );
                  },
                );
              },
            ),
          ),
        ],
      ),
    );
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

enum _MessageCreateAction { chat, group }

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
