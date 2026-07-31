import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
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

class WorldBookPage extends ConsumerStatefulWidget {
  const WorldBookPage({super.key});

  @override
  ConsumerState<WorldBookPage> createState() => _WorldBookPageState();
}

class _WorldBookPageState extends ConsumerState<WorldBookPage> {
  late List<WorldBookEntry> _entries;
  String _selectedCategory = '全部';

  @override
  void initState() {
    super.initState();
    _entries = List.from(MockMemory.worldBookEntries);
  }

  List<String> get _categories {
    final cats = _entries.map((e) => e.category).toSet().toList();
    return ['全部', ...cats];
  }

  List<WorldBookEntry> get _filteredEntries {
    if (_selectedCategory == '全部') return _entries;
    return _entries.where((e) => e.category == _selectedCategory).toList();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '世界书',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
        actions: [
          AmitiaIconButton(
            icon: Icons.add,
            onPressed: () => _showEntryEditor(context, null),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildCategoryTabs(context),
            Expanded(
              child: _filteredEntries.isEmpty
                  ? AmitiaEmptyState(
                      icon: Icons.menu_book_outlined,
                      title: '暂无世界书条目',
                      subtitle: '点击右上角添加新条目',
                      actionText: '新增条目',
                      onAction: () => _showEntryEditor(context, null),
                    )
                  : ListView.separated(
                      padding: const EdgeInsets.all(AppSpacing.pagePadding),
                      itemCount: _filteredEntries.length,
                      separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
                      itemBuilder: (context, index) => _buildEntryCard(context, _filteredEntries[index]),
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
          final isSelected = _selectedCategory == _categories[index];
          return GestureDetector(
            onTap: () => setState(() => _selectedCategory = _categories[index]),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: 8),
              decoration: BoxDecoration(
                color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Center(
                child: Text(_categories[index], style: TextStyle(fontSize: 13, fontWeight: isSelected ? FontWeight.w500 : FontWeight.w400, color: isSelected ? Colors.white : context.textSecondary)),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildEntryCard(BuildContext context, WorldBookEntry entry) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: entry.isEnabled ? context.accentSoft : context.surfaceSecondary,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(Icons.menu_book_outlined, size: 20, color: entry.isEnabled ? context.accentPrimary : context.textTertiary),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Text(entry.keyword, style: AppTypography.cardTitle(context)),
                        const SizedBox(width: AppSpacing.sm),
                        AmitiaStatusBadge(label: 'P${entry.priority}', type: _getPriorityBadge(entry.priority)),
                      ],
                    ),
                    const SizedBox(height: 2),
                    AmitiaStatusBadge(label: entry.category, type: BadgeType.neutral),
                  ],
                ),
              ),
              Switch(
                value: entry.isEnabled,
                onChanged: (v) {
                  setState(() {
                    final idx = _entries.indexWhere((e) => e.id == entry.id);
                    _entries[idx] = WorldBookEntry(
                      id: entry.id, keyword: entry.keyword, content: entry.content,
                      priority: entry.priority, isEnabled: v, category: entry.category,
                    );
                  });
                },
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(AppSpacing.md),
            decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brSmall),
            child: Text(entry.content, style: AppTypography.bodySmall(context).copyWith(height: 1.5)),
          ),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              GestureDetector(
                onTap: () => _showEntryEditor(context, entry),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brTag),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.edit_outlined, size: 14, color: context.accentPrimary),
                      const SizedBox(width: 4),
                      Text('编辑', style: TextStyle(fontSize: 12, color: context.accentPrimary)),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              GestureDetector(
                onTap: () => _showDeleteConfirm(context, entry),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(color: context.error.withValues(alpha: 0.1), borderRadius: AppRadius.brTag),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.delete_outline, size: 14, color: context.error),
                      const SizedBox(width: 4),
                      Text('删除', style: TextStyle(fontSize: 12, color: context.error)),
                    ],
                  ),
                ),
              ),
              const Spacer(),
              Text('优先级 ${entry.priority}/10', style: AppTypography.label(context)),
            ],
          ),
        ],
      ),
    );
  }

  void _showEntryEditor(BuildContext context, WorldBookEntry? existing) {
    final isEdit = existing != null;
    final keywordCtrl = TextEditingController(text: existing?.keyword ?? '');
    final contentCtrl = TextEditingController(text: existing?.content ?? '');
    int priority = existing?.priority ?? 5;
    String category = existing?.category ?? '默认';
    bool isEnabled = existing?.isEnabled ?? true;

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(AppSpacing.xl, AppSpacing.lg, AppSpacing.xl, MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
              const SizedBox(height: AppSpacing.lg),
              Text(isEdit ? '编辑条目' : '新增条目', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.lg),
              Text('关键词', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: keywordCtrl, hintText: '输入触发关键词'),
              const SizedBox(height: AppSpacing.md),
              Text('内容', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: contentCtrl, maxLines: 4, hintText: '输入世界书内容'),
              const SizedBox(height: AppSpacing.md),
              Text('分类', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.sm,
                children: ['默认', '角色设定', '用户设定', '世界设定', '实体'].map((c) {
                  final isSelected = category == c;
                  return GestureDetector(
                    onTap: () => setSheetState(() => category = c),
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                      decoration: BoxDecoration(
                        color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(c, style: TextStyle(fontSize: 13, color: isSelected ? Colors.white : context.textSecondary)),
                    ),
                  );
                }).toList(),
              ),
              const SizedBox(height: AppSpacing.md),
              Text('优先级：$priority/10', style: AppTypography.label(context)),
              Slider(value: priority.toDouble(), min: 1, max: 10, divisions: 9, activeColor: context.accentPrimary, onChanged: (v) => setSheetState(() => priority = v.round())),
              const SizedBox(height: AppSpacing.md),
              AmitiaSwitchTile(
                title: '启用条目',
                subtitle: '启用后该条目将在对话中生效',
                value: isEnabled,
                onChanged: (v) => setSheetState(() => isEnabled = v),
              ),
              const SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () {
                  if (keywordCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  setState(() {
                    if (isEdit) {
                      final idx = _entries.indexWhere((e) => e.id == existing.id);
                      _entries[idx] = WorldBookEntry(
                        id: existing.id, keyword: keywordCtrl.text.trim(), content: contentCtrl.text.trim(),
                        priority: priority, isEnabled: isEnabled, category: category,
                      );
                    } else {
                      _entries.add(WorldBookEntry(
                        id: 'wb${DateTime.now().millisecondsSinceEpoch}', keyword: keywordCtrl.text.trim(),
                        content: contentCtrl.text.trim(), priority: priority, isEnabled: isEnabled, category: category,
                      ));
                    }
                  });
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(isEdit ? '条目已更新' : '条目已创建'), duration: const Duration(seconds: 1)));
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showDeleteConfirm(BuildContext context, WorldBookEntry entry) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除条目', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除关键词「${entry.keyword}」的条目吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() => _entries.removeWhere((e) => e.id == entry.id));
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('条目已删除'), duration: Duration(seconds: 1)));
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  BadgeType _getPriorityBadge(int priority) {
    if (priority >= 9) return BadgeType.error;
    if (priority >= 7) return BadgeType.warning;
    if (priority >= 5) return BadgeType.info;
    return BadgeType.neutral;
  }
}
