import 'dart:convert';

import 'package:flutter/material.dart';

import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_misc.dart';

Future<Map<String, dynamic>?> showContinuityThreadEditor(
  BuildContext context, {
  Map<String, dynamic>? initial,
}) {
  return showModalBottomSheet<Map<String, dynamic>>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    builder: (_) => _ContinuityThreadEditor(initial: initial),
  );
}

Future<Map<String, dynamic>?> showContinuityWaitEditor(BuildContext context) {
  return showModalBottomSheet<Map<String, dynamic>>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    builder: (_) => const _ContinuityWaitEditor(),
  );
}

String continuityWaitTypeLabel(String value) {
  switch (value) {
    case 'user':
      return '等待用户';
    case 'time':
      return '等待时间';
    case 'device':
      return '等待设备';
    case 'approval':
      return '等待审批';
    case 'dependency':
      return '等待依赖';
    default:
      return '等待外部事件';
  }
}

class _ContinuityThreadEditor extends StatefulWidget {
  const _ContinuityThreadEditor({this.initial});

  final Map<String, dynamic>? initial;

  @override
  State<_ContinuityThreadEditor> createState() =>
      _ContinuityThreadEditorState();
}

class _ContinuityThreadEditorState extends State<_ContinuityThreadEditor> {
  late final TextEditingController _titleController;
  late final TextEditingController _goalController;
  late final TextEditingController _currentController;
  late final TextEditingController _nextController;
  String? _error;

  @override
  void initState() {
    super.initState();
    final initial = widget.initial ?? const <String, dynamic>{};
    _titleController = TextEditingController(
      text: (initial['title'] ?? '').toString(),
    );
    _goalController = TextEditingController(
      text: (initial['goal'] ?? '').toString(),
    );
    _currentController = TextEditingController(
      text: (initial['currentState'] ?? '').toString(),
    );
    _nextController = TextEditingController(
      text: (initial['nextAction'] ?? '').toString(),
    );
  }

  @override
  void dispose() {
    _titleController.dispose();
    _goalController.dispose();
    _currentController.dispose();
    _nextController.dispose();
    super.dispose();
  }

  void _submit() {
    final title = _titleController.text.trim();
    if (title.isEmpty) {
      setState(() => _error = '请输入标题');
      return;
    }
    Navigator.pop(context, {
      'title': title,
      'goal': _goalController.text.trim(),
      'currentState': _currentController.text.trim(),
      'nextAction': _nextController.text.trim(),
    });
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.lg,
        AppSpacing.pagePadding,
        AppSpacing.lg + MediaQuery.viewInsetsOf(context).bottom,
      ),
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              widget.initial == null ? '新建持续事项' : '编辑持续事项',
              style: Theme.of(context).textTheme.titleLarge,
            ),
            SizedBox(height: AppSpacing.lg),
            _LabeledField(
              label: '标题',
              child: AmitiaTextField(
                controller: _titleController,
                hintText: '输入持续事项标题',
                maxLength: 120,
                maxLines: 1,
              ),
            ),
            SizedBox(height: AppSpacing.md),
            _LabeledField(
              label: '目标',
              child: AmitiaTextField(
                controller: _goalController,
                hintText: '描述最终目标',
                maxLines: 3,
              ),
            ),
            SizedBox(height: AppSpacing.md),
            _LabeledField(
              label: '当前进度',
              child: AmitiaTextField(
                controller: _currentController,
                hintText: '记录当前完成情况',
                maxLines: 3,
              ),
            ),
            SizedBox(height: AppSpacing.md),
            _LabeledField(
              label: '下一步',
              child: AmitiaTextField(
                controller: _nextController,
                hintText: '记录下一步动作',
                maxLines: 2,
              ),
            ),
            if (_error != null) ...[
              SizedBox(height: AppSpacing.sm),
              Text(
                _error!,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ],
            SizedBox(height: AppSpacing.lg),
            AmitiaButton(
              label: widget.initial == null ? '创建' : '保存',
              isFullWidth: true,
              onPressed: _submit,
            ),
          ],
        ),
      ),
    );
  }
}

class _ContinuityWaitEditor extends StatefulWidget {
  const _ContinuityWaitEditor();

  @override
  State<_ContinuityWaitEditor> createState() => _ContinuityWaitEditorState();
}

class _ContinuityWaitEditorState extends State<_ContinuityWaitEditor> {
  final _descriptionController = TextEditingController();
  final _conditionController = TextEditingController(text: '{}');
  final _resumeHintController = TextEditingController();
  String _type = 'user';
  DateTime? _dueAt;
  bool _autoResume = false;
  String? _error;

  @override
  void dispose() {
    _descriptionController.dispose();
    _conditionController.dispose();
    _resumeHintController.dispose();
    super.dispose();
  }

  Future<void> _pickDueAt() async {
    final now = DateTime.now();
    final date = await showDatePicker(
      context: context,
      initialDate: _dueAt ?? now,
      firstDate: now,
      lastDate: now.add(const Duration(days: 3650)),
    );
    if (date == null || !mounted) return;
    final time = await showTimePicker(
      context: context,
      initialTime: TimeOfDay.fromDateTime(_dueAt ?? now),
    );
    if (time == null) return;
    setState(() {
      _dueAt = DateTime(
        date.year,
        date.month,
        date.day,
        time.hour,
        time.minute,
      );
    });
  }

  Map<String, dynamic>? _condition() {
    if (_type == 'user' || _type == 'time') return const <String, dynamic>{};
    try {
      final decoded = jsonDecode(_conditionController.text.trim());
      if (decoded is Map) return Map<String, dynamic>.from(decoded);
    } catch (_) {}
    return null;
  }

  void _submit() {
    final description = _descriptionController.text.trim();
    if (description.isEmpty) {
      setState(() => _error = '请输入等待说明');
      return;
    }
    if (_type == 'time' && _dueAt == null) {
      setState(() => _error = '请选择到期时间');
      return;
    }
    final condition = _condition();
    if (condition == null) {
      setState(() => _error = '匹配条件必须是 JSON 对象');
      return;
    }
    Navigator.pop(context, {
      'type': _type,
      'description': description,
      'condition': condition,
      'resumeHint': _resumeHintController.text.trim(),
      'dueAt': _dueAt?.toUtc().toIso8601String(),
      'autoResume': _autoResume,
    });
  }

  @override
  Widget build(BuildContext context) {
    final requiresCondition = _type != 'user' && _type != 'time';
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.lg,
        AppSpacing.pagePadding,
        AppSpacing.lg + MediaQuery.viewInsetsOf(context).bottom,
      ),
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text('添加等待条件', style: Theme.of(context).textTheme.titleLarge),
            SizedBox(height: AppSpacing.lg),
            _LabeledField(
              label: '类型',
              child: DropdownButtonFormField<String>(
                initialValue: _type,
                decoration: const InputDecoration(),
                items: const [
                  DropdownMenuItem(value: 'user', child: Text('等待用户')),
                  DropdownMenuItem(value: 'time', child: Text('等待时间')),
                  DropdownMenuItem(value: 'device', child: Text('等待设备')),
                  DropdownMenuItem(value: 'external', child: Text('等待外部事件')),
                  DropdownMenuItem(value: 'approval', child: Text('等待审批')),
                  DropdownMenuItem(value: 'dependency', child: Text('等待依赖')),
                ],
                onChanged: (value) {
                  if (value == null) return;
                  setState(() {
                    _type = value;
                    _autoResume = value != 'user';
                    _error = null;
                  });
                },
              ),
            ),
            SizedBox(height: AppSpacing.md),
            _LabeledField(
              label: '等待说明',
              child: AmitiaTextField(
                controller: _descriptionController,
                hintText: '说明正在等待什么条件',
                maxLines: 2,
              ),
            ),
            if (_type == 'time') ...[
              SizedBox(height: AppSpacing.md),
              AmitiaButton(
                label: _dueAt == null
                    ? '选择到期时间'
                    : '到期时间 ${_formatDateTime(_dueAt!)}',
                icon: Icons.schedule_outlined,
                isSecondary: true,
                outlined: true,
                isFullWidth: true,
                onPressed: _pickDueAt,
              ),
            ],
            if (requiresCondition) ...[
              SizedBox(height: AppSpacing.md),
              _LabeledField(
                label: '匹配条件',
                child: AmitiaTextField(
                  controller: _conditionController,
                  hintText: '例如 {"deviceId":"..."}',
                  maxLines: 4,
                  keyboardType: TextInputType.multiline,
                ),
              ),
            ],
            SizedBox(height: AppSpacing.md),
            _LabeledField(
              label: '恢复提示',
              child: AmitiaTextField(
                controller: _resumeHintController,
                hintText: '条件满足后如何继续',
                maxLines: 2,
              ),
            ),
            SizedBox(height: AppSpacing.sm),
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              title: const Text('条件满足后自动恢复'),
              value: _autoResume,
              onChanged: (value) => setState(() => _autoResume = value),
            ),
            if (_error != null) ...[
              SizedBox(height: AppSpacing.sm),
              Text(
                _error!,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ],
            SizedBox(height: AppSpacing.lg),
            AmitiaButton(label: '保存', isFullWidth: true, onPressed: _submit),
          ],
        ),
      ),
    );
  }
}

class _LabeledField extends StatelessWidget {
  const _LabeledField({required this.label, required this.child});

  final String label;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: AppTypography.label(context)),
        SizedBox(height: AppSpacing.xs),
        child,
      ],
    );
  }
}

String _formatDateTime(DateTime value) {
  String two(int number) => number.toString().padLeft(2, '0');
  return '${value.year}-${two(value.month)}-${two(value.day)} '
      '${two(value.hour)}:${two(value.minute)}';
}
