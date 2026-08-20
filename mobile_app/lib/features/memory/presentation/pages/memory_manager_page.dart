import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/memory.dart';

class MemoryManagerPage extends ConsumerStatefulWidget {
  const MemoryManagerPage({super.key});

  @override
  ConsumerState<MemoryManagerPage> createState() => _MemoryManagerPageState();
}

class _MemoryManagerPageState extends ConsumerState<MemoryManagerPage> {
  bool _batchMode = false;
  final Set<String> _selected = {};
  String _typeFilter = '全部';
  String _importanceFilter = '全部';
  bool _searchVisible = false;
  final _searchController = TextEditingController();
  String _searchQuery = '';

  final _types = ['全部', '长期记忆', '情景记忆', '关系记忆', '世界设定'];
  final _importances = ['全部', '高', '较高', '中', '低'];

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final memoriesAsync = ref.watch(memoryListProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '记忆总览',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
        actions: [
          AmitiaIconButton(
            icon: _searchVisible ? Icons.close : Icons.search,
            onPressed: () => setState(() {
              _searchVisible = !_searchVisible;
              if (!_searchVisible) { _searchController.clear(); _searchQuery = ''; }
            }),
          ),
          AmitiaIconButton(
            icon: _batchMode ? Icons.check : Icons.checklist,
            onPressed: () => setState(() {
              _batchMode = !_batchMode;
              if (!_batchMode) _selected.clear();
            }),
          ),
          AmitiaIconButton(
            icon: Icons.account_tree_outlined,
            tooltip: '记忆图谱',
            onPressed: () => context.push(AppRoutes.memoryGraph),
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
                if (_searchVisible)
                  Padding(
                    padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xs),
                    child: AmitiaSearchField(
                      hintText: '语义搜索记忆...',
                      controller: _searchController,
                      onChanged: (v) => setState(() => _searchQuery = v),
                    ),
                  ),
                _buildFilters(context),
                if (_batchMode) _buildBatchBar(context),
                Expanded(
                  child: filtered.isEmpty
                      ? AmitiaEmptyState(
                          icon: Icons.memory,
                          title: '暂无记忆',
                          subtitle: '与角色对话后记忆会自动生成',
                          actionText: '新建记忆',
                          onAction: () => _showMemoryEditor(context, null),
                        )
                      : ListView.separated(
                          padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
                          itemCount: filtered.length,
                          separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                          itemBuilder: (context, index) => _buildMemoryCard(context, filtered[index]),
                        ),
                ),
              ],
            );
          },
        ),
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: () => _showMemoryEditor(context, null),
        backgroundColor: context.accentPrimary,
        child: const Icon(Icons.add, color: Colors.white),
      ),
    );
  }

  List<MemoryDto> _filterMemories(List<MemoryDto> memories) {
    return memories.where((m) {
      if (_typeFilter != '全部' && m.type != _typeFilter) return false;
      if (_importanceFilter != '全部') {
        final impStr = _importanceIntToString(m.importance);
        if (impStr != _importanceFilter) return false;
      }
      if (_searchQuery.isNotEmpty && !m.content.toLowerCase().contains(_searchQuery.toLowerCase())) return false;
      return true;
    }).toList();
  }

  String _importanceIntToString(int importance) {
    if (importance >= 80) return '高';
    if (importance >= 60) return '较高';
    if (importance >= 40) return '中';
    return '低';
  }

  int _importanceStringToInt(String importance) {
    switch (importance) {
      case '高': return 100;
      case '较高': return 75;
      case '中': return 50;
      default: return 25;
    }
  }

  Widget _buildFilters(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: [
            _buildFilterChip(context, '类型: $_typeFilter', _types, (v) => setState(() => _typeFilter = v)),
            SizedBox(width: AppSpacing.sm),
            _buildFilterChip(context, '重要度: $_importanceFilter', _importances, (v) => setState(() => _importanceFilter = v)),
            SizedBox(width: AppSpacing.sm),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterChip(BuildContext context, String label, List<String> options, ValueChanged<String> onSelected) {
    return GestureDetector(
      onTap: () => _showFilterMenu(context, label, options, onSelected),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brTag,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(label, style: TextStyle(fontSize: 12, color: context.textSecondary)),
            const SizedBox(width: 4),
            Icon(Icons.arrow_drop_down, size: 16, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showFilterMenu(BuildContext context, String title, List<String> options, ValueChanged<String> onSelected) {
    showModalBottomSheet(
      context: context,
      builder: (ctx) => Container(
        padding: EdgeInsets.all(AppSpacing.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title, style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.sm,
              children: options.map((o) => GestureDetector(
                onTap: () {
                  onSelected(o);
                  Navigator.pop(ctx);
                },
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Text(o, style: TextStyle(fontSize: 14, color: context.accentPrimary)),
                ),
              )).toList(),
            ),
            SizedBox(height: AppSpacing.xl),
          ],
        ),
      ),
    );
  }

  Widget _buildBatchBar(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
      color: context.accentSoft,
      child: Row(
        children: [
          Text('已选 ${_selected.length} 项', style: AppTypography.bodySmall(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w600)),
          const Spacer(),
          if (_selected.isNotEmpty)
            GestureDetector(
              onTap: () => _showBatchDeleteConfirm(context),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                decoration: BoxDecoration(
                  color: context.error,
                  borderRadius: AppRadius.brTag,
                ),
                child: Text('批量删除', style: TextStyle(fontSize: 13, color: Colors.white)),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildMemoryCard(BuildContext context, MemoryDto memory) {
    final isSelected = _selected.contains(memory.id);
    return AmitiaCard(
      border: Border.all(
        color: _batchMode && isSelected ? context.accentPrimary : context.borderPrimary,
        width: _batchMode && isSelected ? 1.5 : 0.5,
      ),
      onTap: () {
        if (_batchMode) {
          setState(() {
            if (isSelected) {
              _selected.remove(memory.id);
            } else {
              _selected.add(memory.id);
            }
          });
        } else {
          _showMemoryEditor(context, memory);
        }
      },
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (_batchMode)
            Padding(
              padding: EdgeInsets.only(top: 2, right: AppSpacing.sm),
              child: Icon(
                isSelected ? Icons.check_circle : Icons.radio_button_unchecked,
                size: 20,
                color: isSelected ? context.accentPrimary : context.textTertiary,
              ),
            ),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(child: Text(memory.content, style: AppTypography.body(context), maxLines: 2, overflow: TextOverflow.ellipsis)),
                  ],
                ),
                SizedBox(height: AppSpacing.sm),
                Row(
                  children: [
                    AmitiaStatusBadge(label: _importanceIntToString(memory.importance), type: _importanceToBadgeType(memory.importance)),
                    SizedBox(width: AppSpacing.sm),
                    AmitiaStatusBadge(label: memory.type, type: BadgeType.neutral),
                    SizedBox(width: AppSpacing.sm),
                    Text(memory.status, style: AppTypography.label(context)),
                    const Spacer(),
                    Text(_formatTimeString(memory.createdAt), style: AppTypography.label(context)),
                  ],
                ),
                if (!_batchMode) ...[
                  SizedBox(height: AppSpacing.sm),
                  Row(
                    children: [
                      GestureDetector(
                        onTap: () => _showMemoryEditor(context, memory),
                        child: _buildMiniButton(context, '编辑', context.accentPrimary),
                      ),
                      SizedBox(width: AppSpacing.sm),
                      GestureDetector(
                        onTap: () => _showDeleteConfirm(context, memory),
                        child: _buildMiniButton(context, '删除', context.error),
                      ),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMiniButton(BuildContext context, String label, Color color) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 3),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: AppRadius.brTag,
      ),
      child: Text(label, style: TextStyle(fontSize: 12, color: color)),
    );
  }

  void _showMemoryEditor(BuildContext context, MemoryDto? existing) {
    final isEdit = existing != null;
    final contentCtrl = TextEditingController(text: existing?.content ?? '');
    String importance = existing != null ? _importanceIntToString(existing.importance) : '中';
    String type = existing?.type ?? '情景记忆';

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
              Text(isEdit ? '编辑记忆' : '新建记忆', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              Text('记忆内容', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: contentCtrl, maxLines: 4, hintText: '输入记忆内容'),
              SizedBox(height: AppSpacing.md),
              Text('重要程度', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.sm,
                children: ['高', '较高', '中', '低'].map((i) {
                  final isSelected = importance == i;
                  return GestureDetector(
                    onTap: () => setSheetState(() => importance = i),
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                      decoration: BoxDecoration(
                        color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(i, style: TextStyle(fontSize: 13, color: isSelected ? Colors.white : context.textSecondary)),
                    ),
                  );
                }).toList(),
              ),
              SizedBox(height: AppSpacing.md),
              Text('分类', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.sm,
                children: ['长期记忆', '情景记忆', '关系记忆', '世界设定'].map((c) {
                  final isSelected = type == c;
                  return GestureDetector(
                    onTap: () => setSheetState(() => type = c),
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
              SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () async {
                  if (contentCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  final svc = ref.read(memoryServiceProvider);
                  final data = {
                    'content': contentCtrl.text.trim(),
                    'type': type,
                    'importance': _importanceStringToInt(importance),
                    'status': 'active',
                  };
                  try {
                    if (isEdit) {
                      await svc.update(existing.id, data);
                    } else {
                      await svc.create(data);
                    }
                    ref.invalidate(memoryListProvider);
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(isEdit ? '记忆已更新' : '记忆已创建'), duration: const Duration(seconds: 1)));
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

  void _showDeleteConfirm(BuildContext context, MemoryDto memory) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除记忆', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除这条记忆吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              try {
                final svc = ref.read(memoryServiceProvider);
                await svc.delete(memory.id);
                ref.invalidate(memoryListProvider);
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('记忆已删除'), duration: Duration(seconds: 1)));
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

  void _showBatchDeleteConfirm(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('批量删除', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除选中的 ${_selected.length} 条记忆吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              try {
                final svc = ref.read(memoryServiceProvider);
                for (final id in _selected) {
                  await svc.delete(id);
                }
                ref.invalidate(memoryListProvider);
                if (mounted) {
                  setState(() { _selected.clear(); _batchMode = false; });
                  ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('批量删除完成'), duration: Duration(seconds: 1)));
                }
              } catch (e) {
                ref.invalidate(memoryListProvider);
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

  BadgeType _importanceToBadgeType(int importance) {
    if (importance >= 80) return BadgeType.error;
    if (importance >= 60) return BadgeType.warning;
    if (importance >= 40) return BadgeType.info;
    return BadgeType.neutral;
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
}
