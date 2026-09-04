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
  Map<String, dynamic> _activeSetting = const {
    'enabled': true,
    'activeLevel': 40,
    'minInterval': 60,
    'quietStart': '23:00',
    'quietEnd': '07:00',
    'maxPerDay': 6,
    'maxDailyCalls': 10,
    'channel': 'all',
    'unrepliedSlowdownEnabled': true,
    'unrepliedSlowdownAfter': 2,
    'unrepliedCooldownMultiplier': 2.0,
    'unrepliedRecoveryOnReply': true,
  };
  List<Map<String, dynamic>> _rules = const [];
  List<dynamic> _conversations = const [];
  List<Map<String, dynamic>> _history = const [];
  Map<String, dynamic> _status = const {};
  Map<String, dynamic> _queue = const {};
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
        proactive.status(characterId: widget.characterId),
        proactive.queueSummary(),
        proactive.history(),
        ref.read(chatServiceProvider).listConversations(),
      ]);
      if (!mounted) return;
      final setting = values[1] as Map<String, dynamic>?;
      final rawEnabled = setting?['enabled'];
      final conversations = (values[5] as List).where((item) => item.characterId == widget.characterId).toList(growable: false);
      setState(() {
        _rules = (values[0] as List<Map<String, dynamic>>);
        _activeSetting = {
          ..._activeSetting,
          if (setting != null) ...setting,
        };
        _masterEnabled = rawEnabled == true || rawEnabled == 1 || rawEnabled == null;
        _status = values[2] as Map<String, dynamic>? ?? const {};
        _queue = values[3] as Map<String, dynamic>? ?? const {};
        _history = values[4] as List<Map<String, dynamic>>;
        _conversations = conversations;
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
    setState(() {
      _masterEnabled = enabled;
      _activeSetting = {..._activeSetting, 'enabled': enabled};
    });
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
      setState(() {
        _masterEnabled = previous;
        _activeSetting = {..._activeSetting, 'enabled': previous};
      });
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
          AmitiaIconButton(icon: Icons.restart_alt, tooltip: '恢复系统预设', onPressed: _resetPresets),
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
                        SizedBox(height: AppSpacing.sm),
                        _globalSettingsCard(),
                        SizedBox(height: AppSpacing.sm),
                        _runtimeSummaryCard(),
                        SizedBox(height: AppSpacing.sectionGap),
                        AmitiaSectionHeader(title: '角色规则 (${_rules.length})'),
                        SizedBox(height: AppSpacing.sm),
                        if (_rules.isEmpty)
                          AmitiaCard(child: Text('暂无主动消息规则', style: AppTypography.caption(context)))
                        else
                          ..._rules.map(_ruleCard),
                        SizedBox(height: AppSpacing.sectionGap),
                        AmitiaSectionHeader(title: '最近运行历史'),
                        SizedBox(height: AppSpacing.sm),
                        _historyCard(),
                        SizedBox(height: AppSpacing.xxl),
                      ],
                    ),
                  ),
      ),
    );
  }

  int _intSetting(String key, int fallback) {
    final value = _activeSetting[key];
    if (value is num) return value.toInt();
    return int.tryParse(value?.toString() ?? '') ?? fallback;
  }

  double _doubleSetting(String key, double fallback) {
    final value = _activeSetting[key];
    if (value is num) return value.toDouble();
    return double.tryParse(value?.toString() ?? '') ?? fallback;
  }

  bool _boolSetting(String key, bool fallback) {
    final value = _activeSetting[key];
    if (value is bool) return value;
    if (value is num) return value != 0;
    if (value == null) return fallback;
    return value.toString().toLowerCase() == 'true' || value.toString() == '1';
  }

  void _setSetting(String key, dynamic value) {
    setState(() => _activeSetting = {..._activeSetting, key: value});
  }

  Future<void> _saveGlobalSettings() async {
    try {
      final payload = Map<String, dynamic>.from(_activeSetting)..['enabled'] = _masterEnabled;
      await ref.read(companionServiceProvider).updateActiveMessageSetting(
        payload,
        characterId: widget.characterId,
      );
      final saved = await ref.read(companionServiceProvider).activeMessageSetting(characterId: widget.characterId);
      if (!mounted) return;
      if (saved != null) setState(() => _activeSetting = {..._activeSetting, ...saved});
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('全局主动消息参数已保存')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('保存失败：$e')));
    }
  }

  Future<void> _pickQuietTime(String key) async {
    final raw = (_activeSetting[key] ?? (key == 'quietStart' ? '23:00' : '07:00')).toString();
    final parts = raw.split(':');
    final initial = TimeOfDay(
      hour: parts.isNotEmpty ? int.tryParse(parts.first) ?? 0 : 0,
      minute: parts.length > 1 ? int.tryParse(parts[1]) ?? 0 : 0,
    );
    final picked = await showTimePicker(context: context, initialTime: initial);
    if (picked == null) return;
    _setSetting(key, '${picked.hour.toString().padLeft(2, '0')}:${picked.minute.toString().padLeft(2, '0')}');
  }

  Widget _globalSettingsCard() {
    final slowdown = _boolSetting('unrepliedSlowdownEnabled', true);
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('全局主动消息参数', style: AppTypography.cardTitle(context)),
          SizedBox(height: AppSpacing.md),
          _sliderSetting(
            label: '主动程度',
            value: _intSetting('activeLevel', 40).toDouble(),
            min: 1,
            max: 100,
            divisions: 99,
            suffix: '%',
            onChanged: (value) => _setSetting('activeLevel', value.round()),
          ),
          _sliderSetting(
            label: '最小间隔',
            value: _intSetting('minInterval', 60).clamp(5, 480).toDouble(),
            min: 5,
            max: 480,
            divisions: 95,
            suffix: ' 分钟',
            onChanged: (value) => _setSetting('minInterval', value.round()),
          ),
          _sliderSetting(
            label: '每天最多主动消息',
            value: _intSetting('maxPerDay', 6).clamp(1, 24).toDouble(),
            min: 1,
            max: 24,
            divisions: 23,
            suffix: ' 次',
            onChanged: (value) => _setSetting('maxPerDay', value.round()),
          ),
          _sliderSetting(
            label: '每天最多模型调用',
            value: _intSetting('maxDailyCalls', 10).clamp(1, 50).toDouble(),
            min: 1,
            max: 50,
            divisions: 49,
            suffix: ' 次',
            onChanged: (value) => _setSetting('maxDailyCalls', value.round()),
          ),
          Row(
            children: [
              Expanded(child: _timeButton('静默开始', (_activeSetting['quietStart'] ?? '23:00').toString(), () => _pickQuietTime('quietStart'))),
              SizedBox(width: AppSpacing.sm),
              Expanded(child: _timeButton('静默结束', (_activeSetting['quietEnd'] ?? '07:00').toString(), () => _pickQuietTime('quietEnd'))),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          DropdownButtonFormField<String>(
            value: (_activeSetting['channel'] ?? 'all').toString(),
            decoration: const InputDecoration(labelText: '发送通道'),
            items: const [
              DropdownMenuItem(value: 'all', child: Text('全部/自动')), 
              DropdownMenuItem(value: 'web', child: Text('Web')),
              DropdownMenuItem(value: 'wechat', child: Text('微信')),
              DropdownMenuItem(value: 'qq', child: Text('QQ')),
            ],
            onChanged: (value) => _setSetting('channel', value ?? 'all'),
          ),
          SizedBox(height: AppSpacing.sm),
          AmitiaSwitchTile(
            title: '未回复时自动降频',
            subtitle: '连续主动消息没有得到用户回复时，扩大最小间隔',
            value: slowdown,
            onChanged: (value) => _setSetting('unrepliedSlowdownEnabled', value),
          ),
          if (slowdown) ...[
            _sliderSetting(
              label: '未回复触发阈值',
              value: _intSetting('unrepliedSlowdownAfter', 2).clamp(1, 10).toDouble(),
              min: 1,
              max: 10,
              divisions: 9,
              suffix: ' 条',
              onChanged: (value) => _setSetting('unrepliedSlowdownAfter', value.round()),
            ),
            _sliderSetting(
              label: '冷却倍数',
              value: _doubleSetting('unrepliedCooldownMultiplier', 2).clamp(1, 8).toDouble(),
              min: 1,
              max: 8,
              divisions: 28,
              suffix: 'x',
              onChanged: (value) => _setSetting('unrepliedCooldownMultiplier', double.parse(value.toStringAsFixed(2))),
            ),
            AmitiaSwitchTile(
              title: '用户回复后恢复正常频率',
              value: _boolSetting('unrepliedRecoveryOnReply', true),
              onChanged: (value) => _setSetting('unrepliedRecoveryOnReply', value),
            ),
          ],
          SizedBox(height: AppSpacing.md),
          AmitiaButton(label: '保存全局参数', icon: Icons.save_outlined, isFullWidth: true, onPressed: _saveGlobalSettings),
        ],
      ),
    );
  }

  Widget _sliderSetting({
    required String label,
    required double value,
    required double min,
    required double max,
    required int divisions,
    required String suffix,
    required ValueChanged<double> onChanged,
  }) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('$label：${value % 1 == 0 ? value.toInt() : value.toStringAsFixed(2)}$suffix', style: AppTypography.label(context)),
          Slider(value: value, min: min, max: max, divisions: divisions, onChanged: onChanged),
        ],
      ),
    );
  }

  Widget _timeButton(String label, String value, VoidCallback onTap) {
    return InkWell(
      onTap: onTap,
      borderRadius: AppRadius.brSmall,
      child: Container(
        padding: EdgeInsets.all(AppSpacing.md),
        decoration: BoxDecoration(border: Border.all(color: context.borderPrimary), borderRadius: AppRadius.brSmall),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(label, style: AppTypography.caption(context)),
          const SizedBox(height: 2),
          Text(value, style: AppTypography.body(context)),
        ]),
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
    final ruleType = (rule['ruleType'] ?? 'custom').toString();
    final conversationId = (rule['conversationId'] ?? '').toString();
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
            Text('类型：${_ruleTypeLabel(ruleType)}${conversationId.isEmpty ? ' · 目标会话：自动选择' : ' · 目标会话：${_conversationLabel(conversationId)}'}', style: AppTypography.caption(context)),
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
                SizedBox(width: AppSpacing.sm),
                AmitiaButtonOutline(label: '立即触发', onPressed: id.isEmpty ? null : () => _triggerRule(id, name)),
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
    String channel = (existing?['channel'] ?? 'all').toString();
    String ruleType = (existing?['ruleType'] ?? 'custom').toString();
    String conversationId = (existing?['conversationId'] ?? '').toString();
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
              Text('规则类型', style: AppTypography.label(context)),
              DropdownButton<String>(
                value: const ['daily_greeting', 'sleep_reminder', 'study_checkin', 'work_break', 'custom', 'cron'].contains(ruleType) ? ruleType : 'custom',
                isExpanded: true,
                items: const ['daily_greeting', 'sleep_reminder', 'study_checkin', 'work_break', 'custom', 'cron']
                    .map((value) => DropdownMenuItem(value: value, child: Text(_ruleTypeLabel(value))))
                    .toList(),
                onChanged: (value) => setSheetState(() => ruleType = value ?? 'custom'),
              ),
              SizedBox(height: AppSpacing.md),
              Text('目标会话', style: AppTypography.label(context)),
              DropdownButton<String>(
                value: _conversations.any((item) => item.id == conversationId) ? conversationId : '',
                isExpanded: true,
                items: [
                  const DropdownMenuItem(value: '', child: Text('自动选择当前角色会话')),
                  ..._conversations.map((item) => DropdownMenuItem(value: item.id as String, child: Text(item.title.isEmpty ? item.id : item.title))),
                ],
                onChanged: (value) => setSheetState(() => conversationId = value ?? ''),
              ),
              SizedBox(height: AppSpacing.md),
              Text('发送渠道', style: AppTypography.label(context)),
              DropdownButton<String>(
                value: channel,
                isExpanded: true,
                items: const ['all', 'web', 'wechat', 'qq', 'web,wechat'].map((value) => DropdownMenuItem(value: value, child: Text(value))).toList(),
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
                    'conversationId': conversationId,
                    'ruleType': ruleType,
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
      final result = await ref.read(proactiveServiceProvider).testRule(id);
      if (!mounted) return;
      final preview = (result?['messageContent'] ?? result?['preview'] ?? result?['content'] ?? result?['message'] ?? '测试完成，未产生真实发送副作用').toString();
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: Text('测试：$name'),
          content: SelectableText(preview),
          actions: [TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('关闭'))],
        ),
      );
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('测试失败：$e')));
    }
  }

  Future<void> _triggerRule(String id, String name) async {
    try {
      await ref.read(proactiveServiceProvider).triggerRule(id);
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已真实触发规则：$name')));
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('触发失败：$e')));
    }
  }

  Future<void> _resetPresets() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('恢复系统预设'),
        content: const Text('这会调用后端预设恢复接口，为当前角色补回系统主动消息预设。继续吗？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('恢复')),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(proactiveServiceProvider).resetPresets(characterId: widget.characterId);
      await _load();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('系统预设已恢复')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('恢复预设失败：$e')));
    }
  }

  String _ruleTypeLabel(String type) {
    const labels = {
      'daily_greeting': '每日问候',
      'sleep_reminder': '休息提醒',
      'study_checkin': '学习提醒',
      'work_break': '工作间歇',
      'custom': '自定义',
      'cron': 'Cron',
    };
    return labels[type] ?? type;
  }

  String _conversationLabel(String id) {
    for (final item in _conversations) {
      if (item.id == id) return item.title.isEmpty ? id : item.title;
    }
    return id;
  }

  Widget _runtimeSummaryCard() {
    final schedulerRunning = _status['schedulerRunning'] == true;
    final enabledCount = _status['enabledRuleCount'] ?? _rules.where((rule) => rule['enabled'] == true || rule['enabled'] == 1).length;
    final totalCount = _status['totalRuleCount'] ?? _rules.length;
    final queued = _queue['pending'] ?? _queue['queued'] ?? _queue['total'] ?? 0;
    return AmitiaCard(
      child: Row(
        children: [
          Expanded(child: _summaryMetric('调度器', schedulerRunning ? '运行中' : '未运行')),
          Expanded(child: _summaryMetric('启用规则', '$enabledCount / $totalCount')),
          Expanded(child: _summaryMetric('队列', '$queued')),
        ],
      ),
    );
  }

  Widget _summaryMetric(String label, String value) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: AppTypography.caption(context)),
          const SizedBox(height: 3),
          Text(value, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
        ],
      );

  Widget _historyCard() {
    final items = _history.take(5).toList(growable: false);
    if (items.isEmpty) return AmitiaCard(child: Text('暂无运行历史', style: AppTypography.caption(context)));
    return AmitiaCard(
      child: Column(
        children: items.map((item) {
          final state = (item['state'] ?? item['status'] ?? 'unknown').toString();
          return ListTile(
            dense: true,
            contentPadding: EdgeInsets.zero,
            title: Text((item['title'] ?? item['triggerType'] ?? '主动消息').toString()),
            subtitle: Text('${item['createdAt'] ?? ''}${item['lastError']?.toString().isNotEmpty == true ? ' · ${item['lastError']}' : ''}'),
            trailing: AmitiaStatusBadge(label: state, type: state == 'success' || state == 'sent' ? BadgeType.success : BadgeType.neutral),
          );
        }).toList(growable: false),
      ),
    );
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
