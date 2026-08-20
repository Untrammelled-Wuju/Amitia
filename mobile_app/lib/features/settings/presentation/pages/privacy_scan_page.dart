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
  bool _loading = true;
  bool _scanning = false;
  bool _masking = false;
  bool _deletionBusy = false;
  List<Map<String, dynamic>> _findings = const [];
  List<Map<String, dynamic>> _history = const [];
  Map<String, dynamic>? _latest;
  Map<String, dynamic> _deletionStats = const {};
  Map<String, dynamic>? _deletionStatus;
  List<Map<String, dynamic>> _securityResults = const [];
  final Set<int> _selected = {};
  final TextEditingController _targetIdController = TextEditingController();
  final TextEditingController _reasonController = TextEditingController();
  String _targetType = 'memory';
  String _deletionScope = 'all';

  @override
  void initState() {
    super.initState();
    Future.microtask(_loadResults);
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
        _selected.removeWhere((id) => !findings.any((f) => _id(f) == id && f['masked'] != true));
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _loading = false);
      _snack('加载扫描结果失败：${_message(e)}', error: true);
    }
  }

  Future<void> _scan() async {
    if (_scanning) return;
    setState(() => _scanning = true);
    try {
      final result = await ref.read(privacyServiceProvider).scan();
      if (mounted) {
        setState(() => _latest = result);
        _snack('扫描完成，发现 ${result['totalFound'] ?? result['totalFindings'] ?? 0} 项风险');
      }
      await _loadResults();
    } catch (e) {
      _snack('扫描失败：${_message(e)}', error: true);
    } finally {
      if (mounted) setState(() => _scanning = false);
    }
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
    } catch (e) {
      _snack('创建删除请求失败：${_message(e)}', error: true);
    } finally {
      if (mounted) setState(() => _deletionBusy = false);
    }
  }

  Future<void> _runDeletionCleanup() async {
    if (_deletionBusy) return;
    setState(() => _deletionBusy = true);
    try {
      final result = await ref.read(privacyServiceProvider).runDeletionCleanup();
      final cleaned = result['cleaned'] ?? 0;
      _snack('清理任务执行完成：$cleaned 项');
      final id = (_deletionStatus?['id'] ?? '').toString();
      if (id.isNotEmpty) {
        final status = await ref.read(privacyServiceProvider).deletionStatus(id);
        if (mounted) setState(() => _deletionStatus = status);
      }
      await _loadDeletionStats();
    } catch (e) {
      _snack('执行清理失败：${_message(e)}', error: true);
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
    } catch (e) {
      _snack('安全测试失败：${_message(e)}', error: true);
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

  Future<void> _maskSelected() async {
    if (_selected.isEmpty || _masking) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('确认脱敏'),
        content: Text('将选中的 ${_selected.length} 条消息内容替换为“[已脱敏]”。该操作不可撤销。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('确认脱敏')),
        ],
      ),
    );
    if (confirmed != true) return;
    setState(() => _masking = true);
    try {
      final result = await ref.read(privacyServiceProvider).mask(_selected.toList());
      _snack('已脱敏 ${result['maskedCount'] ?? _selected.length} 条记录');
      _selected.clear();
      await _loadResults();
    } catch (e) {
      _snack('脱敏失败：${_message(e)}', error: true);
    } finally {
      if (mounted) setState(() => _masking = false);
    }
  }

  @override
  void dispose() {
    _targetIdController.dispose();
    _reasonController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '隐私扫描',
        navigation: AmitiaAppBarNavigation.back,
        actions: [IconButton(onPressed: _loading ? null : _loadResults, icon: const Icon(Icons.refresh))],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : RefreshIndicator(
              onRefresh: _loadResults,
              child: ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                padding: EdgeInsets.all(AppSpacing.pagePadding),
                children: [
                  AmitiaCard(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Icon(Icons.shield_outlined, color: context.accentPrimary),
                            SizedBox(width: AppSpacing.sm),
                            Expanded(child: Text('敏感信息扫描', style: AppTypography.cardTitle(context))),
                          ],
                        ),
                        SizedBox(height: AppSpacing.sm),
                        Text('扫描聊天消息中的口令、Token、API Key、Secret 等高风险文本。扫描不会自动修改数据。', style: AppTypography.bodySmall(context)),
                        SizedBox(height: AppSpacing.md),
                        AmitiaButton(
                          label: _scanning ? '扫描中…' : '立即扫描',
                          icon: Icons.manage_search,
                          isFullWidth: true,
                          onPressed: _scanning ? null : _scan,
                        ),
                      ],
                    ),
                  ),
                  SizedBox(height: AppSpacing.sectionGap),
                  _buildSummary(context),
                  SizedBox(height: AppSpacing.sectionGap),
                  _buildFindings(context),
                  SizedBox(height: AppSpacing.sectionGap),
                  _buildHistory(context),
                  SizedBox(height: AppSpacing.sectionGap),
                  _buildDeletionLifecycle(context),
                ],
              ),
            ),
    );
  }

  Widget _buildSummary(BuildContext context) {
    final latest = _latest;
    if (latest == null) {
      return AmitiaCard(child: Text('尚未执行隐私扫描', style: AppTypography.caption(context)));
    }
    final total = latest['totalFound'] ?? latest['totalFindings'] ?? 0;
    final high = latest['highRisk'] ?? latest['high_risk'] ?? 0;
    return AmitiaCard(
      child: Row(
        children: [
          Expanded(child: _Metric(label: '已扫描', value: '${latest['totalScanned'] ?? 0}', color: context.textPrimary)),
          Expanded(child: _Metric(label: '发现风险', value: '$total', color: total == 0 ? context.success : context.warning)),
          Expanded(child: _Metric(label: '高风险', value: '$high', color: high == 0 ? context.success : context.error)),
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
            Expanded(child: Text('扫描结果 (${_findings.length})', style: AppTypography.sectionTitle(context))),
            if (_selected.isNotEmpty)
              AmitiaButton(
                label: _masking ? '处理中…' : '脱敏 (${_selected.length})',
                height: 34,
                isDestructive: true,
                onPressed: _masking ? null : _maskSelected,
              ),
          ],
        ),
        SizedBox(height: AppSpacing.md),
        if (_findings.isEmpty)
          AmitiaCard(
            child: Row(
              children: [
                Icon(Icons.check_circle_outline, color: context.success),
                SizedBox(width: AppSpacing.sm),
                Text('当前没有敏感信息扫描结果', style: AppTypography.bodySmall(context)),
              ],
            ),
          )
        else
          ..._findings.map((finding) {
            final id = _id(finding);
            final masked = finding['masked'] == true;
            final selected = id != null && _selected.contains(id);
            return Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.sm),
              child: AmitiaCard(
                onTap: masked || id == null
                    ? null
                    : () => setState(() => selected ? _selected.remove(id) : _selected.add(id)),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (!masked)
                      Checkbox(
                        value: selected,
                        onChanged: id == null ? null : (value) => setState(() => value == true ? _selected.add(id) : _selected.remove(id)),
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
                          Row(
                            children: [
                              AmitiaStatusBadge(
                                label: _severity(finding),
                                type: _severity(finding) == '高风险' ? BadgeType.error : BadgeType.warning,
                              ),
                              SizedBox(width: AppSpacing.sm),
                              Text((finding['risk_type'] ?? finding['pattern'] ?? '敏感信息').toString(), style: AppTypography.label(context)),
                              const Spacer(),
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

  Widget _buildDeletionLifecycle(BuildContext context) {
    final status = (_deletionStatus?['status'] ?? '').toString();
    final total = _deletionStats['total'] ?? _deletionStats['totalTombstones'] ?? 0;
    final pending = _deletionStats['pending'] ?? _deletionStats['pendingCleanup'] ?? 0;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('数据生命周期删除', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('对指定记忆、消息、会话或角色执行跨存储删除。提交后会立即阻止相关数据被再次检索。', style: AppTypography.bodySmall(context)),
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
          ..._securityResults.map((item) => Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.xs),
                child: AmitiaCard(
                  child: Row(
                    children: [
                      Icon(item['passed'] == true ? Icons.check_circle_outline : Icons.error_outline, color: item['passed'] == true ? context.success : context.error),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(child: Text((item['kind'] ?? 'security_test').toString(), style: AppTypography.bodySmall(context))),
                      AmitiaStatusBadge(label: item['passed'] == true ? '通过' : '未通过', type: item['passed'] == true ? BadgeType.success : BadgeType.error),
                    ],
                  ),
                ),
              )),
        ],
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
          ..._history.take(10).map((item) => Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.sm),
                child: AmitiaCard(
                  child: Row(
                    children: [
                      Icon(Icons.history, size: 20, color: context.textTertiary),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: Text(
                          (item['createdAt'] ?? item['scan_time'] ?? item['scanId'] ?? '').toString(),
                          style: AppTypography.bodySmall(context),
                        ),
                      ),
                      Text('${item['totalFound'] ?? item['totalFindings'] ?? 0} 项', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
              )),
      ],
    );
  }

  void _snack(String message, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), backgroundColor: error ? context.error : null),
    );
  }

  static List<Map<String, dynamic>> _maps(dynamic value) {
    if (value is! List) return const [];
    return value.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
  }

  static int? _id(Map<String, dynamic> finding) {
    final value = finding['id'] ?? finding['messageId'];
    if (value is int) return value;
    return int.tryParse(value?.toString() ?? '');
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
