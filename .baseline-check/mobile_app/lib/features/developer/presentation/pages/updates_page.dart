import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class UpdatesPage extends ConsumerStatefulWidget {
  const UpdatesPage({super.key});

  @override
  ConsumerState<UpdatesPage> createState() => _UpdatesPageState();
}

class _UpdatesPageState extends ConsumerState<UpdatesPage> {
  bool _loading = true;
  bool _checking = false;
  String? _error;
  Map<String, dynamic> _version = {};
  Map<String, dynamic> _check = {};
  Map<String, dynamic> _config = {};
  Map<String, dynamic> _latest = {};
  List<Map<String, dynamic>> _history = [];
  bool _extensionsLoading = true;
  bool _extensionBusy = false;
  List<Map<String, dynamic>> _extensions = const [];
  String _selectedExtension = '';
  Map<String, dynamic> _extensionUpdate = const {};
  Map<String, dynamic> _extensionOperation = const {};
  List<Map<String, dynamic>> _extensionSteps = const [];

  @override
  void initState() {
    super.initState();
    _load();
    _loadExtensions();
  }

  Future<Map<String, dynamic>?> _get(String path) {
    return ref.read(backendServiceProvider).get<Map<String, dynamic>>(
          path,
          fromJson: (value) => Map<String, dynamic>.from(value as Map),
        );
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final values = await Future.wait([
        _get('/api/version'),
        _get('/api/update/check'),
        _get('/api/update/config'),
        _get('/api/release-check/latest'),
        _get('/api/release-check/history'),
      ]);
      if (!mounted) return;
      final historyRaw = values[4]?['history'] as List<dynamic>? ?? const [];
      setState(() {
        _version = values[0] ?? {};
        _check = values[1] ?? {};
        _config = values[2] ?? {};
        _latest = values[3]?['latest'] is Map ? Map<String, dynamic>.from(values[3]!['latest'] as Map) : (values[3] ?? {});
        _history = historyRaw.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList();
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

  Future<void> _runCheck() async {
    setState(() => _checking = true);
    try {
      final api = ref.read(backendServiceProvider);
      await api.post<Map<String, dynamic>>(
        '/api/release-check/run',
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('版本检查已完成')));
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
    } finally {
      if (mounted) setState(() => _checking = false);
    }
  }

  Future<void> _setAutoCheck(bool value) async {
    try {
      final result = await ref.read(backendServiceProvider).put<Map<String, dynamic>>(
        '/api/update/config',
        data: {'autoCheck': value},
        fromJson: (raw) => Map<String, dynamic>.from(raw as Map),
      );
      if (!mounted) return;
      setState(() => _config = result ?? {..._config, 'autoCheck': value});
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '更新中心',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [AmitiaIconButton(icon: Icons.refresh, onPressed: () { _load(); _loadExtensions(); }, tooltip: '刷新')],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    final current = (_version['version'] ?? _check['currentVersion'] ?? '').toString();
    final latest = (_check['latestVersion'] ?? _latest['version'] ?? current).toString();
    final hasUpdate = _check['hasUpdate'] == true;
    final buildTime = (_version['buildTime'] ?? '').toString();
    final autoCheck = _config['autoCheck'] != false;
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
        children: [
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Container(
                        width: 48,
                        height: 48,
                        decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                        child: Icon(Icons.system_update_outlined, color: context.accentPrimary),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(current.isEmpty ? 'Amitia' : 'v$current', style: AppTypography.cardTitle(context).copyWith(fontSize: 18)),
                            if (buildTime.isNotEmpty) Text('构建时间 $buildTime', style: AppTypography.caption(context)),
                          ],
                        ),
                      ),
                      AmitiaStatusBadge(
                        label: hasUpdate ? '检测到更新' : '已是当前版本',
                        type: hasUpdate ? BadgeType.accent : BadgeType.success,
                      ),
                    ],
                  ),
                  SizedBox(height: AppSpacing.md),
                  _row(context, '当前版本', current),
                  _row(context, '检查结果版本', latest),
                  _row(context, '更新通道', (_config['channel'] ?? 'stable').toString()),
                  _row(context, '最后检查', (_check['lastCheckedAt'] ?? _history.firstOrNull?['checkedAt'] ?? '未记录').toString()),
                  SizedBox(height: AppSpacing.md),
                  AmitiaButton(
                    label: _checking ? '检查中...' : '立即检查',
                    isFullWidth: true,
                    icon: Icons.sync,
                    onPressed: _checking ? null : _runCheck,
                  ),
                  SizedBox(height: AppSpacing.sm),
                  Text(
                    hasUpdate
                        ? '后端已检测到新版本。当前后端没有提供跨平台安装 API，因此本页不会伪造下载或安装状态。'
                        : '版本检查结果来自后端 release-check/update 接口。',
                    style: AppTypography.caption(context).copyWith(color: context.textSecondary),
                  ),
                ],
              ),
            ),
          ),
          SizedBox(height: AppSpacing.xl),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaCard(
              child: SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: Text('自动检查更新', style: AppTypography.cardTitle(context)),
                subtitle: Text('由后端更新配置持久化', style: AppTypography.caption(context)),
                value: autoCheck,
                onChanged: _setAutoCheck,
              ),
            ),
          ),
          SizedBox(height: AppSpacing.xl),
          const AmitiaSectionHeader(title: '检查历史'),
          SizedBox(height: AppSpacing.sm),
          if (_history.isEmpty)
            const AmitiaEmptyState(icon: Icons.history, title: '暂无检查历史')
          else
            ..._history.map((item) => Padding(
                  padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
                  child: AmitiaCard(
                    onTap: () => _showJson(item),
                    child: Row(
                      children: [
                        Icon(Icons.history, color: context.textSecondary),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text('v${item['version'] ?? ''}', style: AppTypography.cardTitle(context)),
                              Text((item['checkedAt'] ?? '').toString(), style: AppTypography.caption(context)),
                            ],
                          ),
                        ),
                        AmitiaStatusBadge(
                          label: item['hasUpdate'] == true ? '有更新' : '无更新',
                          type: item['hasUpdate'] == true ? BadgeType.accent : BadgeType.neutral,
                        ),
                      ],
                    ),
                  ),
                )),
          SizedBox(height: AppSpacing.xl),
          const AmitiaSectionHeader(title: '扩展更新'),
          SizedBox(height: AppSpacing.sm),
          _buildExtensionUpdateCenter(context),
        ],
      ),
    );
  }

  Future<void> _loadExtensions() async {
    if (mounted) setState(() => _extensionsLoading = true);
    try {
      final items = await ref.read(extensionServiceProvider).kernelExtensions();
      if (!mounted) return;
      setState(() {
        _extensions = items;
        if (_selectedExtension.isEmpty && items.isNotEmpty) {
          _selectedExtension = (items.first['extensionId'] ?? '').toString();
        } else if (_selectedExtension.isNotEmpty && !items.any((e) => e['extensionId']?.toString() == _selectedExtension)) {
          _selectedExtension = items.isEmpty ? '' : (items.first['extensionId'] ?? '').toString();
        }
        _extensionsLoading = false;
      });
    } catch (e) {
      if (mounted) {
        setState(() => _extensionsLoading = false);
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('扩展列表加载失败: $e')));
      }
    }
  }

  Widget _buildExtensionUpdateCenter(BuildContext context) {
    if (_extensionsLoading) {
      return Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        child: const AmitiaCard(child: Center(child: Padding(padding: EdgeInsets.all(20), child: CircularProgressIndicator()))),
      );
    }
    if (_extensions.isEmpty) {
      return const AmitiaEmptyState(icon: Icons.extension_outlined, title: '暂无已安装扩展');
    }
    final update = _extensionUpdate;
    final op = _extensionOperation;
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            DropdownButtonFormField<String>(
              initialValue: _selectedExtension.isEmpty ? null : _selectedExtension,
              decoration: const InputDecoration(labelText: '扩展'),
              items: _extensions.map((item) {
                final id = (item['extensionId'] ?? '').toString();
                final version = (item['version'] ?? '').toString();
                return DropdownMenuItem(value: id, child: Text('$id${version.isEmpty ? '' : ' · v$version'}'));
              }).toList(),
              onChanged: _extensionBusy ? null : (value) => setState(() {
                _selectedExtension = value ?? '';
                _extensionUpdate = const {};
                _extensionOperation = const {};
                _extensionSteps = const [];
              }),
            ),
            SizedBox(height: AppSpacing.md),
            AmitiaButton(
              label: _extensionBusy ? '处理中...' : '检查扩展更新',
              isFullWidth: true,
              icon: Icons.extension_outlined,
              onPressed: _extensionBusy || _selectedExtension.isEmpty ? null : _checkExtensionUpdate,
            ),
            if (update.isNotEmpty) ...[
              SizedBox(height: AppSpacing.md),
              _row(context, '目标版本', (update['version'] ?? '').toString()),
              _row(context, '发布通道', (update['releaseChannel'] ?? '-').toString()),
              _row(context, '发布者', (update['publisherId'] ?? '-').toString()),
              _row(context, '包大小', _formatBytes(update['packageSize'])),
              _row(context, '发布时间', (update['publishedAt'] ?? '-').toString()),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  FilledButton.icon(onPressed: _extensionBusy ? null : _downloadExtensionUpdate, icon: const Icon(Icons.download), label: const Text('下载')),
                  OutlinedButton.icon(onPressed: _extensionBusy || op['operationId'] == null ? null : _installExtensionUpdate, icon: const Icon(Icons.install_desktop), label: const Text('安装')),
                  OutlinedButton(onPressed: _extensionBusy || op['operationId'] == null ? null : () => _operateExtensionUpdate('cancel'), child: const Text('取消')),
                  OutlinedButton(onPressed: _extensionBusy || op['operationId'] == null ? null : () => _operateExtensionUpdate('retry'), child: const Text('重试')),
                  OutlinedButton(onPressed: _extensionBusy || op['operationId'] == null ? null : () => _operateExtensionUpdate('rollback'), child: const Text('回滚')),
                ],
              ),
            ],
            if (op.isNotEmpty) ...[
              SizedBox(height: AppSpacing.lg),
              Text('当前操作', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.sm),
              _row(context, 'Operation ID', (op['operationId'] ?? '').toString()),
              _row(context, '状态', (op['status'] ?? '').toString()),
              _row(context, '版本', (op['version'] ?? update['version'] ?? '').toString()),
              if ((op['error'] ?? '').toString().isNotEmpty) _row(context, '错误', op['error'].toString()),
              Row(
                children: [
                  TextButton.icon(onPressed: _refreshExtensionOperation, icon: const Icon(Icons.refresh), label: const Text('刷新状态')),
                  TextButton.icon(onPressed: _loadExtensionSteps, icon: const Icon(Icons.list_alt), label: const Text('操作步骤')),
                ],
              ),
            ],
            if (_extensionSteps.isNotEmpty) ...[
              const Divider(),
              Text('执行步骤', style: AppTypography.cardTitle(context)),
              ..._extensionSteps.map((step) => ListTile(
                    dense: true,
                    contentPadding: EdgeInsets.zero,
                    leading: Icon(_stepIcon((step['status'] ?? '').toString())),
                    title: Text((step['name'] ?? step['stepId'] ?? '').toString()),
                    subtitle: Text('${step['status'] ?? ''}${(step['error'] ?? '').toString().isEmpty ? '' : ' · ${step['error']}'}'),
                  )),
            ],
          ],
        ),
      ),
    );
  }

  Future<void> _checkExtensionUpdate() async {
    setState(() => _extensionBusy = true);
    try {
      final result = await ref.read(extensionServiceProvider).checkKernelExtensionUpdate(_selectedExtension);
      final items = result['items'];
      final list = items is List ? items.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : <Map<String, dynamic>>[];
      if (!mounted) return;
      setState(() {
        _extensionUpdate = list.isEmpty ? const {} : list.first;
        _extensionOperation = const {};
        _extensionSteps = const [];
      });
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(list.isEmpty ? '该扩展已是最新版本' : '发现扩展更新 v${list.first['version']}')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('检查扩展更新失败: $e')));
    } finally {
      if (mounted) setState(() => _extensionBusy = false);
    }
  }

  Future<void> _downloadExtensionUpdate() async {
    final version = (_extensionUpdate['version'] ?? '').toString();
    if (version.isEmpty) return;
    await _withExtensionBusy(() async {
      final op = await ref.read(extensionServiceProvider).downloadKernelExtensionUpdate(_selectedExtension, version);
      setState(() => _extensionOperation = op);
      await _refreshExtensionOperation();
    });
  }

  Future<void> _installExtensionUpdate() async {
    final operationId = (_extensionOperation['operationId'] ?? '').toString();
    if (operationId.isEmpty) return;
    await _withExtensionBusy(() async {
      await ref.read(extensionServiceProvider).installKernelExtensionUpdate(_selectedExtension, operationId);
      await _refreshExtensionOperation();
      await _loadExtensionSteps();
      await _loadExtensions();
    });
  }

  Future<void> _operateExtensionUpdate(String action) async {
    final operationId = (_extensionOperation['operationId'] ?? '').toString();
    if (operationId.isEmpty) return;
    await _withExtensionBusy(() async {
      final service = ref.read(extensionServiceProvider);
      switch (action) {
        case 'cancel': await service.cancelKernelExtensionUpdate(_selectedExtension, operationId); break;
        case 'retry': await service.retryKernelExtensionUpdate(_selectedExtension, operationId); break;
        case 'rollback': await service.rollbackKernelExtensionUpdate(_selectedExtension, operationId); break;
      }
      await _refreshExtensionOperation();
      await _loadExtensionSteps();
    });
  }

  Future<void> _refreshExtensionOperation() async {
    final operationId = (_extensionOperation['operationId'] ?? '').toString();
    if (operationId.isEmpty) return;
    final op = await ref.read(extensionServiceProvider).kernelUpdateOperation(operationId);
    if (mounted) setState(() => _extensionOperation = op);
  }

  Future<void> _loadExtensionSteps() async {
    final operationId = (_extensionOperation['operationId'] ?? '').toString();
    if (operationId.isEmpty) return;
    final steps = await ref.read(extensionServiceProvider).kernelUpdateOperationSteps(operationId);
    if (mounted) setState(() => _extensionSteps = steps);
  }

  Future<void> _withExtensionBusy(Future<void> Function() action) async {
    if (mounted) setState(() => _extensionBusy = true);
    try {
      await action();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('扩展更新操作失败: $e')));
    } finally {
      if (mounted) setState(() => _extensionBusy = false);
    }
  }

  String _formatBytes(dynamic value) {
    final bytes = value is num ? value.toDouble() : double.tryParse(value?.toString() ?? '') ?? 0;
    if (bytes <= 0) return '-';
    if (bytes < 1024) return '${bytes.toStringAsFixed(0)} B';
    if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }

  IconData _stepIcon(String status) {
    switch (status.toLowerCase()) {
      case 'completed': case 'success': return Icons.check_circle_outline;
      case 'failed': return Icons.error_outline;
      case 'running': return Icons.sync;
      default: return Icons.radio_button_unchecked;
    }
  }

  Widget _row(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          SizedBox(width: 110, child: Text(label, style: AppTypography.label(context))),
          Expanded(child: Text(value.isEmpty ? '-' : value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }

  void _showJson(Map<String, dynamic> item) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('检查详情'),
        content: SelectableText(const JsonEncoder.withIndent('  ').convert(item)),
        actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('关闭'))],
      ),
    );
  }
}
