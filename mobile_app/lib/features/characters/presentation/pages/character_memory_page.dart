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
import '../../../../core/services/providers.dart';
import '../../../../core/models/memory.dart';

class CharacterMemoryPage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterMemoryPage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterMemoryPage> createState() => _CharacterMemoryPageState();
}

class _CharacterMemoryPageState extends ConsumerState<CharacterMemoryPage> {
  bool _searchVisible = false;
  String _searchQuery = '';
  final _searchController = TextEditingController();

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
        title: '角色记忆',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
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
          AmitiaIconButton(
            icon: Icons.add,
            onPressed: () => _showMemoryEditor(context, null),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            if (_searchVisible)
              Padding(
                padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xs),
                child: AmitiaSearchField(
                  hintText: '搜索记忆',
                  controller: _searchController,
                  onChanged: (v) => setState(() => _searchQuery = v),
                ),
              ),
            Expanded(
              child: memoriesAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (err, _) => Center(child: Text('加载失败: $err')),
                data: (allMemories) {
                  final filtered = allMemories.where((m) {
                    if (_searchQuery.isEmpty) return true;
                    return m.content.toLowerCase().contains(_searchQuery.toLowerCase());
                  }).toList();
                  if (filtered.isEmpty) {
                    return AmitiaEmptyState(
                      icon: Icons.memory,
                      title: '暂无记忆',
                      subtitle: '与角色对话后记忆会自动生成',
                      actionText: '新建记忆',
                      onAction: () => _showMemoryEditor(context, null),
                    );
                  }
                  final grouped = <String, List<MemoryDto>>{};
                  for (final m in filtered) {
                    grouped.putIfAbsent(m.type, () => []).add(m);
                  }
                  return ListView(
                    padding: const EdgeInsets.all(AppSpacing.pagePadding),
                    children: grouped.entries.map((entry) {
                      return _buildMemoryGroup(context, entry.key, entry.value);
                    }).toList(),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildMemoryGroup(BuildContext context, String category, List<MemoryDto> memories) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: AppSpacing.sm, top: AppSpacing.sm),
          child: Row(
            children: [
              Container(
                width: 4,
                height: 16,
                decoration: BoxDecoration(
                  color: context.accentPrimary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              Text(category, style: AppTypography.sectionTitle(context)),
              const SizedBox(width: AppSpacing.sm),
              Text('(${memories.length})', style: AppTypography.label(context)),
            ],
          ),
        ),
        ...memories.map((m) => _buildMemoryCard(context, m)),
        const SizedBox(height: AppSpacing.md),
      ],
    );
  }

  Widget _buildMemoryCard(BuildContext context, MemoryDto memory) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(memory.content, style: AppTypography.body(context)),
            const SizedBox(height: AppSpacing.sm),
            Row(
              children: [
                AmitiaStatusBadge(
                  label: '${memory.importance}',
                  type: memory.importance >= 7 ? BadgeType.error : memory.importance >= 4 ? BadgeType.info : BadgeType.neutral,
                ),
                const SizedBox(width: AppSpacing.sm),
                Text(memory.status, style: AppTypography.label(context)),
                const Spacer(),
                Text(_formatTime(memory.createdAt), style: AppTypography.label(context)),
              ],
            ),
            const SizedBox(height: AppSpacing.sm),
            Row(
              children: [
                GestureDetector(
                  onTap: () => _showMemoryEditor(context, memory),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: context.accentSoft,
                      borderRadius: AppRadius.brTag,
                    ),
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
                  onTap: () => _deleteMemory(memory),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: context.error.withValues(alpha: 0.1),
                      borderRadius: AppRadius.brTag,
                    ),
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
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showMemoryEditor(BuildContext context, MemoryDto? existing) {
    final isEdit = existing != null;
    final contentCtrl = TextEditingController(text: existing?.content ?? '');
    String type = existing?.type ?? '情景记忆';
    int importance = existing?.importance ?? 5;
    String status = existing?.status ?? 'active';

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.xl,
            AppSpacing.lg,
            AppSpacing.xl,
            MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.xl,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(
                child: Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: context.borderPrimary,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.lg),
              Text(isEdit ? '编辑记忆' : '新建记忆', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.lg),
              Text('记忆内容', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: contentCtrl, maxLines: 4, hintText: '输入记忆内容'),
              const SizedBox(height: AppSpacing.md),
              Text('重要程度: $importance', style: AppTypography.label(context)),
              Slider(
                value: importance.toDouble(),
                min: 1,
                max: 10,
                divisions: 9,
                activeColor: context.accentPrimary,
                onChanged: (v) => setSheetState(() => importance = v.round()),
              ),
              const SizedBox(height: AppSpacing.md),
              Text('分类', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
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
              const SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () async {
                  if (contentCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  final svc = ref.read(memoryServiceProvider);
                  if (isEdit) {
                    await svc.update(existing.id, {
                      'content': contentCtrl.text.trim(),
                      'type': type,
                      'importance': importance,
                      'status': status,
                    });
                  } else {
                    await svc.create({
                      'content': contentCtrl.text.trim(),
                      'type': type,
                      'importance': importance,
                      'status': status,
                    });
                  }
                  ref.invalidate(memoryListProvider);
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(isEdit ? '记忆已更新' : '记忆已创建'), duration: const Duration(seconds: 1)),
                    );
                  }
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _deleteMemory(MemoryDto memory) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除记忆', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除这条记忆吗？此操作不可撤销。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              final svc = ref.read(memoryServiceProvider);
              await svc.delete(memory.id);
              ref.invalidate(memoryListProvider);
              if (mounted) {
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('记忆已删除'), duration: Duration(seconds: 1)),
                );
              }
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  String _formatTime(String createdAt) {
    final dt = DateTime.tryParse(createdAt);
    if (dt == null) return '';
    final now = DateTime.now();
    final diff = now.difference(dt);
    if (diff.inHours < 1) return '刚刚';
    if (diff.inDays == 0) return '${diff.inHours}小时前';
    if (diff.inDays < 7) return '${diff.inDays}天前';
    return '${dt.month}/${dt.day}';
  }
}
