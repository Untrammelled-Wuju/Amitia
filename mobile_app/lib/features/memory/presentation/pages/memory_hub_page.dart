import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/memory.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_icons.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/hub_entry_card.dart';
import '../owned_memory_format.dart';

final recentMemoryPageProvider = FutureProvider.autoDispose
    .family<MemoryPage, String>((ref, cursor) async {
      return ref
          .read(deviceOwnedMemoryServiceProvider)
          .pageMemories(cursor: cursor, limit: 10);
    });

class MemoryHubPage extends ConsumerStatefulWidget {
  const MemoryHubPage({super.key});

  @override
  ConsumerState<MemoryHubPage> createState() => _MemoryHubPageState();
}

class _MemoryHubPageState extends ConsumerState<MemoryHubPage> {
  static const _entries = <_MemoryEntry>[
    _MemoryEntry(
      title: '知识库',
      icon: AmitiaIconGlyph.folder,
      route: AppRoutes.knowledge,
    ),
    _MemoryEntry(
      title: '记忆总览',
      icon: AmitiaIconGlyph.memoryOverview,
      route: AppRoutes.memory,
    ),
    _MemoryEntry(
      title: '情景记忆',
      icon: AmitiaIconGlyph.episodicMemory,
      route: AppRoutes.memoryEpisodic,
    ),
    _MemoryEntry(
      title: '记忆图谱',
      icon: AmitiaIconGlyph.memoryGraph,
      route: AppRoutes.memoryGraph,
    ),
    _MemoryEntry(
      title: '时间线',
      icon: AmitiaIconGlyph.memoryTimeline,
      route: AppRoutes.memoryTimeline,
    ),
    _MemoryEntry(
      title: '用户画像',
      icon: AmitiaIconGlyph.userProfile,
      route: AppRoutes.memoryProfiles,
    ),
    _MemoryEntry(
      title: '世界书',
      icon: AmitiaIconGlyph.worldBook,
      route: AppRoutes.memoryWorldBook,
    ),
  ];

  final _scrollController = ScrollController();
  final List<MemoryDto> _memories = <MemoryDto>[];
  String _cursor = '';
  bool _loadingInitial = true;
  bool _loadingMore = false;
  bool _hasMore = true;
  String _error = '';

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_handleScroll);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) unawaited(_loadMore(reset: true));
    });
  }

  @override
  void dispose() {
    _scrollController.removeListener(_handleScroll);
    _scrollController.dispose();
    super.dispose();
  }

  void _handleScroll() {
    if (!_scrollController.hasClients ||
        _scrollController.position.extentAfter > 180 ||
        !_hasMore ||
        _loadingInitial ||
        _loadingMore) {
      return;
    }
    unawaited(_loadMore());
  }

  Future<void> _loadMore({bool reset = false}) async {
    if (_loadingMore || _loadingInitial && !reset) return;
    final cursor = reset ? '' : _cursor;
    if (!reset && !_hasMore) return;
    setState(() {
      _error = '';
      if (reset) {
        _loadingInitial = true;
        _loadingMore = false;
        _hasMore = true;
        _cursor = '';
        _memories.clear();
      } else {
        _loadingMore = true;
      }
    });
    try {
      ref.invalidate(recentMemoryPageProvider(cursor));
      final page = await ref.read(recentMemoryPageProvider(cursor).future);
      if (!mounted) return;
      final knownIds = _memories.map((memory) => memory.id).toSet();
      final incoming =
          page.items
              .where((memory) => knownIds.add(memory.id))
              .toList(growable: false)
            ..sort((left, right) => right.updatedAt.compareTo(left.updatedAt));
      setState(() {
        _memories.addAll(incoming);
        _cursor = page.nextCursor;
        _hasMore = page.hasMore;
      });
    } catch (error) {
      if (mounted) {
        setState(() => _error = error.toString());
      }
    } finally {
      if (mounted) {
        setState(() {
          _loadingInitial = false;
          _loadingMore = false;
        });
      }
    }
  }

  String _memoryTitle(MemoryDto memory) {
    final key = ownedMemoryKeyTitle(memory.key);
    if (key.isNotEmpty) return key;
    return switch (memory.memoryType) {
      'fact' => '事实记忆',
      'preference' => '偏好记忆',
      'profile' => '用户画像',
      'event' => '事件记忆',
      'episodic' => '情景记忆',
      _ => '自定义记忆',
    };
  }

  String _memoryDescription(MemoryDto memory) {
    final value = memory.value.trim().replaceAll(RegExp(r'\s+'), ' ');
    final date = DateTime.tryParse(memory.updatedAt);
    final time = date == null
        ? ''
        : '${date.year}年${date.month}月${date.day}日 ${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}';
    if (value.isEmpty) return time.isEmpty ? '暂无内容' : time;
    return time.isEmpty ? value : '$time · $value';
  }

  Widget _buildRecentMemories() {
    if (_loadingInitial && _memories.isEmpty) {
      return const HubSectionPlaceholder(message: '正在加载记忆…');
    }
    if (_error.isNotEmpty && _memories.isEmpty) {
      return Padding(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
        child: Center(
          child: TextButton(
            onPressed: () => _loadMore(reset: true),
            child: const Text('记忆加载失败，点击重试'),
          ),
        ),
      );
    }
    if (_memories.isEmpty) {
      return const HubSectionPlaceholder(message: '暂无最近记忆');
    }
    return Column(
      children: [
        Container(
          clipBehavior: Clip.antiAlias,
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Column(
            children: [
              for (var index = 0; index < _memories.length; index++)
                HubEntryCard(
                  key: ValueKey('recent-memory-${_memories[index].id}'),
                  icon: AmitiaIconGlyph.memory,
                  title: _memoryTitle(_memories[index]),
                  description: _memoryDescription(_memories[index]),
                  onTap: () => context.push(AppRoutes.memory),
                  showDivider: index < _memories.length - 1,
                ),
            ],
          ),
        ),
        if (_loadingMore)
          Padding(
            padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
            child: SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(
                strokeWidth: 1.8,
                color: context.accentPrimary,
              ),
            ),
          )
        else if (_error.isNotEmpty)
          Padding(
            padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
            child: TextButton(
              onPressed: _loadMore,
              child: const Text('加载失败，点击重试'),
            ),
          )
        else if (!_hasMore && _memories.isNotEmpty)
          HubSectionPlaceholder(message: '没有更多记忆'),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      handlesSystemBack: false,
      appBar: AmitiaAppBar(
        title: '记忆',
        backgroundColor: context.surfacePrimary,
        toolbarHeight: AmitiaAppBar.rootPageToolbarHeight,
      ),
      body: ListView(
        controller: _scrollController,
        padding: EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          AppSpacing.md,
          AppSpacing.pagePadding,
          AppSpacing.xl,
        ),
        children: [
          Container(
            clipBehavior: Clip.antiAlias,
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              children: [
                for (var index = 0; index < _entries.length; index++)
                  HubEntryCard(
                    icon: _entries[index].icon,
                    title: _entries[index].title,
                    onTap: () => context.push(_entries[index].route),
                    showDivider: index < _entries.length - 1,
                  ),
              ],
            ),
          ),
          SizedBox(height: AppSpacing.lg),
          Text('最近记忆', style: AppTypography.label(context)),
          SizedBox(height: AppSpacing.xs),
          _buildRecentMemories(),
        ],
      ),
    );
  }
}

class _MemoryEntry {
  const _MemoryEntry({
    required this.title,
    required this.icon,
    required this.route,
  });

  final String title;
  final AmitiaIconGlyph icon;
  final String route;
}
