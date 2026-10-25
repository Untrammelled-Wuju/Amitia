import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
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
  String _query = '';

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _clearQuery() {
    _controller.clear();
    setState(() => _query = '');
  }

  List<ConversationDto> _results(List<ConversationDto> conversations) {
    final query = _query.trim().toLowerCase();
    if (query.isEmpty) return const <ConversationDto>[];
    final matches = conversations.where((conversation) {
      return conversation.title.toLowerCase().contains(query) ||
          conversation.lastMessage.toLowerCase().contains(query);
    }).toList();
    matches.sort((a, b) {
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
    return matches;
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
            onChanged: (value) => setState(() => _query = value),
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
      body: conversationsAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => AmitiaErrorState(
          message: '搜索对话失败：$error',
          onRetry: () => ref.invalidate(conversationListProvider),
        ),
        data: (conversations) {
          if (_query.trim().isEmpty) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 24),
                child: Text(
                  '输入对话名称或消息摘要',
                  style: AppTypography.bodySmall(
                    context,
                  ).copyWith(color: context.textTertiary),
                ),
              ),
            );
          }
          final results = _results(conversations);
          if (results.isEmpty) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 24),
                child: Text(
                  '没有找到相关对话',
                  style: AppTypography.bodySmall(
                    context,
                  ).copyWith(color: context.textSecondary),
                ),
              ),
            );
          }
          return ListView.builder(
            keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
            padding: EdgeInsets.only(top: AppSpacing.xs, bottom: AppSpacing.lg),
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
          );
        },
      ),
    );
  }
}
