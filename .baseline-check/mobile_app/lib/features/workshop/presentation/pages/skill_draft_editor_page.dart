import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class SkillDraftEditorPage extends ConsumerStatefulWidget {
  const SkillDraftEditorPage({super.key, required this.draftId});

  final String draftId;

  @override
  ConsumerState<SkillDraftEditorPage> createState() => _SkillDraftEditorPageState();
}

class _SkillDraftEditorPageState extends ConsumerState<SkillDraftEditorPage> {
  Map<String, dynamic>? _session;
  Map<String, dynamic>? _revision;
  Map<String, dynamic>? _validation;
  List<Map<String, dynamic>> _tests = const [];
  bool _loading = true;
  bool _busy = false;
  String _testMode = 'dry_run';
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
      final svc = ref.read(extensionServiceProvider);
      final session = await svc.getWorkshopSession(widget.draftId);
      Map<String, dynamic>? revision;
      Map<String, dynamic>? validation;
      if (session != null) {
        final embedded = session['revision'];
        if (embedded is Map) {
          revision = Map<String, dynamic>.from(embedded);
        } else {
          final currentRevision =
              (session['currentRevision'] as num?)?.toInt() ?? 0;
          if (currentRevision > 0) {
            revision = await svc.getWorkshopRevision(
              widget.draftId,
              currentRevision,
            );
          }
        }
        final rawValidation = revision?['validation'];
        if (rawValidation is Map) {
          validation = Map<String, dynamic>.from(rawValidation);
        }
      }
      final tests = await svc.workshopTests(widget.draftId);
      if (!mounted) return;
      setState(() {
        _session = session;
        _revision = revision;
        _validation = validation;
        _tests = tests;
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error.toString();
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'Skill 制作',
        showBackButton: true,
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? _buildError()
              : RefreshIndicator(
                  onRefresh: _load,
                  child: ListView(
                    physics: const AlwaysScrollableScrollPhysics(),
                    padding: EdgeInsets.all(AppSpacing.pagePadding),
                    children: [
                      _buildSessionHeader(),
                      SizedBox(height: AppSpacing.md),
                      _buildActions(),
                      SizedBox(height: AppSpacing.md),
                      _buildRevisionCard(),
                      SizedBox(height: AppSpacing.md),
                      _buildValidationCard(),
                      SizedBox(height: AppSpacing.md),
                      _buildTestsCard(),
                    ],
                  ),
                ),
    );
  }

  Widget _buildError() {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: context.error),
            const SizedBox(height: 12),
            Text(_error ?? '加载失败', textAlign: TextAlign.center),
            const SizedBox(height: 16),
            AmitiaButton(label: '重试', onPressed: _load),
          ],
        ),
      ),
    );
  }

  Widget _buildSessionHeader() {
    final session = _session ?? const <String, dynamic>{};
    final requirement = (session['requirement'] ?? '').toString();
    final status = (session['status'] ?? 'draft').toString();
    final revision = (session['currentRevision'] as num?)?.toInt() ?? 0;
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  requirement.isEmpty ? '未提供需求描述' : requirement,
                  style: AppTypography.sectionTitle(context),
                ),
              ),
              AmitiaStatusBadge(
                label: _statusLabel(status),
                type: _statusBadge(status),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(
            revision > 0 ? '当前 Revision：$revision' : '尚未生成 Revision',
            style: AppTypography.caption(context),
          ),
          if ((session['installedSkillId'] ?? '').toString().isNotEmpty) ...[
            SizedBox(height: AppSpacing.xs),
            Text(
              '已安装 Skill：${session['installedSkillId']} ${session['installedVersion'] ?? ''}',
              style: AppTypography.caption(context),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildActions() {
    final status = (_session?['status'] ?? 'draft').toString();
    final currentRevision =
        (_session?['currentRevision'] as num?)?.toInt() ?? 0;
    final hasRevision = currentRevision > 0;
    final canGenerate = status == 'draft' ||
        status == 'generated' ||
        status == 'validation_failed' ||
        status == 'error';
    final canValidate = hasRevision &&
        (status == 'generated' ||
            status == 'validation_failed' ||
            status == 'validated' ||
            status == 'awaiting_permission_confirmation');
    final canTest = hasRevision &&
        (status == 'validated' ||
            status == 'awaiting_permission_confirmation' ||
            status == 'test_failed' ||
            status == 'test_passed');
    final canInstall = hasRevision && status == 'test_passed';

    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('真实 Workshop 流程', style: AppTypography.cardTitle(context)),
          SizedBox(height: AppSpacing.xs),
          Text(
            '生成 → 校验 → 确认测试权限 → 测试 → 确认生产权限 → 安装。',
            style: AppTypography.caption(context),
          ),
          SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              AmitiaButton(
                label: '生成 Revision',
                height: 38,
                isSecondary: true,
                onPressed: _busy || !canGenerate ? null : _generate,
              ),
              AmitiaButton(
                label: '校验',
                height: 38,
                isSecondary: true,
                onPressed: _busy || !canValidate ? null : _validate,
              ),
              AmitiaButton(
                label: '测试',
                height: 38,
                isSecondary: true,
                onPressed: _busy || !canTest ? null : _prepareAndTest,
              ),
              AmitiaButton(
                label: '安装',
                height: 38,
                onPressed: _busy || !canInstall ? null : _prepareAndInstall,
              ),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Text('测试模式', style: AppTypography.label(context)),
              const SizedBox(width: 12),
              Expanded(
                child: DropdownButtonFormField<String>(
                  value: _testMode,
                  decoration: const InputDecoration(isDense: true),
                  items: const [
                    DropdownMenuItem(value: 'dry_run', child: Text('Dry Run')),
                    DropdownMenuItem(value: 'mocked', child: Text('Mocked')),
                    DropdownMenuItem(
                      value: 'controlled_live',
                      child: Text('Controlled Live'),
                    ),
                  ],
                  onChanged: _busy
                      ? null
                      : (value) => setState(() => _testMode = value ?? 'dry_run'),
                ),
              ),
            ],
          ),
          if (_busy) ...[
            SizedBox(height: AppSpacing.md),
            const LinearProgressIndicator(),
          ],
        ],
      ),
    );
  }

  Widget _buildRevisionCard() {
    final revision = _revision;
    if (revision == null) {
      return AmitiaCard(
        child: Text('尚未生成 Revision。', style: AppTypography.body(context)),
      );
    }
    final draft = revision['normalizedDraft'] is Map
        ? Map<String, dynamic>.from(revision['normalizedDraft'] as Map)
        : revision['draft'] is Map
            ? Map<String, dynamic>.from(revision['draft'] as Map)
            : const <String, dynamic>{};
    final metadata = draft['metadata'] is Map
        ? Map<String, dynamic>.from(draft['metadata'] as Map)
        : const <String, dynamic>{};
    final plan = revision['plan'] is Map
        ? Map<String, dynamic>.from(revision['plan'] as Map)
        : const <String, dynamic>{};
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Revision ${(revision['revision'] as num?)?.toInt() ?? '-'}',
              style: AppTypography.cardTitle(context)),
          SizedBox(height: AppSpacing.sm),
          _kv('Skill', (metadata['name'] ?? metadata['id'] ?? '-').toString()),
          _kv('版本', (metadata['version'] ?? '-').toString()),
          _kv('模型',
              '${revision['modelProvider'] ?? '-'} / ${revision['modelName'] ?? '-'}'),
          if (plan.isNotEmpty) _jsonSection('生成计划', plan),
          _jsonSection('标准化草稿', draft),
        ],
      ),
    );
  }

  Widget _buildValidationCard() {
    final validation = _validation;
    if (validation == null) {
      return AmitiaCard(
        child: Text('尚未执行校验。', style: AppTypography.body(context)),
      );
    }
    final capabilities = validation['capabilities'] is Map
        ? Map<String, dynamic>.from(validation['capabilities'] as Map)
        : const <String, dynamic>{};
    final required = _stringList(capabilities['required']);
    final highRisk = _stringList(capabilities['highRisk']);
    final issues = validation['issues'] is List ? validation['issues'] as List : const [];
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text('校验结果', style: AppTypography.cardTitle(context)),
              ),
              AmitiaStatusBadge(
                label: validation['valid'] == true ? '通过' : '未通过',
                type: validation['valid'] == true
                    ? BadgeType.success
                    : BadgeType.error,
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          _kv('Workflow Checksum',
              (validation['workflowChecksum'] ?? '-').toString()),
          _kv('副作用', validation['hasSideEffects'] == true ? '有' : '无'),
          _kv('幂等', validation['idempotent'] == true ? '是' : '否'),
          _kv('必须 Capability', required.isEmpty ? '无' : required.join(', ')),
          _kv('高风险 Capability', highRisk.isEmpty ? '无' : highRisk.join(', ')),
          if (issues.isNotEmpty) _jsonSection('问题', {'items': issues}),
        ],
      ),
    );
  }

  Widget _buildTestsCard() {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('测试记录', style: AppTypography.cardTitle(context)),
          SizedBox(height: AppSpacing.sm),
          if (_tests.isEmpty)
            Text('暂无测试记录', style: AppTypography.caption(context))
          else
            ..._tests.take(10).map((test) {
              final status = (test['status'] ?? '').toString();
              return Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      status == 'passed' ? Icons.check_circle : Icons.error_outline,
                      size: 18,
                      color: status == 'passed' ? context.success : context.error,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        '${test['testRunId'] ?? '-'} · $status · ${test['durationMs'] ?? 0}ms',
                        style: AppTypography.caption(context),
                      ),
                    ),
                  ],
                ),
              );
            }),
        ],
      ),
    );
  }

  Future<void> _generate() async {
    await _runAction('生成', () async {
      final revision = await ref
          .read(extensionServiceProvider)
          .generateWorkshopDraft(widget.draftId);
      if (revision == null) throw StateError('后端未返回 Revision');
    });
  }

  Future<void> _validate() async {
    final revision = (_session?['currentRevision'] as num?)?.toInt() ?? 0;
    if (revision < 1) return;
    await _runAction('校验', () async {
      final validation = await ref
          .read(extensionServiceProvider)
          .validateWorkshopRevision(widget.draftId, revision);
      if (validation == null) throw StateError('后端未返回校验结果');
    });
  }

  Future<void> _prepareAndTest() async {
    final revision = (_session?['currentRevision'] as num?)?.toInt() ?? 0;
    if (revision < 1) return;
    final validation = _validation ??
        await ref
            .read(extensionServiceProvider)
            .validateWorkshopRevision(widget.draftId, revision);
    if (validation == null || validation['valid'] != true) {
      if (mounted) amitiaSnackBar(context, '当前 Revision 尚未通过校验');
      await _load();
      return;
    }
    final capabilities = validation['capabilities'] is Map
        ? Map<String, dynamic>.from(validation['capabilities'] as Map)
        : const <String, dynamic>{};
    final required = _stringList(capabilities['required']);
    final highRisk = _stringList(capabilities['highRisk']);
    final confirmed = await _confirmCapabilities(
      title: '确认测试权限',
      capabilities: required,
      highRisk: highRisk,
      production: false,
    );
    if (confirmed != true) return;
    if (_testMode == 'controlled_live') {
      final liveConfirmed = await showAmitiaConfirmDialog(
        context,
        title: 'Controlled Live',
        message: '该模式可能产生真实外部副作用。确认继续真实受控测试吗？',
        confirmLabel: '确认执行',
        isDestructive: true,
      );
      if (liveConfirmed != true) return;
    }
    await _runAction('测试', () async {
      final svc = ref.read(extensionServiceProvider);
      await svc.confirmWorkshopPermissions(
        id: widget.draftId,
        revision: revision,
        workflowChecksum: (validation['workflowChecksum'] ?? '').toString(),
        capabilities: required,
        confirmedHighRisk: highRisk,
        production: false,
      );
      final report = await svc.testWorkshopRevision(
        id: widget.draftId,
        revision: revision,
        mode: _testMode,
        controlledLiveConfirmed: _testMode == 'controlled_live',
      );
      if (report == null) throw StateError('测试没有返回报告');
      if ((report['status'] ?? '').toString() != 'passed') {
        throw StateError('测试未通过：${report['error'] ?? report['status']}');
      }
    });
  }

  Future<void> _prepareAndInstall() async {
    final revision = (_session?['currentRevision'] as num?)?.toInt() ?? 0;
    if (revision < 1 || _validation == null) return;
    final validation = _validation!;
    final capabilities = validation['capabilities'] is Map
        ? Map<String, dynamic>.from(validation['capabilities'] as Map)
        : const <String, dynamic>{};
    final required = _stringList(capabilities['required']);
    final highRisk = _stringList(capabilities['highRisk']);
    final confirmed = await _confirmCapabilities(
      title: '确认生产权限并安装',
      capabilities: required,
      highRisk: highRisk,
      production: true,
    );
    if (confirmed != true) return;
    await _runAction('安装', () async {
      final svc = ref.read(extensionServiceProvider);
      await svc.confirmWorkshopPermissions(
        id: widget.draftId,
        revision: revision,
        workflowChecksum: (validation['workflowChecksum'] ?? '').toString(),
        capabilities: required,
        confirmedHighRisk: highRisk,
        production: true,
      );
      final installed = await svc.installWorkshopRevision(
        widget.draftId,
        revision,
      );
      if (installed == null) throw StateError('安装没有返回结果');
    });
  }

  Future<bool?> _confirmCapabilities({
    required String title,
    required List<String> capabilities,
    required List<String> highRisk,
    required bool production,
  }) {
    final text = capabilities.isEmpty
        ? '当前 Revision 不需要额外 Capability。\n\n确认继续${production ? '安装' : '测试'}吗？'
        : '必须 Capability：\n${capabilities.map((e) => '• $e').join('\n')}\n\n'
            '${highRisk.isEmpty ? '' : '高风险 Capability：\n${highRisk.map((e) => '• $e').join('\n')}\n\n'}'
            '确认这些权限与当前 Workflow Checksum 绑定并继续吗？';
    return showAmitiaConfirmDialog(
      context,
      title: title,
      message: text,
      confirmLabel: production ? '确认并安装' : '确认并测试',
      isDestructive: highRisk.isNotEmpty,
    );
  }

  Future<void> _runAction(
    String label,
    Future<void> Function() action,
  ) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await action();
      await _load();
      if (mounted) amitiaSnackBar(context, '$label完成');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '$label失败：$error');
      await _load();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Widget _kv(String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 118,
            child: Text(label, style: AppTypography.caption(context)),
          ),
          Expanded(
            child: SelectableText(
              value,
              style: AppTypography.caption(context)
                  .copyWith(color: context.textPrimary),
            ),
          ),
        ],
      ),
    );
  }

  Widget _jsonSection(String title, Map<String, dynamic> value) {
    const encoder = JsonEncoder.withIndent('  ');
    return Padding(
      padding: EdgeInsets.only(top: AppSpacing.sm),
      child: ExpansionTile(
        tilePadding: EdgeInsets.zero,
        childrenPadding: EdgeInsets.zero,
        title: Text(title, style: AppTypography.label(context)),
        children: [
          Container(
            width: double.infinity,
            padding: EdgeInsets.all(AppSpacing.sm),
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brSmall,
            ),
            child: SelectableText(
              encoder.convert(value),
              style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
            ),
          ),
        ],
      ),
    );
  }

  List<String> _stringList(dynamic value) {
    if (value is! List) return const <String>[];
    return value.map((e) => e.toString()).where((e) => e.isNotEmpty).toList();
  }

  String _statusLabel(String status) {
    const labels = <String, String>{
      'draft': '草稿',
      'generating': '生成中',
      'generated': '已生成',
      'validating': '校验中',
      'validation_failed': '校验失败',
      'validated': '已校验',
      'awaiting_permission_confirmation': '待确认权限',
      'testing': '测试中',
      'test_failed': '测试失败',
      'test_passed': '测试通过',
      'installing': '安装中',
      'installed': '已安装',
      'enabled': '已启用',
      'disabled': '已停用',
      'archived': '已归档',
      'error': '错误',
    };
    return labels[status] ?? status;
  }

  BadgeType _statusBadge(String status) {
    if (status == 'installed' ||
        status == 'enabled' ||
        status == 'test_passed' ||
        status == 'validated') {
      return BadgeType.success;
    }
    if (status.contains('failed') || status == 'error') {
      return BadgeType.error;
    }
    if (status == 'generating' ||
        status == 'validating' ||
        status == 'testing' ||
        status == 'installing') {
      return BadgeType.accent;
    }
    return BadgeType.neutral;
  }
}
