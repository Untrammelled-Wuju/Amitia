import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
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
  String _characterFilter = '';
  String _channelFilter = '';
  List<MessageDto> _messages = const [];
  List<MessageDto> _searchResults = const [];
  Map<String, String> _moodsByMessage = const {};
  bool _loadingMessages = false;
  bool _searching = false;
  final TextEditingController _searchController = TextEditingController();

  static const Map<String, String> _channelLabels = {
    '': '全部渠道',
    'web': 'App / Web',
    'wechat': '微信',
    'qq': 'QQ',
  };

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  List<ConversationDto> get _conversationList =>
      ref.watch(conversationListProvider).valueOrNull ?? const [];

  List<ConversationDto> get _filteredConversations {
    return _conversationList.where((c) {
      if (_characterFilter.isNotEmpty && c.characterId != _characterFilter) return false;
      if (_channelFilter.isNotEmpty && c.channel != _channelFilter) return false;
      return true;
    }).toList(growable: false);
  }

  ConversationDto? get _selectedConversation {
    final id = _selectedConversationId;
    if (id == null) return null;
    for (final conversation in _conversationList) {
      if (conversation.id == id) return conversation;
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final conversationsAsync = ref.watch(conversationListProvider);
    final charactersAsync = ref.watch(characterListProvider);

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
          PopupMenuButton<String>(
            tooltip: '导出当前会话',
            enabled: _selectedConversation != null,
            onSelected: _exportConversation,
            itemBuilder: (_) => const [
              PopupMenuItem(value: 'markdown', child: Text('导出 Markdown')),
              PopupMenuItem(value: 'json', child: Text('导出 JSON')),
            ],
            icon: const Icon(Icons.upload_file_outlined),
          ),
          AmitiaIconButton(
            icon: Icons.summarize_outlined,
            tooltip: '会话摘要',
            onPressed: _selectedConversation == null ? null : _showConversationSummary,
          ),
          AmitiaIconButton(
            icon: Icons.delete_forever_outlined,
            tooltip: '删除全部聊天记录',
            color: context.error,
            onPressed: _deleteAll,
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: conversationsAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => _errorState(context, err),
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
                if (!mounted || _selectedConversationId != null) return;
                _selectConversation(conversations.first.id);
              });
            }
            return Column(
              children: [
                _buildFilters(context, charactersAsync.valueOrNull ?? const []),
                Expanded(
                  child: Row(
                    children: [
                      SizedBox(width: 180, child: _buildConversationList(context)),
                      Container(width: 0.5, color: context.borderPrimary),
                      Expanded(child: _buildMessagePanel(context, charactersAsync.valueOrNull ?? const [])),
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

  Widget _errorState(BuildContext context, Object err) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: context.textSecondary),
            const SizedBox(height: 16),
            Text('加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                style: AppTypography.body(context).copyWith(color: context.error), textAlign: TextAlign.center),
            const SizedBox(height: 16),
            AmitiaButton(label: '重试', onPressed: () => ref.invalidate(conversationListProvider)),
          ],
        ),
      ),
    );
  }

  Future<void> _selectConversation(String id) async {
    setState(() {
      _selectedConversationId = id;
      _messages = const [];
      _searchResults = const [];
      _moodsByMessage = const {};
      _searchController.clear();
      _loadingMessages = true;
    });
    try {
      final messages = await ref.read(chatServiceProvider).getMessages(id);
      final moodMap = <String, String>{};
      try {
        final moods = await ref.read(moodServiceProvider).getByConversation(id);
        for (final item in moods) {
          final messageId = item.messageId.toString();
          final label = item.mood.toString().trim();
          if (messageId.isNotEmpty && label.isNotEmpty) moodMap[messageId] = label;
        }
      } catch (_) {
        // Mood is supplemental metadata; a mood endpoint failure must not block chat logs.
      }
      if (mounted && _selectedConversationId == id) {
        setState(() {
          _messages = messages;
          _moodsByMessage = moodMap;
        });
      }
    } catch (e) {
      _showError('加载消息失败：$e');
    } finally {
      if (mounted && _selectedConversationId == id) setState(() => _loadingMessages = false);
    }
  }

  Future<void> _runSearch(String value) async {
    final keyword = value.trim();
    if (keyword.isEmpty) {
      if (mounted) setState(() => _searchResults = const []);
      return;
    }
    setState(() => _searching = true);
    try {
      final results = await ref.read(chatServiceProvider).searchMessages(keyword);
      if (mounted && _searchController.text.trim() == keyword) setState(() => _searchResults = results);
    } catch (e) {
      _showError('搜索失败：$e');
    } finally {
      if (mounted) setState(() => _searching = false);
    }
  }

  Widget _buildFilters(BuildContext context, List<dynamic> characters) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              controller: _searchController,
              decoration: InputDecoration(
                isDense: true,
                prefixIcon: const Icon(Icons.search, size: 18),
                hintText: '全局搜索消息内容',
                suffixIcon: _searching
                    ? const Padding(padding: EdgeInsets.all(12), child: SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2)))
                    : (_searchController.text.isEmpty
                        ? null
                        : IconButton(
                            icon: const Icon(Icons.close, size: 18),
                            onPressed: () {
                              _searchController.clear();
                              setState(() => _searchResults = const []);
                            },
                          )),
              ),
              onSubmitted: _runSearch,
            ),
          ),
          SizedBox(width: AppSpacing.sm),
          _filterMenu(
            context,
            label: _characterFilter.isEmpty ? '全部角色' : _characterName(_characterFilter),
            items: [
              const MapEntry('', '全部角色'),
              ...characters.map((c) => MapEntry(c.id.toString(), c.name.toString())),
            ],
            onSelected: (value) => setState(() => _characterFilter = value),
          ),
          SizedBox(width: AppSpacing.sm),
          _filterMenu(
            context,
            label: _channelLabels[_channelFilter] ?? _channelFilter,
            items: _channelLabels.entries.toList(),
            onSelected: (value) => setState(() => _channelFilter = value),
          ),
        ],
      ),
    );
  }

  Widget _filterMenu(
    BuildContext context, {
    required String label,
    required List<MapEntry<String, String>> items,
    required ValueChanged<String> onSelected,
  }) {
    return PopupMenuButton<String>(
      onSelected: onSelected,
      itemBuilder: (_) => items.map((e) => PopupMenuItem(value: e.key, child: Text(e.value))).toList(),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brTag,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(mainAxisSize: MainAxisSize.min, children: [Text(label, style: AppTypography.label(context)), const Icon(Icons.arrow_drop_down, size: 17)]),
      ),
    );
  }

  Widget _buildConversationList(BuildContext context) {
    final filtered = _filteredConversations;
    if (filtered.isEmpty) return const Center(child: Text('无匹配会话'));
    return ListView.separated(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.sm),
      itemCount: filtered.length,
      separatorBuilder: (_, _) => const Divider(height: 1),
      itemBuilder: (context, index) {
        final conv = filtered[index];
        final isSelected = conv.id == _selectedConversationId;
        return InkWell(
          borderRadius: AppRadius.brSmall,
          onTap: () => _selectConversation(conv.id),
          child: Container(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.sm, vertical: AppSpacing.md),
            decoration: BoxDecoration(color: isSelected ? context.accentSoft : Colors.transparent, borderRadius: AppRadius.brSmall),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(conv.title.isEmpty ? '新对话' : conv.title,
                    style: AppTypography.bodySmall(context).copyWith(fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400),
                    maxLines: 1, overflow: TextOverflow.ellipsis),
                const SizedBox(height: 4),
                Text('${_characterName(conv.characterId)} · ${_channelLabels[conv.channel] ?? conv.channel} ', style: AppTypography.label(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                const SizedBox(height: 2),
                Text('${conv.messageCount}条 · ${_formatTime(conv.updatedAt)}', style: AppTypography.label(context)),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildMessagePanel(BuildContext context, List<dynamic> characters) {
    if (_searchController.text.trim().isNotEmpty) {
      return _buildSearchResults(context);
    }
    final conv = _selectedConversation;
    if (conv == null) return AmitiaEmptyState(icon: Icons.chat_bubble_outline, title: '选择会话', subtitle: '请从左侧选择一个会话');
    return Column(
      children: [
        _buildMessageHeader(context, conv, characters),
        Expanded(
          child: _loadingMessages
              ? const Center(child: CircularProgressIndicator())
              : _messages.isEmpty
                  ? const Center(child: Text('此会话暂无消息'))
                  : ListView.separated(
                      padding: EdgeInsets.all(AppSpacing.sm),
                      itemCount: _messages.length,
                      separatorBuilder: (_, _) => SizedBox(height: AppSpacing.xs),
                      itemBuilder: (_, index) => _buildMessageItem(context, _messages[index]),
                    ),
        ),
      ],
    );
  }

  Widget _buildSearchResults(BuildContext context) {
    if (_searching) return const Center(child: CircularProgressIndicator());
    if (_searchResults.isEmpty) return const Center(child: Text('没有找到匹配消息'));
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: EdgeInsets.all(AppSpacing.md),
          child: Text('全局搜索结果 · ${_searchResults.length} 条', style: AppTypography.cardTitle(context)),
        ),
        Expanded(
          child: ListView.separated(
            padding: EdgeInsets.all(AppSpacing.sm),
            itemCount: _searchResults.length,
            separatorBuilder: (_, _) => SizedBox(height: AppSpacing.xs),
            itemBuilder: (_, index) => _buildMessageItem(context, _searchResults[index], showConversation: true),
          ),
        ),
      ],
    );
  }

  Widget _buildMessageHeader(BuildContext context, ConversationDto conv, List<dynamic> characters) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: AppSpacing.sm),
      decoration: BoxDecoration(color: context.surfacePrimary, border: Border(bottom: BorderSide(color: context.borderPrimary, width: 0.5))),
      child: Row(
        children: [
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text(conv.title.isEmpty ? '新对话' : conv.title, style: AppTypography.cardTitle(context), maxLines: 1, overflow: TextOverflow.ellipsis),
              Text('${_characterName(conv.characterId)} · ${_channelLabels[conv.channel] ?? conv.channel} · ${conv.messageCount}条消息', style: AppTypography.label(context)),
            ]),
          ),
          IconButton(icon: const Icon(Icons.play_arrow_outlined), tooltip: '继续此会话', onPressed: () => _continueConversation(conv)),
          IconButton(icon: const Icon(Icons.data_object), tooltip: 'Agent 上下文预览', onPressed: () => _showContextPreview(conv)),
          PopupMenuButton<String>(
            tooltip: '导出当前会话',
            onSelected: _exportConversation,
            itemBuilder: (_) => const [
              PopupMenuItem(value: 'markdown', child: Text('Markdown')),
              PopupMenuItem(value: 'json', child: Text('JSON')),
            ],
            icon: const Icon(Icons.upload_file_outlined),
          ),
          PopupMenuButton<String>(
            tooltip: '切换角色',
            onSelected: (characterId) => _changeCharacter(conv.id, characterId),
            itemBuilder: (_) => characters.map<PopupMenuEntry<String>>((c) => PopupMenuItem<String>(value: c.id.toString(), child: Text(c.name.toString()))).toList(),
            icon: const Icon(Icons.person_outline),
          ),
          IconButton(icon: const Icon(Icons.summarize_outlined), tooltip: '摘要', onPressed: _showConversationSummary),
          IconButton(icon: const Icon(Icons.delete_sweep_outlined), color: context.error, tooltip: '清空消息', onPressed: () => _clearMessages(conv)),
          IconButton(icon: const Icon(Icons.delete_outline), color: context.error, tooltip: '删除会话', onPressed: () => _deleteConversation(conv)),
        ],
      ),
    );
  }

  Widget _buildMessageItem(BuildContext context, MessageDto message, {bool showConversation = false}) {
    final isUser = message.role == 'user';
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: AppSpacing.sm),
      decoration: BoxDecoration(
        color: isUser ? context.accentSoft : context.surfacePrimary,
        borderRadius: AppRadius.brSmall,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(children: [
            Text(isUser ? '用户' : 'AI', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600)),
            if (!showConversation && (_moodsByMessage[message.id] ?? '').isNotEmpty) ...[
              const SizedBox(width: 8),
              AmitiaStatusBadge(label: '心情 ${_moodsByMessage[message.id]}', type: BadgeType.info),
            ],
            if (showConversation) ...[
              const SizedBox(width: 8),
              Expanded(child: Text('会话 ${message.conversationId}', style: AppTypography.label(context), overflow: TextOverflow.ellipsis)),
            ] else
              const Spacer(),
            Text(_formatMsgTime(message.createdAt), style: AppTypography.label(context)),
            IconButton(
              visualDensity: VisualDensity.compact,
              icon: const Icon(Icons.thumb_up_alt_outlined, size: 17),
              tooltip: '消息反馈',
              onPressed: () => _showMessageFeedback(message),
            ),
            IconButton(
              visualDensity: VisualDensity.compact,
              icon: const Icon(Icons.psychology_outlined, size: 17),
              tooltip: '心理快照',
              onPressed: () => _showMessagePsyche(message),
            ),
            IconButton(
              visualDensity: VisualDensity.compact,
              icon: const Icon(Icons.info_outline, size: 17),
              tooltip: '消息状态',
              onPressed: () => _showMessageStatus(message),
            ),
            IconButton(
              visualDensity: VisualDensity.compact,
              icon: const Icon(Icons.delete_outline, size: 17),
              color: context.error,
              tooltip: '删除消息',
              onPressed: () => _deleteMessage(message),
            ),
          ]),
          Text(message.content, style: AppTypography.bodySmall(context)),
          if (message.status.isNotEmpty) ...[
            const SizedBox(height: 5),
            Text('状态 ${message.status}${message.tokens == null ? '' : ' · ${message.tokens} tokens'}', style: AppTypography.label(context)),
          ],
        ],
      ),
    );
  }

  Future<void> _exportConversation(String format) async {
    final conv = _selectedConversation;
    if (conv == null) return;
    try {
      final url = await ref.read(chatServiceProvider).exportConversation(conv.id, format: format);
      if (url.isNotEmpty) {
        await Clipboard.setData(ClipboardData(text: url));
        _show('导出完成，资源地址已复制：$url');
      } else {
        _show('导出完成');
      }
    } catch (e) {
      _showError('导出失败：$e');
    }
  }

  Future<void> _showContextPreview(ConversationDto conv) async {
    try {
      final data = await ref.read(chatServiceProvider).contextPreview(conv.id);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('Agent 上下文预览'),
          content: SizedBox(width: 640, child: SingleChildScrollView(child: SelectableText(_formatObject(data ?? const {})))),
          actions: [
            TextButton(onPressed: () async { await Clipboard.setData(ClipboardData(text: _formatObject(data ?? const {}))); if (ctx.mounted) ScaffoldMessenger.of(ctx).showSnackBar(const SnackBar(content: Text('已复制'))); }, child: const Text('复制')),
            TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
          ],
        ),
      );
    } catch (e) {
      _showError('上下文预览失败：$e');
    }
  }

  void _continueConversation(ConversationDto conv) {
    final uri = '${AppRoutes.chat}?conversationId=${Uri.encodeQueryComponent(conv.id)}&characterId=${Uri.encodeQueryComponent(conv.characterId)}';
    context.go(uri);
  }

  Future<void> _showMessageFeedback(MessageDto message) async {
    try {
      final rows = await ref.read(chatServiceProvider).messageFeedback(message.id);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('消息反馈'),
          content: SizedBox(
            width: 520,
            child: rows.isEmpty
                ? const Text('该消息暂无反馈')
                : ListView.separated(
                    shrinkWrap: true,
                    itemCount: rows.length,
                    separatorBuilder: (_, _) => const Divider(height: 1),
                    itemBuilder: (_, i) {
                      final row = rows[i];
                      return ListTile(
                        dense: true,
                        title: Text((row['feedbackType'] ?? 'unknown').toString()),
                        subtitle: Text('${row['reason'] ?? ''}${row['createdAt'] == null ? '' : '\n${row['createdAt']}'}'),
                      );
                    },
                  ),
          ),
          actions: [TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭'))],
        ),
      );
    } catch (e) {
      _showError('读取反馈失败：$e');
    }
  }

  Future<void> _showMessagePsyche(MessageDto message) async {
    try {
      final data = await ref.read(chatServiceProvider).messagePsyche(message.id);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('消息时点心理快照'),
          content: SizedBox(width: 560, child: SingleChildScrollView(child: SelectableText(_formatObject(data ?? const {})))),
          actions: [TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭'))],
        ),
      );
    } catch (e) {
      _showError('心理快照读取失败：$e');
    }
  }

  Future<String?> _editSummary(String current) async {
    final controller = TextEditingController(text: current);
    final result = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('编辑会话摘要'),
        content: TextField(controller: controller, maxLines: 8, minLines: 4, decoration: const InputDecoration(hintText: '输入摘要内容')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(ctx, controller.text), child: const Text('保存')),
        ],
      ),
    );
    controller.dispose();
    return result;
  }

  String _formatObject(dynamic value, {int depth = 0}) {
    final indent = List<String>.filled(depth, '  ').join();
    if (value is Map) {
      return value.entries.map((e) => '$indent${e.key}: ${e.value is Map || e.value is List ? '\n${_formatObject(e.value, depth: depth + 1)}' : e.value}').join('\n');
    }
    if (value is List) {
      return value.map((e) => '$indent- ${e is Map || e is List ? '\n${_formatObject(e, depth: depth + 1)}' : e}').join('\n');
    }
    return value?.toString() ?? '';
  }

  Future<void> _showMessageStatus(MessageDto message) async {
    try {
      final status = await ref.read(chatServiceProvider).messageStatus(message.id);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('消息状态'),
          content: SelectableText(
            status == null || status.isEmpty
                ? '后端未返回状态数据'
                : status.entries.map((entry) => '${entry.key}: ${entry.value}').join('\n'),
          ),
          actions: [TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('关闭'))],
        ),
      );
    } catch (e) {
      _showError('读取消息状态失败：$e');
    }
  }

  Future<void> _changeCharacter(String conversationId, String characterId) async {
    try {
      await ref.read(chatServiceProvider).changeCharacter(conversationId, characterId);
      ref.invalidate(conversationListProvider);
      _show('会话角色已切换');
    } catch (e) {
      _showError('切换角色失败：$e');
    }
  }

  Future<void> _deleteMessage(MessageDto message) async {
    try {
      await ref.read(chatServiceProvider).deleteMessage(message.id);
      if (_searchController.text.trim().isNotEmpty) {
        await _runSearch(_searchController.text);
      } else if (_selectedConversationId != null) {
        await _selectConversation(_selectedConversationId!);
      }
      ref.invalidate(conversationListProvider);
      _show('消息已删除');
    } catch (e) {
      _showError('删除消息失败：$e');
    }
  }

  Future<void> _clearMessages(ConversationDto conv) async {
    final ok = await _confirm('清空消息', '确定清空「${conv.title.isEmpty ? '新对话' : conv.title}」的所有消息吗？会话本身会保留。');
    if (!ok) return;
    try {
      await ref.read(chatServiceProvider).deleteMessages(conv.id);
      setState(() => _messages = const []);
      ref.invalidate(conversationListProvider);
      _show('会话消息已清空');
    } catch (e) {
      _showError('清空失败：$e');
    }
  }

  Future<void> _deleteConversation(ConversationDto conv) async {
    final ok = await _confirm('删除会话', '确定删除该会话及其全部消息吗？此操作不可撤销。');
    if (!ok) return;
    try {
      await ref.read(chatServiceProvider).deleteConversation(conv.id);
      setState(() {
        _selectedConversationId = null;
        _messages = const [];
        _moodsByMessage = const {};
      });
      ref.invalidate(conversationListProvider);
      _show('会话已删除');
    } catch (e) {
      _showError('删除会话失败：$e');
    }
  }

  Future<void> _deleteAll() async {
    final ok = await _confirm('删除全部聊天记录', '确定删除当前账号的全部会话和消息吗？此操作不可撤销。');
    if (!ok) return;
    try {
      await ref.read(chatServiceProvider).deleteAllConversations();
      setState(() {
        _selectedConversationId = null;
        _messages = const [];
        _moodsByMessage = const {};
        _searchResults = const [];
      });
      ref.invalidate(conversationListProvider);
      _show('全部聊天记录已删除');
    } catch (e) {
      _showError('删除全部记录失败：$e');
    }
  }

  Future<void> _showConversationSummary() async {
    final conv = _selectedConversation;
    if (conv == null) return;
    Map<String, dynamic>? initial;
    try {
      initial = await ref.read(chatServiceProvider).conversationSummary(conv.id);
    } catch (_) {
      initial = null;
    }
    if (!mounted) return;
    var summary = (initial?['summaryText'] ?? '').toString();
    var busy = false;
    await showDialog<void>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (ctx, setLocal) => AlertDialog(
          title: const Text('会话摘要'),
          content: SizedBox(
            width: 520,
            child: busy
                ? const Center(child: Padding(padding: EdgeInsets.all(24), child: CircularProgressIndicator()))
                : SelectableText(summary.isEmpty ? '尚未生成摘要。' : summary),
          ),
          actions: [
            if (summary.isNotEmpty)
              TextButton(
                onPressed: busy
                    ? null
                    : () async {
                        setLocal(() => busy = true);
                        try {
                          await ref.read(chatServiceProvider).deleteConversationSummary(conv.id);
                          setLocal(() => summary = '');
                        } finally {
                          setLocal(() => busy = false);
                        }
                      },
                child: Text('删除摘要', style: TextStyle(color: context.error)),
              ),
            if (summary.isNotEmpty)
              TextButton(
                onPressed: busy
                    ? null
                    : () async {
                        final edited = await _editSummary(summary);
                        if (edited == null || edited.trim().isEmpty) return;
                        setLocal(() => busy = true);
                        try {
                          final result = await ref.read(chatServiceProvider).updateConversationSummary(conv.id, edited);
                          setLocal(() => summary = (result?['summaryText'] ?? edited).toString());
                        } catch (e) {
                          if (mounted) _showError('保存摘要失败：$e');
                        } finally {
                          setLocal(() => busy = false);
                        }
                      },
                child: const Text('编辑摘要'),
              ),
            TextButton(
              onPressed: busy
                  ? null
                  : () async {
                      setLocal(() => busy = true);
                      try {
                        final result = await ref.read(chatServiceProvider).generateConversationSummary(conv.id);
                        setLocal(() => summary = (result?['summaryText'] ?? '').toString());
                      } catch (e) {
                        if (mounted) _showError('生成摘要失败：$e');
                      } finally {
                        setLocal(() => busy = false);
                      }
                    },
              child: Text(summary.isEmpty ? '生成摘要' : '重新生成'),
            ),
            TextButton(onPressed: busy ? null : () => Navigator.pop(dialogContext), child: const Text('关闭')),
          ],
        ),
      ),
    );
  }

  Future<bool> _confirm(String title, String content) async {
    return await showDialog<bool>(
          context: context,
          builder: (ctx) => AlertDialog(
            title: Text(title),
            content: Text(content),
            actions: [
              TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
              TextButton(onPressed: () => Navigator.pop(ctx, true), child: Text('确认', style: TextStyle(color: context.error))),
            ],
          ),
        ) ??
        false;
  }

  String _characterName(String id) {
    final characters = ref.read(characterListProvider).valueOrNull ?? const [];
    for (final character in characters) {
      if (character.id == id) return character.name;
    }
    return id.isEmpty ? '未绑定角色' : id;
  }

  void _show(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  void _showError(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message), backgroundColor: context.error));
  }

  String _formatTime(String value) {
    final time = DateTime.tryParse(value);
    if (time == null) return '';
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final date = DateTime(time.year, time.month, time.day);
    if (date == today) return '${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
    if (date == today.subtract(const Duration(days: 1))) return '昨天';
    return '${time.month}/${time.day}';
  }

  String _formatMsgTime(String value) {
    final time = DateTime.tryParse(value);
    if (time == null) return '';
    return '${time.month}/${time.day} ${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
  }
}
