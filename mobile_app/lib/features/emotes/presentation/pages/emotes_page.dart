import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class EmotesPage extends ConsumerStatefulWidget {
  const EmotesPage({super.key});

  @override
  ConsumerState<EmotesPage> createState() => _EmotesPageState();
}

class _EmotesPageState extends ConsumerState<EmotesPage> {
  List<Map<String, dynamic>> _groups = [];
  List<Map<String, dynamic>> _emotes = [];
  String _selectedGroup = '全部';
  bool _batchMode = false;
  Set<String> _selected = {};
  bool _loading = true;
  String? _error;

  bool _asBool(dynamic value, {bool fallback = false}) {
    if (value is bool) return value;
    if (value is num) return value != 0;
    if (value is String) return value == '1' || value.toLowerCase() == 'true';
    return fallback;
  }

  List<String> _stringList(dynamic value) {
    if (value is! List) return const [];
    return value.map((item) => item.toString()).where((item) => item.isNotEmpty).toList(growable: false);
  }

  @override
  void initState() {
    super.initState();
    _loadData();
  }

  Future<void> _loadData() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(emoteServiceProvider);
      final groups = await svc.groups();
      final emotes = await svc.listEmotes();
      if (mounted) setState(() { _groups = groups; _emotes = emotes; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  List<Map<String, dynamic>> get _filteredEmotes {
    if (_selectedGroup == '全部') return _emotes;
    final group = _groups.where((item) => (item['name'] ?? '').toString() == _selectedGroup).firstOrNull;
    final groupId = (group?['id'] ?? '').toString();
    if (groupId.isEmpty) return const [];
    return _emotes.where((emote) {
      final ids = emote['groupIds'];
      return ids is List && ids.map((e) => e.toString()).contains(groupId);
    }).toList(growable: false);
  }

  int _groupCount(String groupName) {
    if (groupName == '全部') return _emotes.length;
    final group = _groups.where((item) => (item['name'] ?? '').toString() == groupName).firstOrNull;
    final groupId = (group?['id'] ?? '').toString();
    if (groupId.isEmpty) return 0;
    return _emotes.where((emote) {
      final ids = emote['groupIds'];
      return ids is List && ids.map((e) => e.toString()).contains(groupId);
    }).length;
  }

  String _groupNames(Map<String, dynamic> emote) {
    final ids = (emote['groupIds'] is List)
        ? (emote['groupIds'] as List).map((e) => e.toString()).toSet()
        : <String>{};
    if (ids.isEmpty) return '未分组';
    final names = _groups
        .where((group) => ids.contains((group['id'] ?? '').toString()))
        .map((group) => (group['name'] ?? '').toString())
        .where((name) => name.isNotEmpty)
        .toList(growable: false);
    return names.isEmpty ? '未分组' : names.join('、');
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '表情包管理',
        navigation: AmitiaAppBarNavigation.back,
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
        child: _loading
            ? const AmitiaLoadingState(message: '加载中...')
            : _error != null
                ? AmitiaErrorState(message: _error!, onRetry: _loadData)
                : Column(
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
                                padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
                                itemCount: _filteredEmotes.length,
                                separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
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
    final allGroups = ['全部', ..._groups.map((g) => (g['name'] ?? '').toString()).where((n) => n.isNotEmpty)];
    return SizedBox(
      height: 50,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
        itemCount: allGroups.length,
        separatorBuilder: (_, _) => SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final group = allGroups[index];
          final isSelected = _selectedGroup == group;
          final count = _groupCount(group);
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
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
      color: context.accentSoft,
      child: Row(
        children: [
          Text('已选 ${_selected.length} 项', style: AppTypography.bodySmall(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w600)),
          const Spacer(),
          if (_selected.isNotEmpty) ...[
            PopupMenuButton<String>(
              tooltip: '批量操作',
              onSelected: _runBatchAction,
              itemBuilder: (_) => const [
                PopupMenuItem(value: 'enable_ai', child: Text('启用 AI 使用')),
                PopupMenuItem(value: 'disable_ai', child: Text('关闭 AI 使用')),
                PopupMenuItem(value: 'enable', child: Text('启用表情')),
                PopupMenuItem(value: 'disable', child: Text('禁用表情')),
                PopupMenuItem(value: 'add_group', child: Text('加入分组')),
                PopupMenuItem(value: 'remove_group', child: Text('移出分组')),
              ],
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                child: Text('批量操作', style: TextStyle(fontSize: 13, color: context.accentPrimary, fontWeight: FontWeight.w600)),
              ),
            ),
            SizedBox(width: AppSpacing.xs),
            GestureDetector(
              onTap: () => _showBatchDeleteConfirm(context),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                decoration: BoxDecoration(color: context.error, borderRadius: AppRadius.brTag),
                child: const Text('批量删除', style: TextStyle(fontSize: 13, color: Colors.white)),
              ),
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _runBatchAction(String action) async {
    if (_selected.isEmpty) return;
    if (action == 'add_group' || action == 'remove_group') {
      await _showBatchGroupEditor(remove: action == 'remove_group');
      return;
    }
    final update = switch (action) {
      'enable_ai' => <String, dynamic>{'aiEnabled': true},
      'disable_ai' => <String, dynamic>{'aiEnabled': false},
      'enable' => <String, dynamic>{'enabled': true},
      'disable' => <String, dynamic>{'enabled': false},
      _ => <String, dynamic>{},
    };
    if (update.isEmpty) return;
    try {
      await ref.read(emoteServiceProvider).batchUpdateEmotes(_selected.toList(growable: false), update);
      await _loadData();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('批量更新完成')));
      }
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('批量更新失败：$e')));
    }
  }

  Future<void> _showBatchGroupEditor({required bool remove}) async {
    if (_groups.isEmpty) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('当前没有可用分组')),
        );
      }
      return;
    }
    String? selectedGroupId;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: Text(remove ? '批量移出分组' : '批量加入分组'),
          content: SizedBox(
            width: double.maxFinite,
            child: ListView(
              shrinkWrap: true,
              children: _groups.map((group) {
                final id = (group['id'] ?? '').toString();
                return RadioListTile<String>(
                  contentPadding: EdgeInsets.zero,
                  title: Text((group['name'] ?? id).toString()),
                  value: id,
                  groupValue: selectedGroupId,
                  onChanged: (value) => setDialogState(() => selectedGroupId = value),
                );
              }).toList(growable: false),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
            FilledButton(
              onPressed: selectedGroupId == null ? null : () => Navigator.pop(dialogContext, true),
              child: Text(remove ? '移出' : '加入'),
            ),
          ],
        ),
      ),
    );
    if (confirmed != true || selectedGroupId == null) return;
    try {
      final service = ref.read(emoteServiceProvider);
      final ids = _selected.toList(growable: false);
      if (remove) {
        for (final emoteId in ids) {
          await service.removeEmoteFromGroup(selectedGroupId!, emoteId);
        }
      } else {
        await service.addEmotesToGroup(selectedGroupId!, ids);
      }
      await _loadData();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(remove ? '已移出分组' : '已加入分组')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${remove ? '移出' : '加入'}分组失败：$e')),
        );
      }
    }
  }

  Widget _buildEmoteCard(BuildContext context, Map<String, dynamic> emote) {
    final id = (emote['id'] ?? '').toString();
    final name = (emote['name'] ?? '').toString();
    final meaning = (emote['meaning'] ?? '').toString();
    final group = _groupNames(emote);
    final emoji = (emote['emoji'] ?? '😊').toString();
    final characterIds = _stringList(emote['characterIds']);
    final isEnabled = _asBool(emote['isEnabled'] ?? emote['enabled'], fallback: true);
    final aiEnabled = _asBool(emote['aiEnabled']);
    final isSelected = _selected.contains(id);

    return AmitiaCard(
      border: Border.all(
        color: _batchMode && isSelected ? context.accentPrimary : context.borderPrimary,
        width: _batchMode && isSelected ? 1.5 : 0.5,
      ),
      onTap: () {
        if (_batchMode) {
          setState(() {
            if (isSelected) { _selected.remove(id); } else { _selected.add(id); }
          });
        } else {
          _showEmoteEditor(emote);
        }
      },
      child: Row(
        children: [
          if (_batchMode)
            Padding(
              padding: EdgeInsets.only(right: AppSpacing.sm),
              child: Icon(isSelected ? Icons.check_circle : Icons.radio_button_unchecked, size: 20, color: isSelected ? context.accentPrimary : context.textTertiary),
            ),
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Center(child: Text(emoji, style: const TextStyle(fontSize: 28))),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(child: Text(name, style: AppTypography.cardTitle(context))),
                    SizedBox(width: AppSpacing.sm),
                    if (characterIds.isNotEmpty)
                      AmitiaStatusBadge(label: '专属', type: BadgeType.accent),
                  ],
                ),
                const SizedBox(height: 2),
                Text('含义：$meaning', style: AppTypography.caption(context)),
                const SizedBox(height: 2),
                Row(
                  children: [
                    Text('分组：$group', style: AppTypography.label(context)),
                    SizedBox(width: AppSpacing.md),
                    AmitiaStatusBadge(
                      label: isEnabled ? '启用' : '禁用',
                      type: isEnabled ? BadgeType.success : BadgeType.neutral,
                    ),
                  ],
                ),
              ],
            ),
          ),
          SizedBox(width: AppSpacing.sm),
          Column(
            children: [
              Text('AI 使用', style: AppTypography.label(context)),
              const SizedBox(height: 2),
              AmitiaStatusBadge(
                label: aiEnabled ? '启用' : '关闭',
                type: aiEnabled ? BadgeType.success : BadgeType.neutral,
              ),
            ],
          ),
          if (!_batchMode) ...[
            SizedBox(width: AppSpacing.sm),
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

  Future<void> _showEmoteEditor(Map<String, dynamic> emote) async {
    final id = (emote['id'] ?? '').toString();
    if (id.isEmpty) return;
    final nameController = TextEditingController(text: (emote['name'] ?? '').toString());
    final meaningController = TextEditingController(text: (emote['meaning'] ?? '').toString());
    final keywordsController = TextEditingController(text: _stringList(emote['keywords']).join(', '));
    var enabled = _asBool(emote['isEnabled'] ?? emote['enabled'], fallback: true);
    var aiEnabled = _asBool(emote['aiEnabled']);
    var roleScope = (emote['roleScope'] ?? 'all_characters').toString();
    final selectedCharacters = _stringList(emote['characterIds']).toSet();
    final selectedGroups = _stringList(emote['groupIds']).toSet();
    final characters = ref.read(characterListProvider).valueOrNull ?? const [];

    final saved = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (context, setSheetState) => SafeArea(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.xl,
              AppSpacing.lg,
              AppSpacing.xl,
              MediaQuery.of(context).viewInsets.bottom + AppSpacing.xl,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('编辑表情', style: AppTypography.sectionTitle(context)),
                SizedBox(height: AppSpacing.md),
                AmitiaTextField(controller: nameController, hintText: '表情名称'),
                SizedBox(height: AppSpacing.sm),
                AmitiaTextField(controller: meaningController, hintText: '含义 / 使用语境'),
                SizedBox(height: AppSpacing.sm),
                AmitiaTextField(controller: keywordsController, hintText: '关键词，用逗号分隔'),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: const Text('启用表情'),
                  value: enabled,
                  onChanged: (value) => setSheetState(() => enabled = value),
                ),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: const Text('允许 AI 使用'),
                  value: aiEnabled,
                  onChanged: (value) => setSheetState(() => aiEnabled = value),
                ),
                DropdownButtonFormField<String>(
                  value: roleScope == 'selected_characters' ? 'selected_characters' : 'all_characters',
                  decoration: const InputDecoration(labelText: '角色作用域'),
                  items: const [
                    DropdownMenuItem(value: 'all_characters', child: Text('全部角色')),
                    DropdownMenuItem(value: 'selected_characters', child: Text('指定角色')),
                  ],
                  onChanged: (value) => setSheetState(() => roleScope = value ?? 'all_characters'),
                ),
                if (roleScope == 'selected_characters') ...[
                  SizedBox(height: AppSpacing.md),
                  Text('可使用角色', style: AppTypography.label(context)),
                  ...characters.map((character) => CheckboxListTile(
                        contentPadding: EdgeInsets.zero,
                        dense: true,
                        title: Text(character.name),
                        value: selectedCharacters.contains(character.id),
                        onChanged: (checked) => setSheetState(() {
                          if (checked == true) {
                            selectedCharacters.add(character.id);
                          } else {
                            selectedCharacters.remove(character.id);
                          }
                        }),
                      )),
                ],
                SizedBox(height: AppSpacing.md),
                Text('所属分组', style: AppTypography.label(context)),
                ..._groups.map((group) {
                  final groupId = (group['id'] ?? '').toString();
                  return CheckboxListTile(
                    contentPadding: EdgeInsets.zero,
                    dense: true,
                    title: Text((group['name'] ?? groupId).toString()),
                    value: selectedGroups.contains(groupId),
                    onChanged: (checked) => setSheetState(() {
                      if (checked == true) {
                        selectedGroups.add(groupId);
                      } else {
                        selectedGroups.remove(groupId);
                      }
                    }),
                  );
                }),
                SizedBox(height: AppSpacing.lg),
                AmitiaButton(
                  label: '保存表情设置',
                  isFullWidth: true,
                  onPressed: () async {
                    final keywords = keywordsController.text
                        .split(RegExp(r'[,，\n]'))
                        .map((value) => value.trim())
                        .where((value) => value.isNotEmpty)
                        .toList(growable: false);
                    try {
                      await ref.read(emoteServiceProvider).updateEmote(id, {
                        'name': nameController.text.trim(),
                        'meaning': meaningController.text.trim(),
                        'keywords': keywords,
                        'enabled': enabled,
                        'aiEnabled': aiEnabled,
                        'roleScope': roleScope,
                        'characterIds': roleScope == 'selected_characters' ? selectedCharacters.toList(growable: false) : <String>[],
                        'groupIds': selectedGroups.toList(growable: false),
                      });
                      if (sheetContext.mounted) Navigator.pop(sheetContext, true);
                    } catch (e) {
                      if (sheetContext.mounted) {
                        ScaffoldMessenger.of(sheetContext).showSnackBar(SnackBar(content: Text('保存失败：$e')));
                      }
                    }
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
    nameController.dispose();
    meaningController.dispose();
    keywordsController.dispose();
    if (saved == true) {
      await _loadData();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('表情设置已保存')));
    }
  }

  Future<void> _showGroupEditor(BuildContext context, Map<String, dynamic>? existing) async {
    final isEdit = existing != null;
    final nameCtrl = TextEditingController(text: (existing?['name'] ?? '').toString());
    var coverEmoteId = (existing?['coverEmoteId'] ?? '').toString();

    await showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => StatefulBuilder(
        builder: (context, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(AppSpacing.xl, AppSpacing.lg, AppSpacing.xl, MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(isEdit ? '编辑分组' : '新建分组', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              AmitiaTextField(controller: nameCtrl, hintText: '输入分组名称'),
              if (isEdit && _emotes.isNotEmpty) ...[
                SizedBox(height: AppSpacing.md),
                DropdownButtonFormField<String>(
                  value: _emotes.any((item) => (item['id'] ?? '').toString() == coverEmoteId) ? coverEmoteId : '',
                  decoration: const InputDecoration(labelText: '分组封面'),
                  items: [
                    const DropdownMenuItem(value: '', child: Text('自动 / 无指定封面')),
                    ..._emotes.map((item) => DropdownMenuItem(
                          value: (item['id'] ?? '').toString(),
                          child: Text((item['name'] ?? item['id'] ?? '').toString()),
                        )),
                  ],
                  onChanged: (value) => setSheetState(() => coverEmoteId = value ?? ''),
                ),
              ],
              SizedBox(height: AppSpacing.xl),
              Row(
                children: [
                  if (isEdit) ...[
                    IconButton(
                      tooltip: '前移分组',
                      onPressed: () async {
                        await _moveGroup(existing!, -1);
                        if (ctx.mounted) Navigator.pop(ctx);
                      },
                      icon: const Icon(Icons.arrow_back),
                    ),
                    IconButton(
                      tooltip: '后移分组',
                      onPressed: () async {
                        await _moveGroup(existing!, 1);
                        if (ctx.mounted) Navigator.pop(ctx);
                      },
                      icon: const Icon(Icons.arrow_forward),
                    ),
                    const Spacer(),
                  ],
                  Expanded(
                    child: AmitiaButton(
                      label: isEdit ? '保存' : '创建',
                      onPressed: () async {
                        if (nameCtrl.text.trim().isEmpty) return;
                        final svc = ref.read(emoteServiceProvider);
                        try {
                          if (isEdit) {
                            final id = (existing['id'] ?? '').toString();
                            await svc.updateGroup(id, {
                              'name': nameCtrl.text.trim(),
                              'coverEmoteId': coverEmoteId,
                            });
                          } else {
                            await svc.createGroup({'name': nameCtrl.text.trim()});
                          }
                          if (ctx.mounted) Navigator.pop(ctx);
                          await _loadData();
                        } catch (e) {
                          if (ctx.mounted) ScaffoldMessenger.of(ctx).showSnackBar(SnackBar(content: Text('保存分组失败：$e')));
                        }
                      },
                    ),
                  ),
                  if (isEdit) ...[
                    SizedBox(width: AppSpacing.sm),
                    AmitiaButton(
                      label: '删除',
                      isDestructive: true,
                      onPressed: () {
                        Navigator.pop(ctx);
                        _showDeleteGroupConfirm(context, existing);
                      },
                    ),
                  ],
                ],
              ),
            ],
          ),
        ),
      ),
    );
    nameCtrl.dispose();
  }

  Future<void> _moveGroup(Map<String, dynamic> group, int delta) async {
    final id = (group['id'] ?? '').toString();
    final current = _groups.indexWhere((item) => (item['id'] ?? '').toString() == id);
    final target = current + delta;
    if (current < 0 || target < 0 || target >= _groups.length) return;
    final reordered = List<Map<String, dynamic>>.from(_groups);
    final item = reordered.removeAt(current);
    reordered.insert(target, item);
    try {
      await ref.read(emoteServiceProvider).reorderGroups(
        reordered.map((item) => (item['id'] ?? '').toString()).where((value) => value.isNotEmpty).toList(growable: false),
      );
      await _loadData();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('调整分组顺序失败：$e')));
    }
  }

  void _showDeleteGroupConfirm(BuildContext context, Map<String, dynamic> group) {
    final groupName = (group['name'] ?? '').toString();
    final groupId = (group['id'] ?? '').toString();
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除分组', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除分组「$groupName」吗？组内表情将被移至默认分组。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              final svc = ref.read(emoteServiceProvider);
              await svc.deleteGroup(groupId);
              if (ctx.mounted) Navigator.pop(ctx);
              _loadData();
              if (mounted) {
                ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('分组已删除'), duration: Duration(seconds: 1)));
              }
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  Future<void> _showImportPreview(BuildContext context) async {
    final result = await FilePicker.platform.pickFiles(
      allowMultiple: true,
      type: FileType.image,
    );
    if (!mounted || result == null || result.files.isEmpty) return;
    final selectedFiles = result.files.where((file) => file.path != null).toList(growable: false);
    if (selectedFiles.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('未能读取所选文件路径')),
      );
      return;
    }

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('批量导入预览', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('已选择 ${selectedFiles.length} 个图片文件。', style: AppTypography.bodySmall(context)),
            SizedBox(height: AppSpacing.sm),
            ConstrainedBox(
              constraints: const BoxConstraints(maxHeight: 180),
              child: ListView(
                shrinkWrap: true,
                children: selectedFiles
                    .map((file) => Padding(
                          padding: const EdgeInsets.symmetric(vertical: 2),
                          child: Text(file.name, style: AppTypography.caption(context)),
                        ))
                    .toList(growable: false),
              ),
            ),
            SizedBox(height: AppSpacing.sm),
            Text(
              '导入后可继续在表情详情中配置含义、关键词、角色作用域和分组。',
              style: AppTypography.caption(context),
            ),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: Text('确认导入', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    try {
      final svc = ref.read(emoteServiceProvider);
      final configs = selectedFiles
          .map((file) => <String, dynamic>{
                'sourceName': file.name,
                'name': file.name.replaceFirst(RegExp(r'\.[^.]+$'), ''),
                'aiEnabled': false,
                'roleScope': 'all_characters',
              })
          .toList(growable: false);
      final response = await svc.batchUploadEmotes(
        selectedFiles.map((file) => file.path!).toList(growable: false),
        configs: configs,
      );
      final summary = response['summary'] is Map
          ? Map<String, dynamic>.from(response['summary'] as Map)
          : <String, dynamic>{};
      await _loadData();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            '导入完成：成功 ${summary['success'] ?? 0}，重复 ${summary['duplicates'] ?? 0}，失败 ${summary['failed'] ?? 0}',
          ),
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('批量导入失败: $e'), backgroundColor: context.error),
        );
      }
    }
  }

  void _showDeleteConfirm(BuildContext context, Map<String, dynamic> emote) {
    final emoteName = (emote['name'] ?? '').toString();
    final emoteId = (emote['id'] ?? '').toString();
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除表情', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「$emoteName」吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              final svc = ref.read(emoteServiceProvider);
              await svc.deleteEmote(emoteId);
              if (ctx.mounted) Navigator.pop(ctx);
              _loadData();
              if (mounted) {
                ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('表情已删除'), duration: Duration(seconds: 1)));
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
        content: Text('确定要删除选中的 ${_selected.length} 个表情吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              final svc = ref.read(emoteServiceProvider);
              for (final id in _selected) {
                await svc.deleteEmote(id);
              }
              if (ctx.mounted) Navigator.pop(ctx);
              _loadData();
              if (mounted) {
                _selected.clear();
                ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('批量删除完成'), duration: Duration(seconds: 1)));
              }
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }
}
