import 'dart:convert';
import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/worldbook.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class WorldBookPage extends ConsumerStatefulWidget {
  const WorldBookPage({super.key});

  @override
  ConsumerState<WorldBookPage> createState() => _WorldBookPageState();
}

class _WorldBookPageState extends ConsumerState<WorldBookPage> {
  String _filterType = '';

  static const _matchTypes = <String, String>{
    'keyword': '关键词匹配',
    'exact': '精确匹配',
    'regex': '正则匹配',
  };
  static const _scopes = <String, String>{
    'full_context': '全部上下文',
    'user_message': '仅用户消息',
    'assistant_reply': '仅 AI 回复',
  };

  @override
  Widget build(BuildContext context) {
    final entriesAsync = ref.watch(worldBookListProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '世界书',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
        actions: [
          AmitiaIconButton(icon: Icons.science_outlined, onPressed: _showMatchTester),
          AmitiaIconButton(icon: Icons.file_upload_outlined, onPressed: _importJson),
          AmitiaIconButton(icon: Icons.file_download_outlined, onPressed: _exportJson),
          AmitiaIconButton(icon: Icons.add, onPressed: () => _showEditor(null)),
        ],
      ),
      body: SafeArea(
        top: false,
        child: entriesAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => AmitiaErrorState(
            message: error.toString().replaceFirst('Exception: ', ''),
            onRetry: () => ref.invalidate(worldBookListProvider),
          ),
          data: (entries) {
            final visible = _filterType.isEmpty
                ? entries
                : entries.where((entry) => entry.matchType == _filterType).toList(growable: false);
            return Column(
              children: [
                _buildFilters(context),
                Expanded(
                  child: visible.isEmpty
                      ? AmitiaEmptyState(
                          icon: Icons.menu_book_outlined,
                          title: '暂无世界书规则',
                          subtitle: '添加规则后，匹配内容会注入对话上下文',
                          actionText: '新增规则',
                          onAction: () => _showEditor(null),
                        )
                      : RefreshIndicator(
                          onRefresh: () async => ref.invalidate(worldBookListProvider),
                          child: ListView.separated(
                            padding: EdgeInsets.all(AppSpacing.pagePadding),
                            itemCount: visible.length,
                            separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                            itemBuilder: (context, index) => _buildCard(visible[index]),
                          ),
                        ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget _buildFilters(BuildContext context) {
    final types = <MapEntry<String, String>>[
      const MapEntry('', '全部'),
      ..._matchTypes.entries,
    ];
    return SizedBox(
      height: 42,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        itemCount: types.length,
        separatorBuilder: (_, _) => SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final item = types[index];
          final selected = item.key == _filterType;
          return GestureDetector(
            onTap: () => setState(() => _filterType = item.key),
            child: Container(
              alignment: Alignment.center,
              padding: const EdgeInsets.symmetric(horizontal: 12),
              decoration: BoxDecoration(
                color: selected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Text(item.value, style: TextStyle(fontSize: 13, color: selected ? Colors.white : context.textSecondary)),
            ),
          );
        },
      ),
    );
  }

  Widget _buildCard(WorldBookDto entry) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(child: Text(entry.matchPattern, style: AppTypography.cardTitle(context))),
              AmitiaStatusBadge(label: 'P${entry.priority}', type: BadgeType.accent),
            ],
          ),
          SizedBox(height: AppSpacing.xs),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.xs,
            children: [
              AmitiaStatusBadge(label: _matchTypes[entry.matchType] ?? entry.matchType, type: BadgeType.info),
              AmitiaStatusBadge(label: _scopes[entry.matchScope] ?? entry.matchScope, type: BadgeType.neutral),
              AmitiaStatusBadge(label: '命中 ${entry.hitCount}', type: BadgeType.success),
              if (entry.characterId.isNotEmpty) const AmitiaStatusBadge(label: '角色限定', type: BadgeType.warning),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Container(
            width: double.infinity,
            padding: EdgeInsets.all(AppSpacing.md),
            decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brSmall),
            child: Text(entry.injectContent, style: AppTypography.bodySmall(context).copyWith(height: 1.5)),
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              TextButton.icon(onPressed: () => _showEditor(entry), icon: const Icon(Icons.edit_outlined, size: 16), label: const Text('编辑')),
              TextButton.icon(
                onPressed: () => _delete(entry),
                icon: Icon(Icons.delete_outline, size: 16, color: context.error),
                label: Text('删除', style: TextStyle(color: context.error)),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _showEditor(WorldBookDto? existing) async {
    final patternController = TextEditingController(text: existing?.matchPattern ?? '');
    final contentController = TextEditingController(text: existing?.injectContent ?? '');
    final characterController = TextEditingController(text: existing?.characterId ?? '');
    var matchType = existing?.matchType ?? 'keyword';
    var matchScope = existing?.matchScope ?? 'full_context';
    var priority = existing?.priority ?? 0;

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.xl,
            AppSpacing.lg,
            AppSpacing.xl,
            MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.xl,
          ),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(existing == null ? '新增规则' : '编辑规则', style: AppTypography.sectionTitle(context)),
                SizedBox(height: AppSpacing.lg),
                DropdownButtonFormField<String>(
                  value: matchType,
                  decoration: const InputDecoration(labelText: '匹配类型', border: OutlineInputBorder()),
                  items: _matchTypes.entries.map((e) => DropdownMenuItem(value: e.key, child: Text(e.value))).toList(growable: false),
                  onChanged: (value) {
                    if (value != null) setSheetState(() => matchType = value);
                  },
                ),
                SizedBox(height: AppSpacing.md),
                AmitiaTextField(controller: patternController, hintText: '正则 / 精确文本 / 关键词（逗号分隔）'),
                SizedBox(height: AppSpacing.md),
                DropdownButtonFormField<String>(
                  value: matchScope,
                  decoration: const InputDecoration(labelText: '匹配范围', border: OutlineInputBorder()),
                  items: _scopes.entries.map((e) => DropdownMenuItem(value: e.key, child: Text(e.value))).toList(growable: false),
                  onChanged: (value) {
                    if (value != null) setSheetState(() => matchScope = value);
                  },
                ),
                SizedBox(height: AppSpacing.md),
                AmitiaTextField(controller: contentController, hintText: '命中后注入的内容', maxLines: 4),
                SizedBox(height: AppSpacing.md),
                AmitiaTextField(controller: characterController, hintText: '角色 ID（留空表示全局）'),
                SizedBox(height: AppSpacing.md),
                Row(
                  children: [
                    Text('优先级', style: AppTypography.label(context)),
                    const Spacer(),
                    IconButton(onPressed: priority > 0 ? () => setSheetState(() => priority--) : null, icon: const Icon(Icons.remove_circle_outline)),
                    Text('$priority', style: AppTypography.body(context)),
                    IconButton(onPressed: priority < 10 ? () => setSheetState(() => priority++) : null, icon: const Icon(Icons.add_circle_outline)),
                  ],
                ),
                SizedBox(height: AppSpacing.lg),
                AmitiaButton(
                  label: existing == null ? '创建' : '保存',
                  isFullWidth: true,
                  onPressed: () async {
                    if (patternController.text.trim().isEmpty || contentController.text.trim().isEmpty) {
                      ScaffoldMessenger.of(sheetContext).showSnackBar(const SnackBar(content: Text('匹配模式和注入内容不能为空')));
                      return;
                    }
                    final payload = <String, dynamic>{
                      'matchType': matchType,
                      'matchPattern': patternController.text.trim(),
                      'matchScope': matchScope,
                      'injectContent': contentController.text.trim(),
                      'priority': priority,
                      'characterId': characterController.text.trim(),
                    };
                    try {
                      final service = ref.read(worldBookServiceProvider);
                      if (existing == null) {
                        await service.create(payload);
                      } else {
                        await service.update(existing.id, payload);
                      }
                      ref.invalidate(worldBookListProvider);
                      if (sheetContext.mounted) Navigator.pop(sheetContext);
                    } catch (error) {
                      if (sheetContext.mounted) {
                        ScaffoldMessenger.of(sheetContext).showSnackBar(SnackBar(content: Text('保存失败：$error')));
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
  }

  Future<void> _showMatchTester() async {
    final controller = TextEditingController();
    List<WorldBookMatchDto> matches = const [];
    var testing = false;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.xl,
            AppSpacing.lg,
            AppSpacing.xl,
            MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.xl,
          ),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('在线匹配测试', style: AppTypography.sectionTitle(context)),
                SizedBox(height: AppSpacing.md),
                AmitiaTextField(controller: controller, hintText: '输入一段待测试文本', maxLines: 4),
                SizedBox(height: AppSpacing.md),
                AmitiaButton(
                  label: testing ? '测试中…' : '运行测试',
                  isFullWidth: true,
                  onPressed: testing
                      ? null
                      : () async {
                          final text = controller.text.trim();
                          if (text.isEmpty) return;
                          setSheetState(() => testing = true);
                          try {
                            final result = await ref.read(worldBookServiceProvider).testMatch(text);
                            if (sheetContext.mounted) setSheetState(() => matches = result);
                          } finally {
                            if (sheetContext.mounted) setSheetState(() => testing = false);
                          }
                        },
                ),
                SizedBox(height: AppSpacing.md),
                if (!testing && matches.isEmpty)
                  Text('暂无命中', style: AppTypography.bodySmall(context))
                else
                  ...matches.map(
                    (match) => Padding(
                      padding: EdgeInsets.only(bottom: AppSpacing.sm),
                      child: AmitiaCard(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(match.entry.matchPattern, style: AppTypography.cardTitle(context)),
                            const SizedBox(height: 4),
                            Text('命中：${match.hitText}', style: AppTypography.bodySmall(context)),
                            const SizedBox(height: 4),
                            Text(match.entry.injectContent, style: AppTypography.caption(context)),
                          ],
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _importJson() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['json'],
      withData: true,
    );
    if (picked == null || picked.files.isEmpty) return;
    try {
      final file = picked.files.single;
      final raw = file.bytes != null
          ? utf8.decode(file.bytes!)
          : file.path != null
              ? await File(file.path!).readAsString()
              : '';
      final decoded = jsonDecode(raw);
      if (decoded is! List) throw const FormatException('JSON 顶层必须是数组');
      var success = 0;
      for (final item in decoded.whereType<Map>()) {
        final data = Map<String, dynamic>.from(item);
        if ((data['matchType'] ?? '').toString().isEmpty ||
            (data['matchPattern'] ?? '').toString().isEmpty ||
            (data['injectContent'] ?? '').toString().isEmpty) {
          continue;
        }
        await ref.read(worldBookServiceProvider).create(data);
        success++;
      }
      ref.invalidate(worldBookListProvider);
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('导入完成：$success 条')));
    } catch (error) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('导入失败：$error')));
    }
  }

  Future<void> _exportJson() async {
    final entries = await ref.read(worldBookServiceProvider).list(pageSize: 500);
    final payload = entries
        .map((entry) => {
              'matchType': entry.matchType,
              'matchPattern': entry.matchPattern,
              'matchScope': entry.matchScope,
              'injectContent': entry.injectContent,
              'priority': entry.priority,
              if (entry.characterId.isNotEmpty) 'characterId': entry.characterId,
            })
        .toList(growable: false);
    final path = await FilePicker.platform.saveFile(dialogTitle: '导出世界书', fileName: 'world_book.json');
    if (path == null) return;
    try {
      await File(path).writeAsString(const JsonEncoder.withIndent('  ').convert(payload));
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('世界书已导出')));
    } catch (error) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('导出失败：$error')));
    }
  }

  Future<void> _delete(WorldBookDto entry) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除世界书规则'),
        content: Text('确定删除“${entry.matchPattern}”吗？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: Text('删除', style: TextStyle(color: context.error))),
        ],
      ),
    );
    if (confirmed != true) return;
    await ref.read(worldBookServiceProvider).delete(entry.id);
    ref.invalidate(worldBookListProvider);
  }
}
