import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/continuity.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../widgets/continuity_editors.dart';

class ContinuityDetailPage extends ConsumerStatefulWidget {
  const ContinuityDetailPage({super.key, required this.threadId});

  final String threadId;

  @override
  ConsumerState<ContinuityDetailPage> createState() =>
      _ContinuityDetailPageState();
}

class _ContinuityDetailPageState extends ConsumerState<ContinuityDetailPage> {
  ContinuityDetailDto? _detail;
  bool _loading = true;
  bool _saving = false;
  String? _error;
  Timer? _refreshTimer;

  @override
  void initState() {
    super.initState();
    _load();
    _refreshTimer = Timer.periodic(const Duration(seconds: 5), (_) {
      if (mounted && !_saving) unawaited(_load(showLoading: false));
    });
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    super.dispose();
  }

  Future<void> _load({bool showLoading = true}) async {
    if (showLoading && mounted) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final detail = await ref
          .read(continuityServiceProvider)
          .get(widget.threadId);
      if (!mounted) return;
      setState(() {
        _detail = detail;
        _loading = false;
        _error = null;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error.toString();
        _loading = false;
      });
    }
  }

  Future<void> _setStatus(String status) async {
    final detail = _detail;
    if (detail == null) return;
    if (status == 'completed') {
      final confirmed = await showAmitiaConfirmDialog(
        context,
        title: '完成持续事项',
        message: '完成后将关闭所有未满足的等待条件。',
        confirmLabel: '完成',
      );
      if (confirmed != true) return;
    }
    _saving = true;
    try {
      await ref.read(continuityServiceProvider).update(detail.thread.id, {
        'status': status,
      });
      await _load(showLoading: false);
    } catch (error) {
      if (mounted) amitiaSnackBar(context, error.toString());
    } finally {
      _saving = false;
    }
  }

  Future<void> _edit() async {
    final thread = _detail?.thread;
    if (thread == null) return;
    final input = await showContinuityThreadEditor(
      context,
      initial: {
        'title': thread.title,
        'goal': thread.goal,
        'currentState': thread.currentState,
        'nextAction': thread.nextAction,
      },
    );
    if (input == null) return;
    _saving = true;
    try {
      await ref.read(continuityServiceProvider).update(thread.id, input);
      await _load(showLoading: false);
    } catch (error) {
      if (mounted) amitiaSnackBar(context, error.toString());
    } finally {
      _saving = false;
    }
  }

  Future<void> _addWait() async {
    final thread = _detail?.thread;
    if (thread == null) return;
    final input = await showContinuityWaitEditor(context);
    if (input == null) return;
    _saving = true;
    try {
      await ref.read(continuityServiceProvider).createWait(thread.id, input);
      await _load(showLoading: false);
    } catch (error) {
      if (mounted) amitiaSnackBar(context, error.toString());
    } finally {
      _saving = false;
    }
  }

  Future<void> _resolveWait(ContinuityWaitDto wait) async {
    final thread = _detail?.thread;
    if (thread == null) return;
    _saving = true;
    try {
      await ref
          .read(continuityServiceProvider)
          .resolveWait(thread.id, wait.id, resume: wait.autoResume);
      await _load(showLoading: false);
    } catch (error) {
      if (mounted) amitiaSnackBar(context, error.toString());
    } finally {
      _saving = false;
    }
  }

  Future<void> _cancelWait(ContinuityWaitDto wait) async {
    final thread = _detail?.thread;
    if (thread == null) return;
    final confirmed = await showAmitiaConfirmDialog(
      context,
      title: '取消等待条件',
      message: '取消后不会触发自动恢复。',
      confirmLabel: '取消条件',
      isDestructive: true,
    );
    if (confirmed != true) return;
    _saving = true;
    try {
      await ref.read(continuityServiceProvider).cancelWait(thread.id, wait.id);
      await _load(showLoading: false);
    } catch (error) {
      if (mounted) amitiaSnackBar(context, error.toString());
    } finally {
      _saving = false;
    }
  }

  @override
  Widget build(BuildContext context) {
    final detail = _detail;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '持续事项',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新',
            onPressed: _load,
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _loading
            ? const AmitiaLoadingState(message: '加载详情…')
            : _error != null
            ? AmitiaErrorState(message: _error!, onRetry: _load)
            : detail == null
            ? const SizedBox.shrink()
            : RefreshIndicator(
                onRefresh: _load,
                child: ListView(
                  padding: EdgeInsets.fromLTRB(
                    AppSpacing.pagePadding,
                    AppSpacing.sm,
                    AppSpacing.pagePadding,
                    AppSpacing.xxl,
                  ),
                  children: [
                    _buildSummary(detail.thread),
                    SizedBox(height: AppSpacing.lg),
                    _buildWaits(detail),
                    SizedBox(height: AppSpacing.lg),
                    _buildEvents(detail.events),
                  ],
                ),
              ),
      ),
    );
  }

  Widget _buildSummary(ContinuityThreadDto thread) {
    final status = _statusMeta(thread.status);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                thread.title,
                style: AppTypography.pageTitle(context),
              ),
            ),
            AmitiaStatusBadge(label: status.label, type: status.type),
          ],
        ),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _detailRow('目标', thread.goal),
              _detailRow('当前进度', thread.currentState),
              _detailRow('下一步', thread.nextAction),
              _detailRow('摘要', thread.summary),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.md),
        Wrap(
          spacing: AppSpacing.sm,
          runSpacing: AppSpacing.sm,
          children: [
            if (!thread.terminal)
              AmitiaButton(
                label: thread.paused ? '恢复' : '暂停',
                icon: thread.paused
                    ? Icons.play_arrow_outlined
                    : Icons.pause_outlined,
                isSecondary: true,
                outlined: true,
                onPressed: _saving
                    ? null
                    : () => _setStatus(thread.paused ? 'active' : 'paused'),
              ),
            if (!thread.terminal)
              AmitiaButton(
                label: '完成',
                icon: Icons.check_circle_outline,
                isSecondary: true,
                outlined: true,
                onPressed: _saving ? null : () => _setStatus('completed'),
              ),
            AmitiaButton(
              label: '编辑',
              icon: Icons.edit_outlined,
              isSecondary: true,
              outlined: true,
              onPressed: _saving ? null : _edit,
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildWaits(ContinuityDetailDto detail) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Text('等待条件', style: AppTypography.sectionTitle(context)),
            ),
            AmitiaButton(
              label: '添加',
              icon: Icons.add,
              isSecondary: true,
              outlined: true,
              height: 38,
              onPressed: detail.thread.terminal || _saving ? null : _addWait,
            ),
          ],
        ),
        SizedBox(height: AppSpacing.sm),
        if (detail.waits.isEmpty)
          AmitiaCard(
            child: Text('暂无等待条件', style: AppTypography.caption(context)),
          )
        else
          ...detail.waits.map(
            (wait) => Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.sm),
              child: _buildWaitCard(wait),
            ),
          ),
      ],
    );
  }

  Widget _buildWaitCard(ContinuityWaitDto wait) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  continuityWaitTypeLabel(wait.waitType),
                  style: AppTypography.cardTitle(context),
                ),
              ),
              AmitiaStatusBadge(
                label: _waitStatusLabel(wait.status),
                type: _waitStatusType(wait.status),
              ),
            ],
          ),
          if (wait.description.isNotEmpty) ...[
            SizedBox(height: AppSpacing.sm),
            Text(wait.description, style: AppTypography.bodySmall(context)),
          ],
          SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.xs,
            children: [
              AmitiaStatusBadge(
                label: wait.autoResume ? '自动恢复' : '仅更新状态',
                type: wait.autoResume ? BadgeType.accent : BadgeType.neutral,
              ),
              if (wait.wakeState.isNotEmpty)
                AmitiaStatusBadge(
                  label: '唤醒 ${wait.wakeState}',
                  type: wait.wakeState == 'failed'
                      ? BadgeType.error
                      : wait.wakeState == 'delivered'
                      ? BadgeType.success
                      : BadgeType.neutral,
                ),
              if (wait.dueAt != null)
                AmitiaStatusBadge(
                  label: _formatDateTime(wait.dueAt!),
                  type: BadgeType.warning,
                ),
            ],
          ),
          if (wait.open) ...[
            SizedBox(height: AppSpacing.md),
            Row(
              children: [
                Expanded(
                  child: AmitiaButton(
                    label: '解除',
                    isSecondary: true,
                    outlined: true,
                    height: 40,
                    onPressed: _saving ? null : () => _resolveWait(wait),
                  ),
                ),
                SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '取消',
                    isSecondary: true,
                    outlined: true,
                    height: 40,
                    onPressed: _saving ? null : () => _cancelWait(wait),
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildEvents(List<ContinuityEventDto> events) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text('最近事件', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        if (events.isEmpty)
          AmitiaCard(child: Text('暂无事件', style: AppTypography.caption(context)))
        else
          ...events.map(
            (event) => Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.sm),
              child: AmitiaCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            event.eventType,
                            style: AppTypography.bodySmall(
                              context,
                            ).copyWith(fontWeight: FontWeight.w600),
                          ),
                        ),
                        Text(
                          _formatDateTime(event.occurredAt),
                          style: AppTypography.caption(context),
                        ),
                      ],
                    ),
                    if (_eventSummary(event.payloadJson).isNotEmpty) ...[
                      SizedBox(height: AppSpacing.xs),
                      Text(
                        _eventSummary(event.payloadJson),
                        style: AppTypography.caption(context),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          ),
      ],
    );
  }
}

Widget _detailRow(String label, String value) {
  if (value.trim().isEmpty) return const SizedBox.shrink();
  return Padding(
    padding: const EdgeInsets.only(bottom: 10),
    child: Builder(
      builder: (context) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: AppTypography.caption(context)),
          const SizedBox(height: 3),
          Text(value, style: AppTypography.bodySmall(context)),
        ],
      ),
    ),
  );
}

String _eventSummary(String raw) {
  try {
    final decoded = jsonDecode(raw);
    if (decoded is Map) {
      final value = decoded['summary'] ?? decoded['error'] ?? '';
      return value.toString();
    }
  } catch (_) {}
  return '';
}

String _waitStatusLabel(String status) {
  switch (status) {
    case 'resolved':
      return '已解除';
    case 'cancelled':
      return '已取消';
    default:
      return '等待中';
  }
}

BadgeType _waitStatusType(String status) {
  switch (status) {
    case 'resolved':
      return BadgeType.success;
    case 'cancelled':
      return BadgeType.neutral;
    default:
      return BadgeType.warning;
  }
}

({String label, BadgeType type}) _statusMeta(String status) {
  switch (status) {
    case 'waiting':
      return (label: '等待中', type: BadgeType.warning);
    case 'blocked':
      return (label: '已阻塞', type: BadgeType.error);
    case 'paused':
      return (label: '已暂停', type: BadgeType.info);
    case 'completed':
      return (label: '已完成', type: BadgeType.success);
    case 'cancelled':
      return (label: '已取消', type: BadgeType.neutral);
    default:
      return (label: '进行中', type: BadgeType.accent);
  }
}

String _formatDateTime(DateTime? value) {
  if (value == null) return '';
  final local = value.toLocal();
  String two(int number) => number.toString().padLeft(2, '0');
  return '${local.year}-${two(local.month)}-${two(local.day)} '
      '${two(local.hour)}:${two(local.minute)}';
}
