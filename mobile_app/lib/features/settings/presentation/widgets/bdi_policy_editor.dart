import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';

const _candidateFallback = <String, String>{
  'chat_reply': '聊天回复',
  'proactive_greet': '主动问候',
  'ask_clarify': '请求澄清',
  'offer_support': '提供支持',
  'set_boundary': '设立边界',
  'express_emotion': '表达情绪',
  'wait_observe': '等待观察',
  'tool_search': '工具搜索',
};

const _dimensionFallback = <String, String>{
  'chat_reply': '聊天回复',
  'proactive_greet': '主动问候',
  'ask_clarify': '请求澄清',
  'offer_support': '提供支持',
  'set_boundary': '设立边界',
  'express_emotion': '表达情绪',
  'wait_observe': '等待观察',
  'tool_search': '工具搜索',
  'reply': '正常回复类',
  'proactive_check': '主动问候类',
  'delay': '等待观察类',
  'repair': '关系修复类',
};

const _copingFallback = <String, String>{
  'problem_solving': '问题解决',
  'reappraisal': '认知重构',
  'seek_support': '寻求支持',
  'acceptance': '接纳',
  'distraction': '转移注意',
  'suppression': '情绪抑制',
  'avoidance': '回避',
};

const _displayModeFallback = <String, String>{
  'show': '如实表露',
  'suppress': '抑制表露',
  'reframe': '重构表达',
};

const _legacyCoping = <String, String>{
  'active': 'problem_solving',
  'support': 'seek_support',
  'reframe': 'reappraisal',
  'avoid': 'avoidance',
  'suppress': 'suppression',
  'accept': 'acceptance',
};

Future<bool?> showBdiPolicyEditor(BuildContext context) {
  return showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (_) => const _BdiPolicyDialog(),
  );
}

class _BdiPolicyDialog extends ConsumerStatefulWidget {
  const _BdiPolicyDialog();

  @override
  ConsumerState<_BdiPolicyDialog> createState() => _BdiPolicyDialogState();
}

class _BdiPolicyDialogState extends ConsumerState<_BdiPolicyDialog> {
  final _copingReason = TextEditingController();
  final _overrideReason = TextEditingController();
  final List<Map<String, dynamic>> _rules = [];
  final List<Map<String, dynamic>> _preferences = [];
  final Set<String> _alternatives = {};

  Map<String, String> _candidates = Map.of(_candidateFallback);
  Map<String, String> _dimensions = Map.of(_dimensionFallback);
  Map<String, String> _coping = Map.of(_copingFallback);
  Map<String, String> _displayModes = Map.of(_displayModeFallback);

  String _copingSelected = 'problem_solving';
  String _displayMode = 'show';
  double _internalIntensity = 5;
  double _displayIntensity = 5;
  int _revision = 0;
  bool _loading = true;
  bool _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _copingReason.dispose();
    _overrideReason.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final service = ref.read(safetyServiceProvider);
    try {
      final results = await Future.wait([
        service.bdiConfig(),
        service.bdiOptions(),
      ]);
      final config = results[0] ?? <String, dynamic>{};
      final options = results[1] ?? <String, dynamic>{};
      _applyOptions(options);
      _applyConfig(config);
      if (mounted) setState(() => _loading = false);
    } catch (e) {
      if (mounted) {
        setState(() {
          _loading = false;
          _error = '$e';
        });
      }
    }
  }

  void _applyOptions(Map<String, dynamic> options) {
    final candidates = options['candidates'];
    if (candidates is List && candidates.isNotEmpty) {
      _candidates = {
        for (final item in candidates)
          if (item is Map && item['id'] != null)
            item['id'].toString(): (item['label'] ?? item['id']).toString(),
      };
    }
    final dimensions = options['dimensions'];
    if (dimensions is List && dimensions.isNotEmpty) {
      _dimensions = {
        for (final item in dimensions)
          if (item is Map && item['id'] != null)
            item['id'].toString(): (item['label'] ?? item['id']).toString(),
      };
    }
    final coping = options['copingStrategies'];
    if (coping is List && coping.isNotEmpty) {
      _coping = {
        for (final item in coping)
          if (item is Map && item['id'] != null)
            item['id'].toString(): (item['label'] ?? item['id']).toString(),
      };
    }
    final modes = options['displayModes'];
    if (modes is List && modes.isNotEmpty) {
      _displayModes = {
        for (final item in modes)
          if (item is Map && item['id'] != null)
            item['id'].toString(): (item['label'] ?? item['id']).toString(),
      };
    }
  }

  void _applyConfig(Map<String, dynamic> config) {
    _revision = int.tryParse('${config['revision'] ?? 0}') ?? 0;
    final rules = config['hardConstraints'];
    if (rules is List) {
      for (final rule in rules) {
        if (rule is! Map) continue;
        _rules.add({
          'ruleId': (rule['ruleId'] ?? '').toString(),
          'candidateKey': (rule['candidateKey'] ?? '').toString(),
          'reason': (rule['reason'] ?? '').toString(),
          'severity': (rule['severity'] ?? 'block') == 'override'
              ? 'override'
              : 'block',
        });
      }
    }
    final preferences = config['softPreferences'];
    if (preferences is List) {
      for (final preference in preferences) {
        if (preference is! Map) continue;
        _preferences.add({
          'dimension': (preference['dimension'] ?? '').toString(),
          'rawScore': _toDouble(preference['rawScore'], 5),
          'normalizedWeight': _toDouble(
            preference['normalizedWeight'],
            0,
          ),
        });
      }
    }
    final coping = config['copingStrategy'];
    if (coping is Map) {
      final selected = _normalizeCoping('${coping['selected'] ?? ''}');
      _copingSelected = _coping.containsKey(selected)
          ? selected
          : 'problem_solving';
      final alternatives = coping['alternatives'];
      if (alternatives is List) {
        for (final item in alternatives) {
          final value = _normalizeCoping('$item');
          if (_coping.containsKey(value)) _alternatives.add(value);
        }
      }
      _copingReason.text = (coping['selectionReason'] ?? '').toString();
    }
    final emotion = config['emotionExpression'];
    if (emotion is Map) {
      final mode = (emotion['displayMode'] ?? 'show').toString();
      _displayMode = _displayModes.containsKey(mode) ? mode : 'show';
      _internalIntensity = _toDouble(emotion['internalIntensity'], 5)
          .clamp(0, 10);
      _displayIntensity = _toDouble(emotion['displayIntensity'], 5).clamp(
        0,
        10,
      );
      _overrideReason.text = (emotion['overrideReason'] ?? '').toString();
    }
  }

  double _toDouble(Object? value, double fallback) {
    if (value is num) return value.toDouble();
    return double.tryParse('$value') ?? fallback;
  }

  String _normalizeCoping(String value) {
    final trimmed = value.trim();
    return _legacyCoping[trimmed] ?? trimmed;
  }

  void _addRule() {
    setState(() {
      _rules.add({
        'ruleId': 'rule-${DateTime.now().millisecondsSinceEpoch % 1000000}',
        'candidateKey': 'proactive_greet',
        'reason': '',
        'severity': 'block',
      });
    });
  }

  void _addPreference() {
    setState(() {
      _preferences.add({
        'dimension': 'chat_reply',
        'rawScore': 5.0,
        'normalizedWeight': 0.2,
      });
    });
  }

  Future<void> _save() async {
    for (final rule in _rules) {
      if ('${rule['ruleId']}'.trim().isEmpty) {
        _snack('存在缺少规则 ID 的约束');
        return;
      }
      if ('${rule['candidateKey']}'.isEmpty) {
        _snack('存在未选择候选行为的约束');
        return;
      }
      if (rule['severity'] == 'block' && '${rule['reason']}'.trim().isEmpty) {
        _snack('阻断规则必须填写原因');
        return;
      }
    }
    for (final preference in _preferences) {
      if ('${preference['dimension']}'.isEmpty) {
        _snack('存在未选择维度的软偏好');
        return;
      }
    }
    setState(() => _saving = true);
    try {
      final updated = await ref.read(safetyServiceProvider).updateBdiConfig({
        'hardConstraints': _rules,
        'softPreferences': _preferences
            .map(
              (item) => {
                'dimension': item['dimension'],
                'rawScore': item['rawScore'],
                'normalizedWeight': item['normalizedWeight'],
              },
            )
            .toList(),
        'copingStrategy': {
          'selected': _copingSelected,
          'alternatives': _alternatives.toList(),
          'selectionReason': _copingReason.text.trim(),
        },
        'emotionExpression': {
          'displayMode': _displayMode,
          'internalIntensity': _internalIntensity.round(),
          'displayIntensity': _displayIntensity.round(),
          'overrideReason': _overrideReason.text.trim(),
        },
        'expectedRevision': _revision,
      });
      _revision = int.tryParse('${updated?['revision'] ?? _revision + 1}') ??
          _revision + 1;
      if (mounted) Navigator.of(context).pop(true);
    } catch (e) {
      _snack('保存失败：$e');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  void _snack(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      insetPadding: EdgeInsets.symmetric(horizontal: 12, vertical: 24),
      child: SizedBox(
        width: double.maxFinite,
        height: double.maxFinite,
        child: Column(
          children: [
            Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.lg,
                AppSpacing.md,
                AppSpacing.sm,
                0,
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      'BDI 安全策略',
                      style: AppTypography.sectionTitle(context),
                    ),
                  ),
                  IconButton(
                    onPressed: _saving
                        ? null
                        : () => Navigator.of(context).pop(false),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
            ),
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _error != null
                  ? Center(
                      child: Padding(
                        padding: EdgeInsets.all(AppSpacing.lg),
                        child: Text(
                          '读取失败：$_error',
                          style: AppTypography.caption(
                            context,
                          ).copyWith(color: context.error),
                        ),
                      ),
                    )
                  : ListView(
                      padding: EdgeInsets.fromLTRB(
                        AppSpacing.lg,
                        AppSpacing.sm,
                        AppSpacing.lg,
                        AppSpacing.lg,
                      ),
                      children: [
                        _hint('配置直接进入候选仲裁与表达计划，保存后下一轮对话生效。'),
                        _section('应对策略'),
                        DropdownButtonFormField<String>(
                          initialValue: _copingSelected,
                          decoration: const InputDecoration(
                            labelText: '默认策略',
                            isDense: true,
                          ),
                          items: _coping.entries
                              .map(
                                (entry) => DropdownMenuItem(
                                  value: entry.key,
                                  child: Text(entry.value),
                                ),
                              )
                              .toList(),
                          onChanged: (value) {
                            if (value == null) return;
                            setState(() => _copingSelected = value);
                          },
                        ),
                        SizedBox(height: AppSpacing.sm),
                        Wrap(
                          spacing: 8,
                          runSpacing: 4,
                          children: _coping.entries
                              .where((entry) => entry.key != _copingSelected)
                              .map(
                                (entry) => FilterChip(
                                  label: Text(entry.value),
                                  selected: _alternatives.contains(entry.key),
                                  onSelected: (selected) {
                                    setState(() {
                                      if (selected) {
                                        _alternatives.add(entry.key);
                                      } else {
                                        _alternatives.remove(entry.key);
                                      }
                                    });
                                  },
                                ),
                              )
                              .toList(),
                        ),
                        SizedBox(height: AppSpacing.sm),
                        TextField(
                          controller: _copingReason,
                          decoration: const InputDecoration(
                            labelText: '选择理由（可选）',
                            isDense: true,
                          ),
                          maxLength: 200,
                        ),
                        _section('情绪表达'),
                        DropdownButtonFormField<String>(
                          initialValue: _displayMode,
                          decoration: const InputDecoration(
                            labelText: '表达模式',
                            isDense: true,
                          ),
                          items: _displayModes.entries
                              .map(
                                (entry) => DropdownMenuItem(
                                  value: entry.key,
                                  child: Text(entry.value),
                                ),
                              )
                              .toList(),
                          onChanged: (value) {
                            if (value == null) return;
                            setState(() => _displayMode = value);
                          },
                        ),
                        _slider(
                          label: '内在感受强度',
                          value: _internalIntensity,
                          min: 0,
                          max: 10,
                          divisions: 20,
                          onChanged: (value) =>
                              setState(() => _internalIntensity = value),
                        ),
                        _slider(
                          label: '外在表达上限',
                          value: _displayIntensity,
                          min: 0,
                          max: 10,
                          divisions: 20,
                          onChanged: (value) =>
                              setState(() => _displayIntensity = value),
                        ),
                        TextField(
                          controller: _overrideReason,
                          decoration: const InputDecoration(
                            labelText: '覆盖理由（可选）',
                            isDense: true,
                          ),
                          maxLength: 200,
                        ),
                        _section(
                          '硬约束',
                          action: TextButton(
                            onPressed: _addRule,
                            child: const Text('添加'),
                          ),
                        ),
                        if (_rules.isEmpty)
                          _hint('暂无约束。阻断会移除候选，优先会保留候选并排到最前。'),
                        for (var index = 0; index < _rules.length; index++)
                          _ruleCard(index),
                        _section(
                          '软偏好',
                          action: TextButton(
                            onPressed: _addPreference,
                            child: const Text('添加'),
                          ),
                        ),
                        if (_preferences.isEmpty)
                          _hint('暂无软偏好。偏好只影响候选排序，不会阻断。'),
                        for (var index = 0;
                            index < _preferences.length;
                            index++)
                          _preferenceCard(index),
                      ],
                    ),
            ),
            SafeArea(
              top: false,
              child: Padding(
                padding: EdgeInsets.all(AppSpacing.lg),
                child: Row(
                  children: [
                    Expanded(
                      child: OutlinedButton(
                        onPressed: _saving
                            ? null
                            : () => Navigator.of(context).pop(false),
                        child: const Text('取消'),
                      ),
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: FilledButton(
                        onPressed: _loading || _saving ? null : _save,
                        child: Text(_saving ? '保存中...' : '保存'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _section(String title, {Widget? action}) => Padding(
    padding: EdgeInsets.only(top: AppSpacing.md, bottom: AppSpacing.xs),
    child: Row(
      children: [
        Expanded(
          child: Text(title, style: AppTypography.body(context)),
        ),
        if (action != null) action,
      ],
    ),
  );

  Widget _hint(String text) => Padding(
    padding: EdgeInsets.only(bottom: AppSpacing.sm),
    child: Text(text, style: AppTypography.caption(context)),
  );

  Widget _slider({
    required String label,
    required double value,
    required double min,
    required double max,
    required int divisions,
    required ValueChanged<double> onChanged,
  }) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Row(
        children: [
          Expanded(
            child: Text(label, style: AppTypography.caption(context)),
          ),
          Text(
            value.toStringAsFixed(1),
            style: AppTypography.caption(context),
          ),
        ],
      ),
      Slider(
        value: value,
        min: min,
        max: max,
        divisions: divisions,
        label: value.toStringAsFixed(1),
        onChanged: onChanged,
      ),
    ],
  );

  Widget _ruleCard(int index) {
    final rule = _rules[index];
    return Card(
      margin: EdgeInsets.only(bottom: AppSpacing.sm),
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.sm),
        child: Column(
          children: [
            Row(
              children: [
                Expanded(
                  child: DropdownButtonFormField<String>(
                    initialValue: _candidates.containsKey(rule['candidateKey'])
                        ? rule['candidateKey'] as String
                        : 'chat_reply',
                    decoration: const InputDecoration(
                      labelText: '候选行为',
                      isDense: true,
                    ),
                    items: _candidates.entries
                        .map(
                          (entry) => DropdownMenuItem(
                            value: entry.key,
                            child: Text(entry.value),
                          ),
                        )
                        .toList(),
                    onChanged: (value) {
                      if (value == null) return;
                      setState(() => rule['candidateKey'] = value);
                    },
                  ),
                ),
                IconButton(
                  onPressed: () => setState(() => _rules.removeAt(index)),
                  icon: const Icon(Icons.delete_outline),
                ),
              ],
            ),
            SizedBox(height: AppSpacing.xs),
            TextFormField(
              initialValue: '${rule['ruleId']}',
              decoration: const InputDecoration(
                labelText: '规则 ID',
                isDense: true,
              ),
              onChanged: (value) => rule['ruleId'] = value.trim(),
            ),
            SizedBox(height: AppSpacing.xs),
            TextFormField(
              initialValue: '${rule['reason']}',
              decoration: const InputDecoration(
                labelText: '原因 / 禁止话题',
                isDense: true,
              ),
              onChanged: (value) => rule['reason'] = value.trim(),
            ),
            SizedBox(height: AppSpacing.xs),
            DropdownButtonFormField<String>(
              initialValue: rule['severity'] as String,
              decoration: const InputDecoration(
                labelText: '生效方式',
                isDense: true,
              ),
              items: const [
                DropdownMenuItem(value: 'block', child: Text('阻断该行为')),
                DropdownMenuItem(value: 'override', child: Text('保留但优先')),
              ],
              onChanged: (value) {
                if (value == null) return;
                setState(() => rule['severity'] = value);
              },
            ),
          ],
        ),
      ),
    );
  }

  Widget _preferenceCard(int index) {
    final preference = _preferences[index];
    final raw = _toDouble(preference['rawScore'], 5).clamp(0, 10).toDouble();
    final weight = _toDouble(preference['normalizedWeight'], 0).clamp(-1, 1).toDouble();
    final contribution = weight == 0
        ? (raw - 5) / 5
        : weight * ((raw - 5) / 5);
    return Card(
      margin: EdgeInsets.only(bottom: AppSpacing.sm),
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.sm),
        child: Column(
          children: [
            Row(
              children: [
                Expanded(
                  child: DropdownButtonFormField<String>(
                    initialValue: _dimensions.containsKey(
                      preference['dimension'],
                    )
                        ? preference['dimension'] as String
                        : 'chat_reply',
                    decoration: const InputDecoration(
                      labelText: '维度',
                      isDense: true,
                    ),
                    items: _dimensions.entries
                        .map(
                          (entry) => DropdownMenuItem(
                            value: entry.key,
                            child: Text(entry.value),
                          ),
                        )
                        .toList(),
                    onChanged: (value) {
                      if (value == null) return;
                      setState(() => preference['dimension'] = value);
                    },
                  ),
                ),
                IconButton(
                  onPressed: () =>
                      setState(() => _preferences.removeAt(index)),
                  icon: const Icon(Icons.delete_outline),
                ),
              ],
            ),
            _slider(
              label: '原始分',
              value: raw,
              min: 0,
              max: 10,
              divisions: 10,
              onChanged: (value) =>
                  setState(() => preference['rawScore'] = value),
            ),
            _slider(
              label: '权重',
              value: weight,
              min: -1,
              max: 1,
              divisions: 20,
              onChanged: (value) => setState(() =>
                  preference['normalizedWeight'] = double.parse(
                    value.toStringAsFixed(2),
                  )),
            ),
            Align(
              alignment: Alignment.centerLeft,
              child: Text(
                '贡献值 ${contribution.toStringAsFixed(2)}',
                style: AppTypography.caption(context),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
