import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/services/chat_service.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../widgets/conversation_list_scroll.dart';
import '../widgets/conversation_list_tile.dart';

class ConversationSearchPage extends ConsumerStatefulWidget {
  const ConversationSearchPage({super.key});

  @override
  ConsumerState<ConversationSearchPage> createState() =>
      _ConversationSearchPageState();
}

class _ConversationSearchPageState
    extends ConsumerState<ConversationSearchPage> {
  final _controller = TextEditingController();
  final _scroll = ScrollController();
  ConversationPage? _page;
  Timer? _debounce;
  String _query = '';
  String _error = '';
  bool _loading = false;
  int _request = 0;

  @override
  void dispose() {
    _debounce?.cancel();
    _scroll.dispose();
    _controller.dispose();
    super.dispose();
  }

  void _changeQuery(String value) {
    _debounce?.cancel();
    _request++;
    setState(() {
      _query = value;
      _page = null;
      _error = '';
      _loading = value.trim().isNotEmpty;
    });
    if (_loading) {
      _debounce = Timer(const Duration(milliseconds: 240), () => _load());
    }
  }

  void _clearQuery() {
    _controller.clear();
    _changeQuery('');
  }

  Future<void> _load({bool append = false}) async {
    final previous = append ? _page : null;
    if (append &&
        (_loading ||
            _error.isNotEmpty ||
            previous == null ||
            previous.cursors.isEmpty))
      return;
    final ticket = ++_request;
    final query = _query.trim();
    setState(() {
      _loading = true;
      _error = '';
    });
    try {
      final page = await ref
          .read(chatServiceProvider)
          .conversationPage(keyword: query, cursors: previous?.cursors);
      if (!mounted || ticket != _request) return;
      final rows =
          {
            for (final row in previous?.rows ?? const <ConversationDto>[])
              row.id: row,
            for (final row in page.rows) row.id: row,
          }.values.toList()..sort((a, b) {
            final pinned =
                (b.pinnedAt.isNotEmpty ? 1 : 0) -
                (a.pinnedAt.isNotEmpty ? 1 : 0);
            return pinned == 0 ? b.updatedAt.compareTo(a.updatedAt) : pinned;
          });
      setState(() => _page = (rows: rows, cursors: page.cursors));
    } catch (error, stackTrace) {
      if (!mounted || ticket != _request) return;
      setState(() => _error = '$error\n$stackTrace');
    } finally {
      if (mounted && ticket == _request) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final results = _page?.rows ?? const <ConversationDto>[];
    final hasMore = _page?.cursors.isNotEmpty ?? false;
    final charactersById = <String, CharacterDto>{
      for (final character
          in ref.watch(characterListProvider).valueOrNull ??
              const <CharacterDto>[])
        if (character.id.isNotEmpty) character.id: character,
    };

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        titleWidget: Container(
          height: 40,
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: BorderRadius.circular(20),
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: AmitiaSearchField(
            controller: _controller,
            hintText: '搜索',
            autofocus: true,
            onChanged: _changeQuery,
          ),
        ),
        actions: [
          if (_query.isNotEmpty)
            AmitiaIconButton(
              icon: Icons.close,
              tooltip: '清空',
              onPressed: _clearQuery,
            ),
          const SizedBox(width: 4),
        ],
      ),
      body: _query.trim().isEmpty
          ? Center(
              child: Text(
                '输入对话名称或消息内容',
                style: AppTypography.bodySmall(
                  context,
                ).copyWith(color: context.textTertiary),
              ),
            )
          : _page == null && _loading
          ? const Center(child: CircularProgressIndicator())
          : _page == null && _error.isNotEmpty
          ? AmitiaErrorState(message: _error, onRetry: () => _load())
          : results.isEmpty && !hasMore
          ? const Center(child: Text('没有找到相关对话'))
          : ConversationListScroll(
              controller: _scroll,
              enabled: hasMore && !_loading && _error.isEmpty,
              onEnd: () => _load(append: true),
              loading: _loading,
              error: _error,
              child: ListView.builder(
                controller: _scroll,
                keyboardDismissBehavior:
                    ScrollViewKeyboardDismissBehavior.onDrag,
                padding: EdgeInsets.only(
                  top: AppSpacing.xs,
                  bottom: AppSpacing.lg,
                ),
                itemCount: results.length,
                itemBuilder: (context, index) {
                  final conversation = results[index];
                  return ConversationListTile(
                    key: ValueKey('search-result-${conversation.id}'),
                    conversation: conversation,
                    character: charactersById[conversation.characterId],
                    timeText: formatConversationTime(conversation.updatedAt),
                    showDivider: index < results.length - 1,
                    onTap: () {
                      final uri =
                          '${AppRoutes.chat}?conversationId=${Uri.encodeQueryComponent(conversation.id)}';
                      context.push(uri);
                    },
                  );
                },
              ),
            ),
    );
  }
}
