import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/worldbook.dart';

class WorldBookPage extends ConsumerStatefulWidget {
  const WorldBookPage({super.key});

  @override
  ConsumerState<WorldBookPage> createState() => _WorldBookPageState();
}

class _WorldBookPageState extends ConsumerState<WorldBookPage> {
  String _selectedCategory = '全部';

  List<String> _categories(List<WorldBookDto> entries) {
    final cats = entries.map((e) => e.title.isNotEmpty ? e.title : (e.keywords.isNotEmpty ? e.keywords.first : '')).where((c) => c.isNotEmpty).toSet().toList();
    return ['全部', ...cats];
  }

  List<WorldBookDto> _filteredEntries(List<WorldBookDto> entries, List<String> categories) {
    if (_selectedCategory == '全部') return entries;
    return entries.where((e) {
      final cat = e.title.isNotEmpty ? e.title : (e.keywords.isNotEmpty ? e.keywords.first : '');
      return cat == _selectedCategory;
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    final wbAsync = ref.watch(worldBookListProvider);
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
        child: wbAsync.when(
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
                  AmitiaButton(label: '重试', onPressed: () => ref.invalidate(worldBookListProvider)),
                ],
              ),
            ),
          ),
          data: (entries) {
            final categories = _categories(entries);
            final filtered = _filteredEntries(entries, categories);
            return Column(
              children: [
                _buildCategoryTabs(context, categories),
                Expanded(
                  child: filtered.isEmpty
                      ? AmitiaEmptyState(
                          icon: Icons.menu_book_outlined,
                          title: '暂无世界书条目',
                          subtitle: '点击右上角添加新条目',
                          actionText: '新增条目',
                          onAction: () => _showEntryEditor(context, null),
                        )
                      : ListView.separated(
                          padding: EdgeInsets.all(AppSpacing.pagePadding),
                          itemCount: filtered.length,
                          separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                          itemBuilder: (context, index) => _buildEntryCard(context, filtered[index]),
                        ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget _buildCategoryTabs(BuildContext context, List<String> categories) {
    return SizedBox(
      height: 38,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        itemCount: categories.length,
        separatorBuilder: (_, _) => SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final cat = categories[index];
          final isSelected = _selectedCategory == cat;
          return GestureDetector(
            onTap: () => setState(() => _selectedCategory = cat),
            child: Container(
              padding: EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: 8),
              decoration: BoxDecoration(
                color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Center(
                child: Text(cat, style: TextStyle(fontSize: 13, fontWeight: isSelected ? FontWeight.w500 : FontWeight.w400, color: isSelected ? Colors.white : context.textSecondary)),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildEntryCard(BuildContext context, WorldBookDto entry) {
    final isEnabled = entry.enabled == 1;
    final keywordStr = entry.keywords.isNotEmpty ? entry.keywords.first : entry.title;
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
                  color: isEnabled ? context.accentSoft : context.surfaceSecondary,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(Icons.menu_book_outlined, size: 20, color: isEnabled ? context.accentPrimary : context.textTertiary),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Text(keywordStr, style: AppTypography.cardTitle(context)),
                        SizedBox(width: AppSpacing.sm),
                        AmitiaStatusBadge(label: 'P${entry.priority}', type: _getPriorityBadge(entry.priority)),
                      ],
                    ),
                    const SizedBox(height: 2),
                    AmitiaStatusBadge(label: entry.title.isNotEmpty ? entry.title : '未分类', type: BadgeType.neutral),
                  ],
                ),
              ),
              Switch(
                value: isEnabled,
                onChanged: (v) => _toggleEntry(context, entry, v),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Container(
            width: double.infinity,
            padding: EdgeInsets.all(AppSpacing.md),
            decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brSmall),
            child: Text(entry.content, style: AppTypography.bodySmall(context).copyWith(height: 1.5)),
          ),
          SizedBox(height: AppSpacing.sm),
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
              SizedBox(width: AppSpacing.sm),
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

  Future<void> _toggleEntry(BuildContext context, WorldBookDto entry, bool enabled) async {
    try {
      final svc = ref.read(worldBookServiceProvider);
      await svc.update(entry.id, {
        'id': entry.id,
        'title': entry.title,
        'content': entry.content,
        'keywords': entry.keywords,
        'priority': entry.priority,
        'enabled': enabled ? 1 : 0,
      });
      ref.invalidate(worldBookListProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('更新失败: ${e.toString().replaceFirst('Exception: ', '')}')));
      }
    }
  }

  void _showEntryEditor(BuildContext context, WorldBookDto? existing) {
    final isEdit = existing != null;
    final titleCtrl = TextEditingController(text: existing?.title ?? '');
    final contentCtrl = TextEditingController(text: existing?.content ?? '');
    final keywordCtrl = TextEditingController(text: existing?.keywords.join(', ') ?? '');
    int priority = existing?.priority ?? 5;
    int enabled = existing?.enabled ?? 1;

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
              SizedBox(height: AppSpacing.lg),
              Text(isEdit ? '编辑条目' : '新增条目', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              Text('标题', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: titleCtrl, hintText: '输入标题'),
              SizedBox(height: AppSpacing.md),
              Text('内容', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: contentCtrl, maxLines: 4, hintText: '输入世界书内容'),
              SizedBox(height: AppSpacing.md),
              Text('关键词', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: keywordCtrl, hintText: '输入关键词，用逗号分隔'),
              SizedBox(height: AppSpacing.md),
              Text('优先级：$priority/10', style: AppTypography.label(context)),
              Slider(value: priority.toDouble(), min: 1, max: 10, divisions: 9, activeColor: context.accentPrimary, onChanged: (v) => setSheetState(() => priority = v.round())),
              SizedBox(height: AppSpacing.md),
              AmitiaSwitchTile(
                title: '启用条目',
                subtitle: '启用后该条目将在对话中生效',
                value: enabled == 1,
                onChanged: (v) => setSheetState(() => enabled = v ? 1 : 0),
              ),
              SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () async {
                  if (titleCtrl.text.trim().isEmpty && keywordCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  final svc = ref.read(worldBookServiceProvider);
                  final keywords = keywordCtrl.text.trim().split(',').map((s) => s.trim()).where((s) => s.isNotEmpty).toList();
                  final data = {
                    'title': titleCtrl.text.trim(),
                    'content': contentCtrl.text.trim(),
                    'keywords': keywords,
                    'priority': priority,
                    'enabled': enabled,
                  };
                  try {
                    if (isEdit) {
                      await svc.update(existing.id, data);
                    } else {
                      await svc.create(data);
                    }
                    ref.invalidate(worldBookListProvider);
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(isEdit ? '条目已更新' : '条目已创建'), duration: const Duration(seconds: 1)));
                    }
                  } catch (e) {
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败: ${e.toString().replaceFirst('Exception: ', '')}')));
                    }
                  }
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showDeleteConfirm(BuildContext context, WorldBookDto entry) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除条目', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除关键词「${entry.keywords.isNotEmpty ? entry.keywords.first : entry.title}」的条目吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              try {
                final svc = ref.read(worldBookServiceProvider);
                await svc.delete(entry.id);
                ref.invalidate(worldBookListProvider);
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('条目已删除'), duration: Duration(seconds: 1)));
                }
              } catch (e) {
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('删除失败: ${e.toString().replaceFirst('Exception: ', '')}')));
                }
              }
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
