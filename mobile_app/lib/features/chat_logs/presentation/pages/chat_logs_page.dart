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
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';
import '../../../../app/app_routes.dart';

class ChatLogsPage extends ConsumerStatefulWidget {
  const ChatLogsPage({super.key});

  @override
  ConsumerState<ChatLogsPage> createState() => _ChatLogsPageState();
}

class _ChatLogsPageState extends ConsumerState<ChatLogsPage> {
  late List<ChatLogConversation> _conversations;
  late List<ChatLogMessage> _messages;
  String? _selectedConversationId;
  String _characterFilter = '全部';
  String _channelFilter = '全部';

  final _characters = ['全部', '阿米娅', '小雨', 'Epsilon', 'Karin'];
  final _channels = ['全部', 'App', '微信', 'QQ'];

  @override
  void initState() {
    super.initState();
    _conversations = List.from(MockMemory.chatLogConversations);
    _messages = List.from(MockMemory.chatLogMessages);
    _selectedConversationId = _conversations.first.id;
  }

  List<ChatLogConversation> get _filteredConversations {
    return _conversations.where((c) {
      if (_characterFilter != '全部') {
        final charName = _getCharacterName(c.characterId);
        if (charName != _characterFilter) return false;
      }
      if (_channelFilter != '全部' && c.channel != _channelFilter) return false;
      return true;
    }).toList();
  }

  ChatLogConversation? get _selectedConversation {
    return _conversations.where((c) => c.id == _selectedConversationId).firstOrNull;
  }

  @override
  Widget build(BuildContext context) {
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
        child: Column(
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
        ),
      ),
    );
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
    return ListView.separated(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.sm),
      itemCount: _filteredConversations.length,
      separatorBuilder: (_, _) => const Divider(height: 1),
      itemBuilder: (context, index) {
        final conv = _filteredConversations[index];
        final isSelected = conv.id == _selectedConversationId;
        return GestureDetector(
          onTap: () => setState(() => _selectedConversationId = conv.id),
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
                    AmitiaStatusBadge(label: conv.channel, type: BadgeType.neutral, fontSize: 10),
                  ],
                ),
                const SizedBox(height: 2),
                Text('${conv.messageCount}条 · ${_formatTime(conv.lastTime)}', style: AppTypography.label(context)),
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

  Widget _buildMessageHeader(BuildContext context, ChatLogConversation conv) {
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
                Text('${_getCharacterName(conv.characterId)} · ${conv.channel} · ${conv.messageCount}条消息', style: AppTypography.label(context)),
              ],
            ),
          ),
          AmitiaIconButton(icon: Icons.swap_horiz, size: 18, tooltip: '切换角色', onPressed: () => _showSwitchCharacterConfirm(context, conv)),
          AmitiaIconButton(icon: Icons.delete_sweep_outlined, size: 18, color: context.error, tooltip: '清空会话', onPressed: () => _showClearConfirm(context, conv)),
        ],
      ),
    );
  }

  Widget _buildMessageList(BuildContext context) {
    return ListView.separated(
      padding: const EdgeInsets.all(AppSpacing.sm),
      itemCount: _messages.length,
      separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.xs),
      itemBuilder: (context, index) => _buildMessageItem(context, _messages[index]),
    );
  }

  Widget _buildMessageItem(BuildContext context, ChatLogMessage message) {
    final isUser = message.role == 'user';
    return GestureDetector(
      onTap: () => message.context != null ? _showContextPreview(context, message) : null,
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
                Text(_formatTime(message.time), style: AppTypography.label(context)),
                if (message.context != null) ...[
                  const SizedBox(width: AppSpacing.sm),
                  Icon(Icons.code, size: 12, color: context.accentPrimary),
                ],
                const SizedBox(width: AppSpacing.xs),
                GestureDetector(
                  onTap: () => _showDeleteMessageConfirm(context, message),
                  child: Icon(Icons.close, size: 14, color: context.textTertiary),
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.xs),
            Text(message.content, style: AppTypography.bodySmall(context)),
            if (message.context != null) ...[
              const SizedBox(height: AppSpacing.xs),
              GestureDetector(
                onTap: () => _showContextPreview(context, message),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                  decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brTag),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.code, size: 12, color: context.accentPrimary),
                      const SizedBox(width: 4),
                      Text('上下文: ${message.context}', style: TextStyle(fontSize: 11, color: context.accentPrimary)),
                    ],
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  void _showContextPreview(BuildContext context, ChatLogMessage message) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('上下文预览', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('消息内容', style: AppTypography.label(context)),
            const SizedBox(height: 4),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brSmall),
              child: Text(message.content, style: AppTypography.bodySmall(context)),
            ),
            const SizedBox(height: AppSpacing.md),
            Text('上下文信息', style: AppTypography.label(context)),
            const SizedBox(height: 4),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
              child: Text(message.context ?? '无上下文', style: AppTypography.bodySmall(context).copyWith(color: context.accentPrimary)),
            ),
            const SizedBox(height: AppSpacing.md),
            Text('角色：${message.role}', style: AppTypography.caption(context)),
            Text('时间：${_formatTime(message.time)}', style: AppTypography.caption(context)),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
        ],
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
                  '本次会话主要讨论了文件整理和文档分析。用户请求整理下载目录，AI 扫描了1,247个文件并识别了23个重复文件。随后用户提交了产品需求文档，AI 生成了包含三个模块的摘要。',
                  style: AppTypography.bodySmall(context).copyWith(height: 1.6),
                ),
              ),
              const SizedBox(height: AppSpacing.md),
              Text('消息数：${conv?.messageCount ?? 0}', style: AppTypography.caption(context)),
              Text('最后时间：${conv != null ? _formatTime(conv.lastTime) : '-'}', style: AppTypography.caption(context)),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              _showDeleteSummaryConfirm(context);
            },
            child: Text('删除摘要', style: TextStyle(color: context.error)),
          ),
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
        ],
      ),
    );
  }

  void _showDeleteSummaryConfirm(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除摘要', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除该会话的摘要吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('摘要已删除'), duration: Duration(seconds: 1)));
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  void _showSwitchCharacterConfirm(BuildContext context, ChatLogConversation conv) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('切换会话角色', style: AppTypography.cardTitle(context)),
        content: Text('确定要将此会话的角色从「${_getCharacterName(conv.characterId)}」切换为「小雨」吗？切换后历史消息不受影响。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                final idx = _conversations.indexWhere((c) => c.id == conv.id);
                _conversations[idx] = ChatLogConversation(
                  id: conv.id, title: conv.title, characterId: 'c2',
                  channel: conv.channel, messageCount: conv.messageCount, lastTime: conv.lastTime,
                );
              });
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('已切换角色为「小雨」'), duration: Duration(seconds: 1)));
            },
            child: Text('确认切换', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _showClearConfirm(BuildContext context, ChatLogConversation conv) {
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
              setState(() {
                _messages.clear();
              });
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('会话已清空'), duration: Duration(seconds: 1)));
            },
            child: Text('清空', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  void _showDeleteMessageConfirm(BuildContext context, ChatLogMessage message) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除消息', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除这条消息吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() => _messages.removeWhere((m) => m.id == message.id));
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('消息已删除'), duration: Duration(seconds: 1)));
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  String _getCharacterName(String id) {
    switch (id) {
      case 'c1': return '阿米娅';
      case 'c2': return '小雨';
      case 'c3': return 'Epsilon';
      case 'c4': return 'Karin';
      default: return '未知';
    }
  }

  String _formatTime(DateTime time) {
    return '${time.month}/${time.day} ${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
  }
}
