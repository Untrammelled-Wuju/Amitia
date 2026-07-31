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

class EmotesPage extends ConsumerStatefulWidget {
  const EmotesPage({super.key});

  @override
  ConsumerState<EmotesPage> createState() => _EmotesPageState();
}

class _EmotesPageState extends ConsumerState<EmotesPage> {
  late List<EmoteGroup> _groups;
  late List<EmoteItem> _emotes;
  String _selectedGroup = '全部';
  bool _batchMode = false;
  Set<String> _selected = {};

  @override
  void initState() {
    super.initState();
    _groups = List.from(MockMemory.emoteGroups);
    _emotes = List.from(MockMemory.emotes);
  }

  List<EmoteItem> get _filteredEmotes {
    if (_selectedGroup == '全部') return _emotes;
    return _emotes.where((e) => e.group == _selectedGroup).toList();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '表情包管理',
        navigation: AmitiaAppBarNavigation.drawer,
        actions: [
          AmitiaIconButton(
            icon: _batchMode ? Icons.check : Icons.checklist,
            onPressed: () => setState(() {
              _batchMode = !_batchMode;
              if (!_batchMode) _selected.clear();
            }),
          ),
          AmitiaIconButton(
            icon: Icons.download,
            tooltip: '批量导入',
            onPressed: () => _showImportPreview(context),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildGroupList(context),
            if (_batchMode) _buildBatchBar(context),
            Expanded(
              child: _filteredEmotes.isEmpty
                  ? AmitiaEmptyState(
                      icon: Icons.emoji_emotions_outlined,
                      title: '暂无表情',
                      subtitle: '点击右上角导入表情包',
                    )
                  : ListView.separated(
                      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
                      itemCount: _filteredEmotes.length,
                      separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
                      itemBuilder: (context, index) => _buildEmoteCard(context, _filteredEmotes[index]),
                    ),
            ),
          ],
        ),
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: () => _showGroupEditor(context, null),
        backgroundColor: context.accentPrimary,
        child: const Icon(Icons.add, color: Colors.white),
      ),
    );
  }

  Widget _buildGroupList(BuildContext context) {
    final allGroups = ['全部', ..._groups.map((g) => g.name)];
    return SizedBox(
      height: 50,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
        itemCount: allGroups.length,
        separatorBuilder: (_, _) => const SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final group = allGroups[index];
          final isSelected = _selectedGroup == group;
          final count = group == '全部' ? _emotes.length : _groups.firstWhere((g) => g.name == group, orElse: () => EmoteGroup(id: '', name: '')).count;
          return GestureDetector(
            onTap: () => setState(() => _selectedGroup = group),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
              decoration: BoxDecoration(
                color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Center(
                child: Text('$group ($count)', style: TextStyle(fontSize: 13, fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400, color: isSelected ? Colors.white : context.textSecondary)),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildBatchBar(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
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
                decoration: BoxDecoration(color: context.error, borderRadius: AppRadius.brTag),
                child: Text('批量删除', style: TextStyle(fontSize: 13, color: Colors.white)),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildEmoteCard(BuildContext context, EmoteItem emote) {
    final isSelected = _selected.contains(emote.id);
    return AmitiaCard(
      border: Border.all(
        color: _batchMode && isSelected ? context.accentPrimary : context.borderPrimary,
        width: _batchMode && isSelected ? 1.5 : 0.5,
      ),
      onTap: () {
        if (_batchMode) {
          setState(() {
            if (isSelected) { _selected.remove(emote.id); } else { _selected.add(emote.id); }
          });
        }
      },
      child: Row(
        children: [
          if (_batchMode)
            Padding(
              padding: const EdgeInsets.only(right: AppSpacing.sm),
              child: Icon(isSelected ? Icons.check_circle : Icons.radio_button_unchecked, size: 20, color: isSelected ? context.accentPrimary : context.textTertiary),
            ),
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Center(child: Text(emote.emoji, style: const TextStyle(fontSize: 28))),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(emote.name, style: AppTypography.cardTitle(context)),
                    const SizedBox(width: AppSpacing.sm),
                    if (emote.characterId != null)
                      AmitiaStatusBadge(label: '专属', type: BadgeType.accent),
                  ],
                ),
                const SizedBox(height: 2),
                Text('含义：${emote.meaning}', style: AppTypography.caption(context)),
                const SizedBox(height: 2),
                Row(
                  children: [
                    Text('分组：${emote.group}', style: AppTypography.label(context)),
                    const SizedBox(width: AppSpacing.md),
                    AmitiaStatusBadge(
                      label: emote.isEnabled ? '启用' : '禁用',
                      type: emote.isEnabled ? BadgeType.success : BadgeType.neutral,
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(width: AppSpacing.sm),
          Column(
            children: [
              Text('发送概率', style: AppTypography.label(context)),
              const SizedBox(height: 2),
              Text('${emote.sendProbability}%', style: AppTypography.bodySmall(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w600)),
            ],
          ),
          if (!_batchMode) ...[
            const SizedBox(width: AppSpacing.sm),
            AmitiaIconButton(
              icon: Icons.delete_outline,
              size: 18,
              color: context.error,
              onPressed: () => _showDeleteConfirm(context, emote),
            ),
          ],
        ],
      ),
    );
  }

  void _showGroupEditor(BuildContext context, EmoteGroup? existing) {
    final isEdit = existing != null;
    final nameCtrl = TextEditingController(text: existing?.name ?? '');

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => Padding(
        padding: EdgeInsets.fromLTRB(AppSpacing.xl, AppSpacing.lg, AppSpacing.xl, MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
            const SizedBox(height: AppSpacing.lg),
            Text(isEdit ? '重命名分组' : '新建分组', style: AppTypography.sectionTitle(context)),
            const SizedBox(height: AppSpacing.lg),
            Text('分组名称', style: AppTypography.label(context)),
            const SizedBox(height: AppSpacing.xs),
            AmitiaTextField(controller: nameCtrl, hintText: '输入分组名称'),
            const SizedBox(height: AppSpacing.xl),
            Row(
              children: [
                if (isEdit)
                  Expanded(
                    child: AmitiaButton(
                      label: '删除分组',
                      isDestructive: true,
                      onPressed: () => _showDeleteGroupConfirm(context, existing!),
                    ),
                  ),
                if (isEdit) const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: isEdit ? '保存' : '创建',
                    onPressed: () {
                      if (nameCtrl.text.trim().isEmpty) return;
                      Navigator.pop(ctx);
                      setState(() {
                        if (isEdit) {
                          final idx = _groups.indexWhere((g) => g.id == existing.id);
                          _groups[idx] = EmoteGroup(id: existing.id, name: nameCtrl.text.trim(), count: existing.count);
                        } else {
                          _groups.add(EmoteGroup(id: 'eg${DateTime.now().millisecondsSinceEpoch}', name: nameCtrl.text.trim()));
                        }
                      });
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(isEdit ? '分组已更新' : '分组已创建'), duration: const Duration(seconds: 1)));
                    },
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showDeleteGroupConfirm(BuildContext context, EmoteGroup group) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除分组', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除分组「${group.name}」吗？组内表情将被移至默认分组。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              Navigator.pop(ctx);
              setState(() => _groups.removeWhere((g) => g.id == group.id));
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('分组已删除'), duration: Duration(seconds: 1)));
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  void _showImportPreview(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('批量导入预览', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppSpacing.lg),
              decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brMedium),
              child: Column(
                children: [
                  Icon(Icons.folder_open, size: 40, color: context.accentPrimary),
                  const SizedBox(height: AppSpacing.sm),
                  Text('表情包文件.zip', style: AppTypography.bodySmall(context)),
                  Text('包含 24 个表情', style: AppTypography.label(context)),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            Text('导入后将创建新分组并添加表情。', style: AppTypography.caption(context)),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                final newGroup = EmoteGroup(id: 'eg${DateTime.now().millisecondsSinceEpoch}', name: '导入表情', count: 24);
                _groups.add(newGroup);
              });
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('已导入24个表情'), duration: Duration(seconds: 1)));
            },
            child: Text('确认导入', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _showDeleteConfirm(BuildContext context, EmoteItem emote) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除表情', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「${emote.name}」吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() => _emotes.removeWhere((e) => e.id == emote.id));
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('表情已删除'), duration: Duration(seconds: 1)));
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
        content: Text('确定要删除选中的 ${_selected.length} 个表情吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                _emotes.removeWhere((e) => _selected.contains(e.id));
                _selected.clear();
              });
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('批量删除完成'), duration: Duration(seconds: 1)));
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }
}
