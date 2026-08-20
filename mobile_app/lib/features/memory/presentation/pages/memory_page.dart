import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/memory.dart';

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

  String _importanceIntToString(int importance) {
    if (importance >= 80) return '高';
    if (importance >= 60) return '较高';
    if (importance >= 40) return '中';
    return '低';
  }

  String _formatTimeString(String timeStr) {
    if (timeStr.isEmpty) return '';
    try {
      final time = DateTime.parse(timeStr);
      final now = DateTime.now();
      final diff = now.difference(time);
      if (diff.inHours < 1) return '刚刚';
      if (diff.inDays == 0) return '${diff.inHours}小时前';
      if (diff.inDays < 7) return '${diff.inDays}天前';
      return '${time.month}/${time.day}';
    } catch (_) {
      return timeStr;
    }
  }

  List<MemoryDto> _filterMemories(List<MemoryDto> memories) {
    var result = memories.where((m) {
      if (_selectedCategory == 0) return true;
      return m.type == _categories[_selectedCategory];
    }).where((m) {
      if (_searchQuery.isEmpty) return true;
      return m.content.toLowerCase().contains(_searchQuery.toLowerCase());
    }).toList();

    result.sort((a, b) => b.createdAt.compareTo(a.createdAt));
    return result;
  }

  @override
  Widget build(BuildContext context) {
    final memoriesAsync = ref.watch(memoryListProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '记忆',
        navigation: AmitiaAppBarNavigation.back,
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
        child: memoriesAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => Center(
            child: Padding(
              padding: const EdgeInsets.all(32),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                  const SizedBox(height: 16),
                  Text('加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                    style: AppTypography.body(context).copyWith(color: context.error),
                    textAlign: TextAlign.center),
                  const SizedBox(height: 16),
                  AmitiaButton(label: '重试', onPressed: () => ref.invalidate(memoryListProvider)),
                ],
              ),
            ),
          ),
          data: (memories) {
            final filtered = _filterMemories(memories);
            return Column(
              children: [
                _buildMemoryTools(context),
                if (_searchVisible)
                  Padding(
                    padding: EdgeInsets.fromLTRB(
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
                SizedBox(height: AppSpacing.sm),
                Expanded(
                  child: filtered.isEmpty
                      ? AmitiaEmptyState(
                          icon: Icons.memory,
                          title: '暂无记忆',
                          subtitle: '与角色对话后，记忆会自动生成',
                        )
                      : ListView.separated(
                          padding: EdgeInsets.symmetric(
                            horizontal: AppSpacing.pagePadding,
                          ),
                          itemCount: filtered.length,
                          separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                          itemBuilder: (context, index) {
                            return _buildMemoryCard(context, filtered[index]);
                          },
                        ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget _buildCategoryTabs(BuildContext context) {
    return SizedBox(
      height: 38,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        itemCount: _categories.length,
        separatorBuilder: (_, _) => SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final isSelected = _selectedCategory == index;
          return GestureDetector(
            onTap: () {
              setState(() {
                _selectedCategory = index;
              });
            },
            child: Container(
              padding: EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: 8),
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

  Widget _buildMemoryCard(BuildContext context, MemoryDto memory) {
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
                    Expanded(
                      child: Text(
                        memory.content,
                        style: AppTypography.body(context),
                      ),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.sm),
                Row(
                  children: [
                    AmitiaStatusBadge(
                      label: _importanceIntToString(memory.importance),
                      type: _importanceToBadgeType(memory.importance),
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Flexible(
                      child: Text(
                        memory.type,
                        style: AppTypography.label(context),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Text(
                      _formatTimeString(memory.createdAt),
                      style: AppTypography.label(context),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  BadgeType _importanceToBadgeType(int importance) {
    if (importance >= 80) return BadgeType.error;
    if (importance >= 60) return BadgeType.warning;
    if (importance >= 40) return BadgeType.info;
    return BadgeType.neutral;
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
        SizedBox(height: AppSpacing.sm),
        const AmitiaSectionHeader(title: '记忆工具'),
        SizedBox(height: AppSpacing.sm),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: GridView.builder(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
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
                  padding: EdgeInsets.symmetric(horizontal: AppSpacing.sm, vertical: AppSpacing.xs),
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
                      SizedBox(width: AppSpacing.sm),
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
        SizedBox(height: AppSpacing.md),
      ],
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
