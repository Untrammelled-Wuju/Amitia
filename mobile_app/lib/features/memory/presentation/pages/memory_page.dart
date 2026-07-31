import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
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
        navigation: AmitiaAppBarNavigation.drawer,
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
            _buildMemoryTools(context),
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
                onTap: () => _showMemoryOptions(context, memory),
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

  Widget _buildMemoryTools(BuildContext context) {
    final tools = <_ToolEntry>[
      _ToolEntry(title: '记忆管理', subtitle: '查看与管理全部记忆', icon: Icons.folder_special_outlined, route: AppRoutes.memoryManager),
      _ToolEntry(title: '情景记忆', subtitle: '回顾情景与事件', icon: Icons.event_note_outlined, route: AppRoutes.memoryEpisodic),
      _ToolEntry(title: '记忆图谱', subtitle: '可视化记忆关联', icon: Icons.account_tree_outlined, route: AppRoutes.memoryGraph),
      _ToolEntry(title: '记忆时间线', subtitle: '按时间浏览记忆', icon: Icons.timeline, route: AppRoutes.memoryTimeline),
      _ToolEntry(title: '用户画像', subtitle: '用户偏好与特征', icon: Icons.person_outline, route: AppRoutes.memoryProfiles),
      _ToolEntry(title: '世界书', subtitle: '世界观与设定', icon: Icons.menu_book_outlined, route: AppRoutes.memoryWorldBook),
    ];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: AppSpacing.sm),
        const AmitiaSectionHeader(title: '记忆工具'),
        const SizedBox(height: AppSpacing.sm),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: GridView.builder(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 2,
              mainAxisSpacing: AppSpacing.sm,
              crossAxisSpacing: AppSpacing.sm,
              childAspectRatio: 3.0,
            ),
            itemCount: tools.length,
            itemBuilder: (context, index) {
              final t = tools[index];
              return GestureDetector(
                onTap: () => context.push(t.route),
                child: AmitiaCard(
                  padding: const EdgeInsets.symmetric(horizontal: AppSpacing.sm, vertical: AppSpacing.xs),
                  child: Row(
                    children: [
                      Container(
                        width: 34,
                        height: 34,
                        decoration: BoxDecoration(
                          color: context.accentSoft,
                          borderRadius: AppRadius.brSmall,
                        ),
                        child: Icon(t.icon, size: 18, color: context.accentPrimary),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            Text(t.title, style: AppTypography.body(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                            const SizedBox(height: 2),
                            Text(t.subtitle, style: AppTypography.label(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
        const SizedBox(height: AppSpacing.md),
      ],
    );
  }

  void _showMemoryOptions(BuildContext context, Memory memory) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (sheetContext) {
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const SizedBox(height: AppSpacing.sm),
              Container(
                width: 36,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderSecondary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              const SizedBox(height: AppSpacing.md),
              _buildSheetOption(
                sheetContext,
                icon: Icons.visibility_outlined,
                title: '查看详情',
                onTap: () => Navigator.pop(sheetContext),
              ),
              _buildSheetOption(
                sheetContext,
                icon: Icons.edit_outlined,
                title: '编辑',
                onTap: () => Navigator.pop(sheetContext),
              ),
              _buildSheetOption(
                sheetContext,
                icon: Icons.tune_outlined,
                title: '调整重要程度',
                onTap: () => Navigator.pop(sheetContext),
              ),
              _buildSheetOption(
                sheetContext,
                icon: Icons.category_outlined,
                title: '移动分类',
                onTap: () => Navigator.pop(sheetContext),
              ),
              _buildSheetOption(
                sheetContext,
                icon: Icons.source_outlined,
                title: '查看来源',
                onTap: () => Navigator.pop(sheetContext),
              ),
              Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
              _buildSheetOption(
                sheetContext,
                icon: Icons.delete_outline,
                title: '删除',
                isDestructive: true,
                onTap: () {
                  Navigator.pop(sheetContext);
                  _confirmDelete(context, memory);
                },
              ),
              const SizedBox(height: AppSpacing.sm),
            ],
          ),
        );
      },
    );
  }

  Widget _buildSheetOption(
    BuildContext context, {
    required IconData icon,
    required String title,
    bool isDestructive = false,
    required VoidCallback onTap,
  }) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 14),
        child: Row(
          children: [
            Icon(icon, size: 20, color: isDestructive ? context.error : context.textSecondary),
            const SizedBox(width: 12),
            Text(
              title,
              style: AppTypography.body(context).copyWith(
                color: isDestructive ? context.error : null,
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _confirmDelete(BuildContext context, Memory memory) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: const Text('删除记忆'),
          content: const Text('确定要删除这条记忆吗？此操作不可撤销。'),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('取消'),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  MockData.memories.remove(memory);
                });
                Navigator.pop(dialogContext);
              },
              child: Text('删除', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }
}

class _ToolEntry {
  final String title;
  final String subtitle;
  final IconData icon;
  final String route;

  _ToolEntry({
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.route,
  });
}
