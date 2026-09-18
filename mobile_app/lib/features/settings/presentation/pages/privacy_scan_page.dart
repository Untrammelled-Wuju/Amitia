import 'dart:convert';
import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class PrivacyScanPage extends ConsumerStatefulWidget {
  const PrivacyScanPage({super.key});

  @override
  ConsumerState<PrivacyScanPage> createState() => _PrivacyScanPageState();
}

class _PrivacyScanPageState extends ConsumerState<PrivacyScanPage> {
  static const Map<String, String> _scopeLabels = {
    'messages': '聊天消息',
    'memories': '记忆数据',
    'import_items': '导入内容',
  };

  bool _loading = true;
  bool _scanning = false;
  bool _masking = false;
  bool _deletionBusy = false;
  final Set<String> _scanScope = {'messages', 'memories', 'import_items'};
  final Set<String> _selected = {};
  List<Map<String, dynamic>> _findings = const [];
  List<Map<String, dynamic>> _history = const [];
  Map<String, dynamic>? _latest;
  Map<String, dynamic> _deletionStats = const {};
  Map<String, dynamic>? _deletionStatus;
  List<Map<String, dynamic>> _securityResults = const [];
  final TextEditingController _targetIdController = TextEditingController();
  final TextEditingController _reasonController = TextEditingController();
  String _targetType = 'memory';
  String _deletionScope = 'all';

  @override
  void initState() {
    super.initState();
    Future.microtask(_loadResults);
  }

  @override
  void dispose() {
    _targetIdController.dispose();
    _reasonController.dispose();
    super.dispose();
  }

  Future<void> _loadResults() async {
    if (mounted) setState(() => _loading = true);
    try {
      final service = ref.read(privacyServiceProvider);
      final values = await Future.wait([service.results(), service.deletionStats()]);
      final data = values[0];
      final deletionStats = values[1];
      final findings = _maps(data['items']);
      final history = _maps(data['history']);
      if (!mounted) return;
      setState(() {
        _findings = findings;
        _history = history;
        _latest = history.isEmpty ? null : history.first;
        _deletionStats = deletionStats;
        _selected.removeWhere(
          (key) => !findings.any((finding) => _findingKey(finding) == key && finding['masked'] != true),
        );
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() => _loading = false);
      _snack('加载扫描结果失败：${_message(error)}', error: true);
    }
  }

  Future<void> _scan() async {
    if (_scanning) return;
    if (_scanScope.isEmpty) {
      _snack('至少选择一个扫描范围', error: true);
      return;
    }
    setState(() => _scanning = true);
    try {
      final result = await ref.read(privacyServiceProvider).scan(scope: _scanScope.toList(growable: false));
      if (!mounted) return;
      setState(() {
        _latest = result;
        _findings = _maps(result['findings']);
        _selected.clear();
      });
      _snack('扫描完成，发现 ${result['totalFound'] ?? result['totalFindings'] ?? 0} 项风险');
      await _loadResults();
    } catch (error) {
      _snack('扫描失败：${_message(error)}', error: true);
    } finally {
      if (mounted) setState(() => _scanning = false);
    }
  }

  Future<void> _loadHistoryItem(Map<String, dynamic> item) async {
    final findings = _maps(item['findings']);
    setState(() {
      _latest = item;
      _findings = findings;
      _selected.clear();
    });
    final id = (item['scanId'] ?? item['id'] ?? '').toString();
    if (findings.isNotEmpty || id.isEmpty) return;
    try {
      final remote = await ref.read(privacyServiceProvider).scanResult(id);
      if (!mounted || remote.isEmpty) return;
      setState(() {
        _latest = remote;
        _findings = _maps(remote['findings']);
      });
    } catch (error) {
      _snack('加载历史扫描失败：${_message(error)}', error: true);
    }
  }

  Future<void> _maskSelected() async {
    if (_selected.isEmpty || _masking) return;
    final targets = _findings
        .where((finding) => _selected.contains(_findingKey(finding)) && finding['masked'] != true)
        .toList(growable: false);
    if (targets.isEmpty) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('确认脱敏'),
        content: Text('将选中的 ${targets.length} 条记录替换为“[已脱敏]”。该操作不可撤销。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('确认脱敏')),
        ],
      ),
    );
    if (confirmed != true) return;
    setState(() => _masking = true);
    try {
      final result = await ref.read(privacyServiceProvider).mask(targets);
      _snack('已脱敏 ${result['maskedCount'] ?? targets.length} 条记录');
      _selected.clear();
      final currentId = (_latest?['scanId'] ?? _latest?['id'] ?? '').toString();
      if (currentId.isNotEmpty) {
        final refreshed = await ref.read(privacyServiceProvider).scanResult(currentId);
        if (mounted && refreshed.isNotEmpty) {
          setState(() {
            _latest = refreshed;
            _findings = _maps(refreshed['findings']);
          });
        }
      }
      await _loadResultsPreservingCurrent(currentId);
    } catch (error) {
      _snack('脱敏失败：${_message(error)}', error: true);
    } finally {
      if (mounted) setState(() => _masking = false);
    }
  }

  Future<void> _loadResultsPreservingCurrent(String scanId) async {
    try {
      final data = await ref.read(privacyServiceProvider).results();
      final history = _maps(data['history']);
      if (!mounted) return;
      setState(() {
        _history = history;
        if (scanId.isEmpty) {
          _findings = _maps(data['items']);
          _latest = history.isEmpty ? null : history.first;
        }
      });
    } catch (_) {}
  }

  Future<void> _export(String format) async {
    final items = _findings;
    if (items.isEmpty) {
      _snack('当前扫描没有可导出的结果', error: true);
      return;
    }
    try {
      final now = DateTime.now();
      final stamp = '${now.year}${now.month.toString().padLeft(2, '0')}${now.day.toString().padLeft(2, '0')}_${now.hour.toString().padLeft(2, '0')}${now.minute.toString().padLeft(2, '0')}';
      final ext = format == 'csv' ? 'csv' : 'json';
      final path = await FilePicker.platform.saveFile(
        dialogTitle: '导出敏感数据扫描报告',
        fileName: 'privacy_scan_$stamp.$ext',
        type: FileType.custom,
        allowedExtensions: [ext],
      );
      if (path == null || path.trim().isEmpty) return;
      final content = format == 'csv' ? _toCsv(items) : const JsonEncoder.withIndent('  ').convert({
        'exportedAt': now.toUtc().toIso8601String(),
        'scanId': _latest?['scanId'] ?? _latest?['id'],
        'scope': _latest?['scope'] ?? _scanScope.toList(growable: false),
        'items': items,
      });
      await File(path).writeAsString(content, flush: true);
      _snack('报告已导出为 ${ext.toUpperCase()}');
    } catch (error) {
      _snack('导出失败：${_message(error)}', error: true);
    }
  }

  String _toCsv(List<Map<String, dynamic>> items) {
    String quote(Object? value) => '"${(value ?? '').toString().replaceAll('"', '""')}"';
    final rows = <List<Object?>>[
      const ['id', 'risk_level', 'risk_type', 'source_table', 'snippet', 'masked'],
      ...items.map((item) => [
            item['id'],
            item['risk_level'] ?? item['severity'],
            item['risk_type'] ?? item['pattern'],
            item['source_table'] ?? item['sourceTable'],
            item['snippet'] ?? item['preview'],
            item['masked'],
          ]),
    ];
    return rows.map((row) => row.map(quote).join(',')).join('\n');
  }

  Future<void> _requestDeletion() async {
    final targetId = _targetIdController.text.trim();
    if (targetId.isEmpty || _deletionBusy) {
      if (targetId.isEmpty) _snack('请输入要删除的数据 ID', error: true);
      return;
    }
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('确认数据生命周期删除'),
        content: Text('将阻止读取并清理 $_targetType/$targetId 相关数据。该操作不可撤销。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('提交删除')),
        ],
      ),
    );
    if (confirmed != true) return;
    setState(() => _deletionBusy = true);
    try {
      final result = await ref.read(privacyServiceProvider).requestDeletion(
            targetId: targetId,
            targetType: _targetType,
            scope: _deletionScope,
            reason: _reasonController.text.trim(),
          );
      if (!mounted) return;
      setState(() => _deletionStatus = result);
      _snack('删除请求已创建，读取已被阻止');
      await _loadDeletionStats();
    } catch (error) {
      _snack('创建删除请求失败：${_message(error)}', error: true);
    } finally {
      if (mounted) setState(() => _deletionBusy = false);
    }
  }

  Future<void> _runDeletionCleanup() async {
    if (_deletionBusy) return;
    setState(() => _deletionBusy = true);
    try {
      final result = await ref.read(privacyServiceProvider).runDeletionCleanup();
      _snack('清理任务执行完成：${result['cleaned'] ?? 0} 项');
      final id = (_deletionStatus?['id'] ?? '').toString();
      if (id.isNotEmpty) {
        final status = await ref.read(privacyServiceProvider).deletionStatus(id);
        if (mounted) setState(() => _deletionStatus = status);
      }
      await _loadDeletionStats();
    } catch (error) {
      _snack('执行清理失败：${_message(error)}', error: true);
    } finally {
      if (mounted) setState(() => _deletionBusy = false);
    }
  }

  Future<void> _runSecurityTests() async {
    final targetId = _targetIdController.text.trim();
    if (targetId.isEmpty || _deletionBusy) {
      if (targetId.isEmpty) _snack('请输入数据 ID 后再运行安全测试', error: true);
      return;
    }
    setState(() => _deletionBusy = true);
    try {
      final results = await ref.read(privacyServiceProvider).deletionSecurityTests(
            targetId: targetId,
            targetType: _targetType,
          );
      if (mounted) setState(() => _securityResults = results);
    } catch (error) {
      _snack('安全测试失败：${_message(error)}', error: true);
    } finally {
      if (mounted) setState(() => _deletionBusy = false);
    }
  }

  Future<void> _loadDeletionStats() async {
    try {
      final stats = await ref.read(privacyServiceProvider).deletionStats();
      if (mounted) setState(() => _deletionStats = stats);
    } catch (_) {}
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '敏感数据扫描',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          IconButton(
            onPressed: _loading ? null : _loadResults,
            icon: const Icon(Icons.refresh),
            tooltip: '刷新',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : RefreshIndicator(
                onRefresh: _loadResults,
                child: ListView(
                  physics: const AlwaysScrollableScrollPhysics(),
                  padding: EdgeInsets.all(AppSpacing.pagePadding),
                  children: [
                    _buildScopeCard(context),
                    SizedBox(height: AppSpacing.md),
                    if (_latest != null) _buildSummary(context),
                    if (_latest != null) SizedBox(height: AppSpacing.md),
                    _buildFindings(context),
                    SizedBox(height: AppSpacing.sectionGap),
                    _buildHistory(context),
                    SizedBox(height: AppSpacing.sectionGap),
                    _buildDeletionLifecycle(context),
                    SizedBox(height: AppSpacing.xxl),
                  ],
                ),
              ),
      ),
    );
  }

  Widget _buildScopeCard(BuildContext context) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('选择扫描范围', style: AppTypography.cardTitle(context)),
          SizedBox(height: AppSpacing.xs),
          Text('扫描仅检测敏感信息，不会自动修改或删除数据。脱敏操作必须手动确认。', style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.sm),
          ..._scopeLabels.entries.map(
            (entry) => CheckboxListTile(
              dense: true,
              contentPadding: EdgeInsets.zero,
              title: Text(entry.value),
              value: _scanScope.contains(entry.key),
              onChanged: _scanning
                  ? null
                  : (value) => setState(() {
                        if (value == true) {
                          _scanScope.add(entry.key);
                        } else {
                          _scanScope.remove(entry.key);
                        }
                      }),
            ),
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Expanded(
                child: AmitiaButton(
                  label: _scanning ? '扫描中...' : '开始扫描',
                  icon: Icons.policy_outlined,
                  onPressed: _scanning || _scanScope.isEmpty ? null : _scan,
                ),
              ),
              if (_latest != null) ...[
                SizedBox(width: AppSpacing.sm),
                OutlinedButton(onPressed: () => _export('csv'), child: const Text('CSV')),
                SizedBox(width: AppSpacing.xs),
                OutlinedButton(onPressed: () => _export('json'), child: const Text('JSON')),
              ],
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildSummary(BuildContext context) {
    final latest = _latest ?? const {};
    return AmitiaCard(
      child: Row(
        children: [
          Expanded(child: _Metric(label: '发现', value: '${latest['totalFound'] ?? latest['totalFindings'] ?? _findings.length}', color: context.textPrimary)),
          Expanded(child: _Metric(label: '高风险', value: '${latest['highRisk'] ?? latest['high_risk'] ?? 0}', color: context.error)),
          Expanded(child: _Metric(label: '中风险', value: '${latest['mediumRisk'] ?? 0}', color: context.warning)),
          Expanded(child: _Metric(label: '扫描记录', value: '${latest['totalScanned'] ?? 0}', color: context.accentPrimary)),
        ],
      ),
    );
  }

  Widget _buildFindings(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(child: Text('详细结果 (${_findings.length})', style: AppTypography.sectionTitle(context))),
            if (_selected.isNotEmpty)
              AmitiaButton(
                label: _masking ? '处理中...' : '脱敏 ${_selected.length} 条',
                isDestructive: true,
                onPressed: _masking ? null : _maskSelected,
              ),
          ],
        ),
        SizedBox(height: AppSpacing.md),
        if (_findings.isEmpty)
          AmitiaCard(child: Text('当前扫描没有发现敏感信息', style: AppTypography.caption(context)))
        else
          ..._findings.map((finding) {
            final key = _findingKey(finding);
            final masked = finding['masked'] == true;
            final selected = _selected.contains(key);
            final source = _sourceLabel((finding['source_table'] ?? finding['sourceTable'] ?? '').toString());
            return Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.sm),
              child: AmitiaCard(
                onTap: masked
                    ? null
                    : () => setState(() => selected ? _selected.remove(key) : _selected.add(key)),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (!masked)
                      Checkbox(
                        value: selected,
                        onChanged: (value) => setState(() => value == true ? _selected.add(key) : _selected.remove(key)),
                      )
                    else
                      Padding(
                        padding: const EdgeInsets.all(12),
                        child: Icon(Icons.visibility_off_outlined, size: 20, color: context.textTertiary),
                      ),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Wrap(
                            spacing: AppSpacing.xs,
                            runSpacing: AppSpacing.xs,
                            crossAxisAlignment: WrapCrossAlignment.center,
                            children: [
                              AmitiaStatusBadge(
                                label: _severity(finding),
                                type: _severity(finding) == '高风险' ? BadgeType.error : BadgeType.warning,
                              ),
                              AmitiaStatusBadge(label: source, type: BadgeType.neutral),
                              Text((finding['risk_type'] ?? finding['pattern'] ?? '敏感信息').toString(), style: AppTypography.label(context)),
                              if (masked) const AmitiaStatusBadge(label: '已脱敏', type: BadgeType.success),
                            ],
                          ),
                          SizedBox(height: AppSpacing.xs),
                          Text((finding['snippet'] ?? finding['preview'] ?? '').toString(), style: AppTypography.bodySmall(context)),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            );
          }),
      ],
    );
  }

  Widget _buildHistory(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('扫描历史', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        if (_history.isEmpty)
          AmitiaCard(child: Text('暂无扫描记录', style: AppTypography.caption(context)))
        else
          ..._history.take(10).map(
                (item) => Padding(
                  padding: EdgeInsets.only(bottom: AppSpacing.sm),
                  child: AmitiaCard(
                    onTap: () => _loadHistoryItem(item),
                    child: Row(
                      children: [
                        Icon(Icons.history, size: 20, color: context.textTertiary),
                        SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text((item['createdAt'] ?? item['scan_time'] ?? item['scanId'] ?? '').toString(), style: AppTypography.bodySmall(context)),
                              Text(_scopeText(item['scope']), style: AppTypography.caption(context)),
                            ],
                          ),
                        ),
                        Text('${item['totalFound'] ?? item['totalFindings'] ?? 0} 项', style: AppTypography.caption(context)),
                        const Icon(Icons.chevron_right, size: 18),
                      ],
                    ),
                  ),
                ),
              ),
      ],
    );
  }

  Widget _buildDeletionLifecycle(BuildContext context) {
    final status = (_deletionStatus?['status'] ?? '').toString();
    final total = _deletionStats['total'] ?? _deletionStats['totalTombstones'] ?? 0;
    final pending = _deletionStats['pending'] ?? _deletionStats['pendingCleanup'] ?? 0;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('数据生命周期删除', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('对指定记忆、消息、会话或角色执行跨存储删除。提交后会立即阻止相关数据再次被检索。', style: AppTypography.bodySmall(context)),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            children: [
              Row(
                children: [
                  Expanded(
                    child: DropdownButtonFormField<String>(
                      value: _targetType,
                      decoration: const InputDecoration(labelText: '数据类型'),
                      items: const [
                        DropdownMenuItem(value: 'memory', child: Text('记忆')),
                        DropdownMenuItem(value: 'message', child: Text('消息')),
                        DropdownMenuItem(value: 'conversation', child: Text('会话')),
                        DropdownMenuItem(value: 'character', child: Text('角色')),
                      ],
                      onChanged: _deletionBusy ? null : (value) => setState(() => _targetType = value ?? 'memory'),
                    ),
                  ),
                  SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: DropdownButtonFormField<String>(
                      value: _deletionScope,
                      decoration: const InputDecoration(labelText: '删除范围'),
                      items: const [
                        DropdownMenuItem(value: 'all', child: Text('全部关联数据')),
                        DropdownMenuItem(value: 'memory', child: Text('记忆')),
                        DropdownMenuItem(value: 'belief', child: Text('信念')),
                        DropdownMenuItem(value: 'relation', child: Text('关系')),
                        DropdownMenuItem(value: 'trace', child: Text('运行轨迹')),
                      ],
                      onChanged: _deletionBusy ? null : (value) => setState(() => _deletionScope = value ?? 'all'),
                    ),
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.sm),
              TextField(controller: _targetIdController, decoration: const InputDecoration(labelText: '数据 ID', hintText: 'memory / message / conversation / character ID')),
              SizedBox(height: AppSpacing.sm),
              TextField(controller: _reasonController, decoration: const InputDecoration(labelText: '删除原因（可选）')),
              SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  Expanded(child: AmitiaButton(label: '运行安全测试', onPressed: _deletionBusy ? null : _runSecurityTests)),
                  SizedBox(width: AppSpacing.sm),
                  Expanded(child: AmitiaButton(label: '提交删除', isDestructive: true, onPressed: _deletionBusy ? null : _requestDeletion)),
                ],
              ),
              SizedBox(height: AppSpacing.sm),
              SizedBox(
                width: double.infinity,
                child: OutlinedButton.icon(
                  onPressed: _deletionBusy ? null : _runDeletionCleanup,
                  icon: const Icon(Icons.cleaning_services_outlined),
                  label: const Text('执行待处理清理'),
                ),
              ),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Row(
            children: [
              Expanded(child: _Metric(label: '删除记录', value: '$total', color: context.textPrimary)),
              Expanded(child: _Metric(label: '待清理', value: '$pending', color: context.warning)),
              Expanded(child: _Metric(label: '当前状态', value: status.isEmpty ? '—' : status, color: status == 'completed' ? context.success : context.textPrimary)),
            ],
          ),
        ),
        if (_securityResults.isNotEmpty) ...[
          SizedBox(height: AppSpacing.sm),
          ..._securityResults.map(
            (item) => Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.xs),
              child: AmitiaCard(
                child: Row(
                  children: [
                    Icon(item['passed'] == true ? Icons.check_circle_outline : Icons.error_outline, color: item['passed'] == true ? context.success : context.error),
                    SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text((item['kind'] ?? 'security_test').toString(), style: AppTypography.bodySmall(context)),
                          if ((item['detail'] ?? '').toString().isNotEmpty) Text(item['detail'].toString(), style: AppTypography.caption(context)),
                        ],
                      ),
                    ),
                    AmitiaStatusBadge(label: item['passed'] == true ? '通过' : '未通过', type: item['passed'] == true ? BadgeType.success : BadgeType.error),
                  ],
                ),
              ),
            ),
          ),
        ],
      ],
    );
  }

  String _scopeText(dynamic value) {
    if (value is! List) return '';
    return value.map((item) => _scopeLabels[item.toString()] ?? item.toString()).join(' · ');
  }

  String _sourceLabel(String source) {
    switch (source) {
      case 'messages':
        return '聊天消息';
      case 'memories':
        return '记忆';
      case 'import_items':
        return '导入内容';
      default:
        return source.isEmpty ? '未知来源' : source;
    }
  }

  void _snack(String message, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), backgroundColor: error ? context.error : null),
    );
  }

  static List<Map<String, dynamic>> _maps(dynamic value) {
    if (value is! List) return const [];
    return value.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  static String _findingKey(Map<String, dynamic> finding) {
    final source = (finding['source_table'] ?? finding['sourceTable'] ?? 'messages').toString();
    final id = (finding['id'] ?? finding['recordId'] ?? finding['messageId'] ?? '').toString();
    return '$source::$id';
  }

  static String _severity(Map<String, dynamic> finding) {
    final raw = (finding['risk_level'] ?? finding['severity'] ?? '').toString().toLowerCase();
    return raw == 'critical' || raw == 'high' ? '高风险' : raw == 'medium' ? '中风险' : '低风险';
  }

  static String _message(Object error) => error.toString().replaceFirst('Exception: ', '');
}

class _Metric extends StatelessWidget {
  final String label;
  final String value;
  final Color color;

  const _Metric({required this.label, required this.value, required this.color});

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Text(value, style: AppTypography.cardTitle(context).copyWith(color: color, fontSize: 20)),
        const SizedBox(height: 2),
        Text(label, style: AppTypography.label(context)),
      ],
    );
  }
}
