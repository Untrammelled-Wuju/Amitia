import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class MemoryPage extends ConsumerStatefulWidget {
  const MemoryPage({super.key});

  @override
  ConsumerState<MemoryPage> createState() => _MemoryPageState();
}

class _MemoryPageState extends ConsumerState<MemoryPage> {
  bool _searchVisible = false;
  String _searchQuery = '';
  int _selectedCategory = 0;

  final _searchController = TextEditingController();
  final _categories = const ['全部', '长期记忆', '情景记忆', '关系记忆', '世界设定'];

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  List<Memory> get _filteredMemories {
    var result = MockData.memories.where((m) {
      if (_selectedCategory == 0) return true;
      return m.category == _categories[_selectedCategory];
    }).where((m) {
      if (_searchQuery.isEmpty) return true;
      return m.content.toLowerCase().contains(_searchQuery.toLowerCase());
    }).toList();

    result.sort((a, b) {
      if (a.isPinned && !b.isPinned) return -1;
      if (!a.isPinned && b.isPinned) return 1;
      return b.time.compareTo(a.time);
    });

    return result;
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '记忆',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: _searchVisible ? Icons.close : Icons.search,
            onPressed: () {
              setState(() {
                _searchVisible = !_searchVisible;
                if (!_searchVisible) {
                  _searchController.clear();
                  _searchQuery = '';
                }
              });
            },
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            if (_searchVisible)
              Padding(
                padding: const EdgeInsets.fromLTRB(
                  AppSpacing.pagePadding,
                  AppSpacing.sm,
                  AppSpacing.pagePadding,
                  AppSpacing.xs,
                ),
                child: AmitiaSearchField(
                  hintText: '搜索记忆',
                  controller: _searchController,
                  onChanged: (value) {
                    setState(() {
                      _searchQuery = value;
                    });
                  },
                ),
              ),
            _buildCategoryTabs(context),
            const SizedBox(height: AppSpacing.sm),
            Expanded(
              child: _filteredMemories.isEmpty
                  ? AmitiaEmptyState(
                      icon: Icons.memory,
                      title: '暂无记忆',
                      subtitle: '与角色对话后，记忆会自动生成',
                    )
                  : ListView.separated(
                      padding: const EdgeInsets.symmetric(
                        horizontal: AppSpacing.pagePadding,
                      ),
                      itemCount: _filteredMemories.length,
                      separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
                      itemBuilder: (context, index) {
                        return _buildMemoryCard(context, _filteredMemories[index]);
                      },
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCategoryTabs(BuildContext context) {
    return SizedBox(
      height: 38,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        itemCount: _categories.length,
        separatorBuilder: (_, _) => const SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final isSelected = _selectedCategory == index;
          return GestureDetector(
            onTap: () {
              setState(() {
                _selectedCategory = index;
              });
            },
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: 8),
              decoration: BoxDecoration(
                color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
                border: Border.all(
                  color: isSelected ? context.accentPrimary : Colors.transparent,
                  width: 0.5,
                ),
              ),
              child: Center(
                child: Text(
                  _categories[index],
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: isSelected ? FontWeight.w500 : FontWeight.w400,
                    color: isSelected ? Colors.white : context.textSecondary,
                  ),
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildMemoryCard(BuildContext context, Memory memory) {
    return AmitiaCard(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (memory.isPinned) ...[
                      Icon(Icons.push_pin, size: 14, color: context.accentPrimary),
                      const SizedBox(width: 6),
                    ],
                    Expanded(
                      child: Text(
                        memory.content,
                        style: AppTypography.body(context),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.sm),
                Row(
                  children: [
                    AmitiaStatusBadge(
                      label: memory.importance,
                      type: _importanceToBadgeType(memory.importance),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Flexible(
                      child: Text(
                        memory.source,
                        style: AppTypography.label(context),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Text(
                      _formatTime(memory.time),
                      style: AppTypography.label(context),
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(width: AppSpacing.sm),
          Column(
            children: [
              GestureDetector(
                onTap: () {
                  setState(() {
                    memory.isPinned = !memory.isPinned;
                  });
                },
                child: Padding(
                  padding: const EdgeInsets.all(4),
                  child: Icon(
                    memory.isPinned ? Icons.push_pin : Icons.push_pin_outlined,
                    size: 18,
                    color: memory.isPinned ? context.accentPrimary : context.textTertiary,
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.xs),
              GestureDetector(
                onTap: () {},
                child: Padding(
                  padding: const EdgeInsets.all(4),
                  child: Icon(
                    Icons.more_horiz,
                    size: 18,
                    color: context.textTertiary,
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  BadgeType _importanceToBadgeType(String importance) {
    switch (importance) {
      case '高':
        return BadgeType.error;
      case '较高':
        return BadgeType.warning;
      case '中':
        return BadgeType.info;
      default:
        return BadgeType.neutral;
    }
  }

  String _formatTime(DateTime time) {
    final now = DateTime.now();
    final diff = now.difference(time);
    if (diff.inHours < 1) return '刚刚';
    if (diff.inDays == 0) return '${diff.inHours}小时前';
    if (diff.inDays < 7) return '${diff.inDays}天前';
    return '${time.month}/${time.day}';
  }
}
