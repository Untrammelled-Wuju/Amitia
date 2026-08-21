import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class CharacterProactivePage extends ConsumerStatefulWidget {
  final String characterId;
  const CharacterProactivePage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterProactivePage> createState() => _CharacterProactivePageState();
}

class _CharacterProactivePageState extends ConsumerState<CharacterProactivePage> {
  bool _loading = true;
  bool _masterEnabled = true;
  List<Map<String, dynamic>> _rules = const [];
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final proactive = ref.read(proactiveServiceProvider);
      final companion = ref.read(companionServiceProvider);
      final values = await Future.wait<dynamic>([
        proactive.rules(characterId: widget.characterId),
        companion.activeMessageSetting(characterId: widget.characterId),
      ]);
      if (!mounted) return;
      final setting = values[1] as Map<String, dynamic>?;
      final rawEnabled = setting?['enabled'];
      setState(() {
        _rules = (values[0] as List<Map<String, dynamic>>);
        _masterEnabled = rawEnabled == true || rawEnabled == 1 || rawEnabled == null;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _setMaster(bool enabled) async {
    final previous = _masterEnabled;
    setState(() => _masterEnabled = enabled);
    try {
      await ref.read(companionServiceProvider).updateActiveMessageSetting(
        {'enabled': enabled},
        characterId: widget.characterId,
      );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('主动消息已${enabled ? '开启' : '关闭'}')),
        );
      }
    } catch (e) {
      if (!mounted) return;
      setState(() => _masterEnabled = previous);
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('保存失败：$e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '主动消息',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
        actions: [
          AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _load),
          AmitiaIconButton(icon: Icons.add, tooltip: '新建规则', onPressed: () => _showRuleEditor(null)),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? Center(child: Text('加载失败：$_error'))
                : RefreshIndicator(
                    onRefresh: _load,
                    child: ListView(
                      padding: EdgeInsets.all(AppSpacing.pagePadding),
                      children: [
                        AmitiaCard(
                          backgroundColor: _masterEnabled ? context.accentSoft : null,
                          child: AmitiaSwitchTile(
                            title: '主动消息总开关',
                            subtitle: _masterEnabled
                                ? '当前角色会执行启用的主动消息规则'
                                : '当前角色的主动消息生成已暂停',
                            value: _masterEnabled,
                            onChanged: _setMaster,
                          ),
                        ),
                        SizedBox(height: AppSpacing.sectionGap),
                        AmitiaSectionHeader(title: '角色规则 (${_rules.length})'),
                        SizedBox(height: AppSpacing.sm),
                        if (_rules.isEmpty)
                          AmitiaCard(child: Text('暂无主动消息规则', style: AppTypography.caption(context)))
                        else
                          ..._rules.map(_ruleCard),
                        SizedBox(height: AppSpacing.xxl),
                      ],
                    ),
                  ),
      ),
    );
  }

  Widget _ruleCard(Map<String, dynamic> rule) {
    final id = (rule['id'] ?? '').toString();
    final enabledRaw = rule['enabled'];
    final enabled = enabledRaw == true || enabledRaw == 1;
    final name = (rule['name'] ?? '未命名规则').toString();
    final cron = (rule['scheduleCron'] ?? '').toString();
    final channel = (rule['channel'] ?? 'web').toString();
    final prompt = (rule['promptTemplate'] ?? '').toString();
    final quietStart = (rule['quietStart'] ?? '').toString();
    final quietEnd = (rule['quietEnd'] ?? '').toString();
    final maxPerDay = (rule['maxPerDay'] as num?)?.toInt() ?? 1;
    final randomMinutes = (rule['randomMinutes'] as num?)?.toInt() ?? 0;
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text(name, style: AppTypography.cardTitle(context))),
                AmitiaStatusBadge(label: channel, type: BadgeType.accent),
                SizedBox(width: AppSpacing.sm),
                Switch(
                  value: enabled,
                  onChanged: id.isEmpty
                      ? null
                      : (value) async {
                          try {
                            await ref.read(proactiveServiceProvider).toggleRule(id, value);
                            await _load();
                          } catch (e) {
                            if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('切换失败：$e')));
                          }
                        },
                ),
              ],
            ),
            if (cron.isNotEmpty) Text('Cron：$cron', style: AppTypography.caption(context)),
            if (quietStart.isNotEmpty || quietEnd.isNotEmpty)
              Text('静默时段：${quietStart.isEmpty ? '—' : quietStart} - ${quietEnd.isEmpty ? '—' : quietEnd}', style: AppTypography.caption(context)),
            Text('每天最多 $maxPerDay 次 · 随机偏移 $randomMinutes 分钟', style: AppTypography.caption(context)),
            if (prompt.isNotEmpty) ...[
              SizedBox(height: AppSpacing.sm),
              Text(prompt, maxLines: 3, overflow: TextOverflow.ellipsis, style: AppTypography.bodySmall(context)),
            ],
            SizedBox(height: AppSpacing.md),
            Row(
              children: [
                AmitiaButtonOutline(label: '编辑', onPressed: () => _showRuleEditor(rule)),
                SizedBox(width: AppSpacing.sm),
                AmitiaButtonOutline(label: '测试', onPressed: id.isEmpty ? null : () => _testRule(id, name)),
                const Spacer(),
                IconButton(onPressed: id.isEmpty ? null : () => _deleteRule(id, name), icon: Icon(Icons.delete_outline, color: context.error)),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showRuleEditor(Map<String, dynamic>? existing) {
    final isEdit = existing != null;
    final name = TextEditingController(text: (existing?['name'] ?? '').toString());
    final cron = TextEditingController(text: (existing?['scheduleCron'] ?? '0 9 * * *').toString());
    final quietStart = TextEditingController(text: (existing?['quietStart'] ?? '23:00').toString());
    final quietEnd = TextEditingController(text: (existing?['quietEnd'] ?? '07:00').toString());
    final prompt = TextEditingController(text: (existing?['promptTemplate'] ?? '').toString());
    int maxPerDay = (existing?['maxPerDay'] as num?)?.toInt() ?? 1;
    int randomMinutes = (existing?['randomMinutes'] as num?)?.toInt() ?? 30;
    String channel = (existing?['channel'] ?? 'web').toString();
    final id = (existing?['id'] ?? '').toString();

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => SingleChildScrollView(
          padding: EdgeInsets.fromLTRB(AppSpacing.xl, AppSpacing.lg, AppSpacing.xl, MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(isEdit ? '编辑主动消息规则' : '新建主动消息规则', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              AmitiaTextField(controller: name, hintText: '规则名称'),
              SizedBox(height: AppSpacing.md),
              AmitiaTextField(controller: cron, hintText: 'Cron，例如 0 9 * * *'),
              SizedBox(height: AppSpacing.md),
              Row(children: [
                Expanded(child: AmitiaTextField(controller: quietStart, hintText: '静默开始 23:00')),
                SizedBox(width: AppSpacing.sm),
                Expanded(child: AmitiaTextField(controller: quietEnd, hintText: '静默结束 07:00')),
              ]),
              SizedBox(height: AppSpacing.md),
              Text('发送渠道', style: AppTypography.label(context)),
              DropdownButton<String>(
                value: channel,
                isExpanded: true,
                items: const ['web', 'wechat', 'qq', 'all'].map((value) => DropdownMenuItem(value: value, child: Text(value))).toList(),
                onChanged: (value) => setSheetState(() => channel = value ?? 'web'),
              ),
              SizedBox(height: AppSpacing.md),
              Text('每天最多 $maxPerDay 次', style: AppTypography.label(context)),
              Slider(value: maxPerDay.toDouble(), min: 1, max: 20, divisions: 19, onChanged: (v) => setSheetState(() => maxPerDay = v.round())),
              Text('随机偏移 $randomMinutes 分钟', style: AppTypography.label(context)),
              Slider(value: randomMinutes.toDouble(), min: 0, max: 180, divisions: 36, onChanged: (v) => setSheetState(() => randomMinutes = v.round())),
              AmitiaTextField(controller: prompt, maxLines: 4, hintText: '主动消息 Prompt'),
              SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () async {
                  if (name.text.trim().isEmpty || cron.text.trim().isEmpty) return;
                  final data = <String, dynamic>{
                    'name': name.text.trim(),
                    'characterId': widget.characterId,
                    'channel': channel,
                    'ruleType': 'cron',
                    'scheduleCron': cron.text.trim(),
                    'quietStart': quietStart.text.trim(),
                    'quietEnd': quietEnd.text.trim(),
                    'maxPerDay': maxPerDay,
                    'promptTemplate': prompt.text.trim(),
                    'randomMinutes': randomMinutes,
                  };
                  try {
                    if (isEdit) {
                      await ref.read(proactiveServiceProvider).updateRule(id, data);
                    } else {
                      await ref.read(proactiveServiceProvider).createRule(data);
                    }
                    if (sheetContext.mounted) Navigator.pop(sheetContext);
                    await _load();
                  } catch (e) {
                    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('保存失败：$e')));
                  }
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _testRule(String id, String name) async {
    try {
      await ref.read(proactiveServiceProvider).triggerRule(id);
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已触发规则：$name')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('触发失败：$e')));
    }
  }

  Future<void> _deleteRule(String id, String name) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除规则'),
        content: Text('确定删除“$name”吗？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('删除')),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(proactiveServiceProvider).deleteRule(id);
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('删除失败：$e')));
    }
  }
}
