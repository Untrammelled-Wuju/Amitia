import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/conversation.dart';
import '../../../../app/app_routes.dart';

class ChatLogsPage extends ConsumerStatefulWidget {
  const ChatLogsPage({super.key});

  @override
  ConsumerState<ChatLogsPage> createState() => _ChatLogsPageState();
}

class _ChatLogsPageState extends ConsumerState<ChatLogsPage> {
  String? _selectedConversationId;
  String _characterFilter = '全部';
  String _channelFilter = '全部';
  List<MessageDto> _messages = [];

  final _characters = ['全部', 'Amitia', '小雨', 'Epsilon', 'Karin'];
  final _channels = ['全部', 'App', '微信', 'QQ'];

  List<ConversationDto> get _filteredConversations {
    final conversations = _conversationList;
    return conversations.where((c) {
      if (_characterFilter != '全部') {
        final charName = _getCharacterName(c.characterId);
        if (charName != _characterFilter) return false;
      }
      if (_channelFilter != '全部' && c.channel != _channelFilter) return false;
      return true;
    }).toList();
  }

  List<ConversationDto> get _conversationList {
    final async = ref.watch(conversationListProvider);
    return async.valueOrNull ?? [];
  }

  ConversationDto? get _selectedConversation {
    final conversations = _filteredConversations;
    return conversations.where((c) => c.id == _selectedConversationId).firstOrNull;
  }

  @override
  Widget build(BuildContext context) {
    final conversationsAsync = ref.watch(conversationListProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '聊天记录',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          AmitiaIconButton(
            icon: Icons.download_outlined,
            tooltip: '导入聊天记录',
            onPressed: () => context.push(AppRoutes.chatImport),
          ),
          AmitiaIconButton(
            icon: Icons.summarize_outlined,
            tooltip: '会话摘要',
            onPressed: () => _showConversationSummary(context),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: conversationsAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => Center(
            child: Padding(
              padding: const EdgeInsets.all(32),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                  const SizedBox(height: 16),
                  Text(
                    '加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                    style: AppTypography.body(context).copyWith(color: context.error),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 16),
                  AmitiaButton(
                    label: '重试',
                    onPressed: () => ref.invalidate(conversationListProvider),
                  ),
                ],
              ),
            ),
          ),
          data: (conversations) {
            if (conversations.isEmpty) {
              return AmitiaEmptyState(
                icon: Icons.chat_bubble_outline,
                title: '暂无聊天记录',
                subtitle: '开始一个新的对话吧',
              );
            }

            if (_selectedConversationId == null && conversations.isNotEmpty) {
              WidgetsBinding.instance.addPostFrameCallback((_) {
                if (mounted) {
                  setState(() => _selectedConversationId = conversations.first.id);
                  _loadMessages(conversations.first.id);
                }
              });
            }

            if (_selectedConversationId != null && _messages.isEmpty) {
              _loadMessages(_selectedConversationId!);
            }

            return Column(
              children: [
                _buildFilters(context),
                Expanded(
                  child: Row(
                    children: [
                      SizedBox(
                        width: 160,
                        child: _buildConversationList(context),
                      ),
                      Container(width: 0.5, color: context.borderPrimary),
                      Expanded(child: _buildMessagePanel(context)),
                    ],
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  void _loadMessages(String conversationId) {
    final chatApi = ref.read(chatServiceProvider);
    chatApi.getMessages(conversationId).then((msgs) {
      if (mounted) {
        setState(() => _messages = msgs);
      }
    });
  }

  Widget _buildFilters(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: [
            _buildFilterChip(context, '角色', _characterFilter, _characters, (v) => setState(() => _characterFilter = v)),
            const SizedBox(width: AppSpacing.sm),
            _buildFilterChip(context, '渠道', _channelFilter, _channels, (v) => setState(() => _channelFilter = v)),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterChip(BuildContext context, String label, String current, List<String> options, ValueChanged<String> onSelected) {
    return GestureDetector(
      onTap: () => showModalBottomSheet(
        context: context,
        builder: (ctx) => Container(
          padding: const EdgeInsets.all(AppSpacing.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('$label筛选', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.md),
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: options.map((o) => GestureDetector(
                  onTap: () { onSelected(o); Navigator.pop(ctx); },
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                    decoration: BoxDecoration(
                      color: o == current ? context.accentPrimary : context.accentSoft,
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Text(o, style: TextStyle(fontSize: 14, color: o == current ? Colors.white : context.accentPrimary)),
                  ),
                )).toList(),
              ),
              const SizedBox(height: AppSpacing.xl),
            ],
          ),
        ),
      ),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brTag, border: Border.all(color: context.borderPrimary, width: 0.5)),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('$label: $current', style: TextStyle(fontSize: 12, color: context.textSecondary)),
            const SizedBox(width: 4),
            Icon(Icons.arrow_drop_down, size: 16, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  Widget _buildConversationList(BuildContext context) {
    final filtered = _filteredConversations;
    return ListView.separated(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.sm),
      itemCount: filtered.length,
      separatorBuilder: (_, _) => const Divider(height: 1),
      itemBuilder: (context, index) {
        final conv = filtered[index];
        final isSelected = conv.id == _selectedConversationId;
        return GestureDetector(
          onTap: () {
            setState(() {
              _selectedConversationId = conv.id;
              _messages = [];
            });
            _loadMessages(conv.id);
          },
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.sm, vertical: AppSpacing.md),
            decoration: BoxDecoration(
              color: isSelected ? context.accentSoft : Colors.transparent,
              borderRadius: AppRadius.brSmall,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(conv.title, style: AppTypography.bodySmall(context).copyWith(fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400), maxLines: 1, overflow: TextOverflow.ellipsis),
                const SizedBox(height: 2),
                Row(
                  children: [
                    AmitiaStatusBadge(label: _getCharacterName(conv.characterId), type: BadgeType.accent, fontSize: 10),
                    const SizedBox(width: 4),
                    AmitiaStatusBadge(label: conv.channel.isEmpty ? 'App' : conv.channel, type: BadgeType.neutral, fontSize: 10),
                  ],
                ),
                const SizedBox(height: 2),
                Text('${conv.messageCount}条 · ${_formatTime(conv.updatedAt)}', style: AppTypography.label(context)),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildMessagePanel(BuildContext context) {
    final conv = _selectedConversation;
    if (conv == null) {
      return AmitiaEmptyState(icon: Icons.chat_bubble_outline, title: '选择会话', subtitle: '请从左侧选择一个会话');
    }
    return Column(
      children: [
        _buildMessageHeader(context, conv),
        Expanded(child: _buildMessageList(context)),
      ],
    );
  }

  Widget _buildMessageHeader(BuildContext context, ConversationDto conv) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: AppSpacing.sm),
      decoration: BoxDecoration(color: context.surfacePrimary, border: Border(bottom: BorderSide(color: context.borderPrimary, width: 0.5))),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(conv.title, style: AppTypography.cardTitle(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                Text('${_getCharacterName(conv.characterId)} · ${conv.channel.isEmpty ? "App" : conv.channel} · ${conv.messageCount}条消息', style: AppTypography.label(context)),
              ],
            ),
          ),
          AmitiaIconButton(icon: Icons.delete_sweep_outlined, size: 18, color: context.error, tooltip: '清空会话', onPressed: () => _showClearConfirm(context, conv)),
        ],
      ),
    );
  }

  Widget _buildMessageList(BuildContext context) {
    if (_messages.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    return ListView.separated(
      padding: const EdgeInsets.all(AppSpacing.sm),
      itemCount: _messages.length,
      separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.xs),
      itemBuilder: (context, index) => _buildMessageItem(context, _messages[index]),
    );
  }

  Widget _buildMessageItem(BuildContext context, MessageDto message) {
    final isUser = message.role == 'user';
    return GestureDetector(
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: AppSpacing.sm),
        decoration: BoxDecoration(
          color: isUser ? context.accentSoft : context.surfacePrimary,
          borderRadius: AppRadius.brSmall,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 24,
                  height: 24,
                  decoration: BoxDecoration(
                    color: isUser ? context.accentPrimary : context.info,
                    shape: BoxShape.circle,
                  ),
                  child: Center(
                    child: Text(isUser ? '我' : 'AI', style: const TextStyle(color: Colors.white, fontSize: 10, fontWeight: FontWeight.w600)),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Text(isUser ? '用户' : '角色', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w500)),
                const Spacer(),
                Text(_formatMsgTime(message.createdAt), style: AppTypography.label(context)),
              ],
            ),
            const SizedBox(height: AppSpacing.xs),
            Text(message.content, style: AppTypography.bodySmall(context)),
          ],
        ),
      ),
    );
  }

  void _showConversationSummary(BuildContext context) {
    final conv = _selectedConversation;
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('会话摘要', style: AppTypography.cardTitle(context)),
        content: SizedBox(
          width: double.maxFinite,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(conv?.title ?? '未选择会话', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(AppSpacing.lg),
                decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brMedium),
                child: Text(
                  '${_getCharacterName(conv?.characterId ?? "")} · ${_messages.length}条消息',
                  style: AppTypography.bodySmall(context).copyWith(height: 1.6),
                ),
              ),
              const SizedBox(height: AppSpacing.md),
              Text('消息数：${conv?.messageCount ?? 0}', style: AppTypography.caption(context)),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
        ],
      ),
    );
  }

  void _showClearConfirm(BuildContext context, ConversationDto conv) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('清空会话', style: AppTypography.cardTitle(context)),
        content: Text('确定要清空「${conv.title}」的所有消息吗？此操作不可撤销。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              final chatApi = ref.read(chatServiceProvider);
              chatApi.deleteConversation(conv.id).then((_) {
                if (mounted) {
                  ref.invalidate(conversationListProvider);
                  setState(() => _selectedConversationId = null);
                  ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('会话已删除'), duration: Duration(seconds: 1)));
                }
              });
            },
            child: Text('清空', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  String _getCharacterName(String id) {
    final characters = ref.read(characterListProvider).valueOrNull ?? [];
    final character = characters.where((c) => c.id == id).firstOrNull;
    return character?.name ?? '未知';
  }

  String _formatTime(String updatedAt) {
    final time = DateTime.tryParse(updatedAt);
    if (time == null) return '';
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final convDate = DateTime(time.year, time.month, time.day);
    if (convDate == today) {
      return '${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
    } else if (convDate == today.subtract(const Duration(days: 1))) {
      return '昨天';
    } else {
      return '${time.month}/${time.day}';
    }
  }

  String _formatMsgTime(String createdAt) {
    final time = DateTime.tryParse(createdAt);
    if (time == null) return '';
    return '${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
  }
}
