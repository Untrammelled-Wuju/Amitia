import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/backend_transport/errors/backend_transport_error.dart';
import '../../../../core/backend_transport/errors/backend_transport_error_code.dart';
import '../../../../core/services/providers.dart';
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
  Map<String, dynamic> _extensionDownload = const {};
  Map<String, dynamic> _extensionPreview = const {};
  Map<String, dynamic> _extensionOperation = const {};
  bool _extensionConfirmed = false;

  @override
  void initState() {
    super.initState();
    _load();
    _loadExtensions();
  }

  Future<Map<String, dynamic>?> _get(String path) {
    return ref
        .read(backendServiceProvider)
        .get<Map<String, dynamic>>(
          path,
          fromJson: (value) => Map<String, dynamic>.from(value as Map),
        );
  }

  Future<Map<String, dynamic>?> _getOptional(String path) async {
    try {
      return await _get(path);
    } on BackendTransportError catch (error) {
      if (error.code == BackendTransportErrorCode.notFound ||
          error.statusCode == 404) {
        return null;
      }
      rethrow;
    }
  }

  Future<Map<String, dynamic>?> _checkCoreUpdateOptional() async {
    try {
      return await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>(
            '/api/update/check',
            fromJson: (value) => Map<String, dynamic>.from(value as Map),
          );
    } on BackendTransportError catch (error) {
      if (error.code == BackendTransportErrorCode.notFound ||
          error.statusCode == 404) {
        return null;
      }
      rethrow;
    }
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final values = await Future.wait([
        _getOptional('/api/version'),
        _checkCoreUpdateOptional(),
        _getOptional('/api/update/config'),
        _getOptional('/api/release-check/latest'),
        _getOptional('/api/release-check/history'),
      ]);
      if (!mounted) return;
      final historyRaw = values[4]?['history'] as List<dynamic>? ?? const [];
      setState(() {
        _version = values[0] ?? {};
        _check = values[1] ?? {};
        _config = values[2] ?? {};
        _latest = values[3]?['latest'] is Map
            ? Map<String, dynamic>.from(values[3]!['latest'] as Map)
            : (values[3] ?? {});
        _history = historyRaw
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList();
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
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('版本检查已完成')));
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(e.toString())));
    } finally {
      if (mounted) setState(() => _checking = false);
    }
  }

  Future<void> _setAutoCheck(bool value) async {
    try {
      final result = await ref
          .read(backendServiceProvider)
          .put<Map<String, dynamic>>(
            '/api/update/config',
            data: {'autoCheck': value},
            fromJson: (raw) => Map<String, dynamic>.from(raw as Map),
          );
      if (!mounted) return;
      setState(() => _config = result ?? {..._config, 'autoCheck': value});
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '更新中心',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            onPressed: () {
              _load();
              _loadExtensions();
            },
            tooltip: '刷新',
          ),
        ],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) {
      return AmitiaErrorState(message: _error!, onRetry: _load);
    }
    final current = (_version['version'] ?? _check['currentVersion'] ?? '')
        .toString();
    final latest = (_check['latestVersion'] ?? _latest['version'] ?? current)
        .toString();
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
                        decoration: BoxDecoration(
                          color: context.accentSoft,
                          borderRadius: AppRadius.brSmall,
                        ),
                        child: Icon(
                          Icons.system_update_outlined,
                          color: context.accentPrimary,
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              current.isEmpty ? 'Amitia' : 'v$current',
                              style: AppTypography.cardTitle(
                                context,
                              ).copyWith(fontSize: 18),
                            ),
                            if (buildTime.isNotEmpty)
                              Text(
                                '构建时间 $buildTime',
                                style: AppTypography.caption(context),
                              ),
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
                  _row(
                    context,
                    '更新通道',
                    (_config['channel'] ?? 'stable').toString(),
                  ),
                  _row(
                    context,
                    '最后检查',
                    (_check['lastCheckedAt'] ??
                            _history.firstOrNull?['checkedAt'] ??
                            '未记录')
                        .toString(),
                  ),
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
                    style: AppTypography.caption(
                      context,
                    ).copyWith(color: context.textSecondary),
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
                subtitle: Text(
                  '由后端更新配置持久化',
                  style: AppTypography.caption(context),
                ),
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
            ..._history.map(
              (item) => Padding(
                padding: EdgeInsets.symmetric(
                  horizontal: AppSpacing.pagePadding,
                  vertical: AppSpacing.xs,
                ),
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
                            Text(
                              'v${item['version'] ?? ''}',
                              style: AppTypography.cardTitle(context),
                            ),
                            Text(
                              (item['checkedAt'] ?? '').toString(),
                              style: AppTypography.caption(context),
                            ),
                          ],
                        ),
                      ),
                      AmitiaStatusBadge(
                        label: item['hasUpdate'] == true ? '有更新' : '无更新',
                        type: item['hasUpdate'] == true
                            ? BadgeType.accent
                            : BadgeType.neutral,
                      ),
                    ],
                  ),
                ),
              ),
            ),
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
        } else if (_selectedExtension.isNotEmpty &&
            !items.any(
              (e) => e['extensionId']?.toString() == _selectedExtension,
            )) {
          _selectedExtension = items.isEmpty
              ? ''
              : (items.first['extensionId'] ?? '').toString();
        }
        _extensionsLoading = false;
      });
    } catch (e) {
      if (mounted) {
        setState(() => _extensionsLoading = false);
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('扩展列表加载失败: $e')));
      }
    }
  }

  Widget _buildExtensionUpdateCenter(BuildContext context) {
    if (_extensionsLoading) {
      return Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        child: const AmitiaCard(
          child: Center(
            child: Padding(
              padding: EdgeInsets.all(20),
              child: CircularProgressIndicator(),
            ),
          ),
        ),
      );
    }
    if (_extensions.isEmpty) {
      return const AmitiaEmptyState(
        icon: Icons.extension_outlined,
        title: '暂无已安装扩展',
      );
    }
    final update = _extensionUpdate;
    final download = _extensionDownload;
    final preview = _extensionPreview;
    final op = _extensionOperation;
    final steps = ((op['steps'] as List?) ?? const [])
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
    final downloadStatus = (download['status'] ?? '').toString();
    final downloading = const {
      'pending',
      'downloading',
      'paused',
    }.contains(downloadStatus);
    final expectedSize = (download['expectedSize'] as num?)?.toInt() ?? 0;
    final downloadedSize = (download['downloadedSize'] as num?)?.toInt() ?? 0;
    final progress = expectedSize <= 0
        ? downloadStatus == 'completed'
              ? 1.0
              : 0.0
        : (downloadedSize / expectedSize).clamp(0.0, 1.0);
    final requiredConfirmations =
        ((preview['requiredConfirmations'] as List?) ?? const [])
            .map((item) => item.toString())
            .where((item) => item.isNotEmpty)
            .toList(growable: false);
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            DropdownButtonFormField<String>(
              initialValue: _selectedExtension.isEmpty
                  ? null
                  : _selectedExtension,
              decoration: const InputDecoration(labelText: '扩展'),
              items: _extensions.map((item) {
                final id = (item['extensionId'] ?? '').toString();
                final version = (item['version'] ?? '').toString();
                return DropdownMenuItem(
                  value: id,
                  child: Text('$id${version.isEmpty ? '' : ' · v$version'}'),
                );
              }).toList(),
              onChanged: _extensionBusy
                  ? null
                  : (value) => setState(() {
                      _selectedExtension = value ?? '';
                      _extensionUpdate = const {};
                      _extensionDownload = const {};
                      _extensionPreview = const {};
                      _extensionOperation = const {};
                      _extensionConfirmed = false;
                    }),
            ),
            SizedBox(height: AppSpacing.md),
            AmitiaButton(
              label: _extensionBusy ? '处理中...' : '检查扩展更新',
              isFullWidth: true,
              icon: Icons.extension_outlined,
              onPressed: _extensionBusy || _selectedExtension.isEmpty
                  ? null
                  : _checkExtensionUpdate,
            ),
            if (update.isNotEmpty) ...[
              SizedBox(height: AppSpacing.md),
              _row(context, '目标版本', (update['version'] ?? '').toString()),
              _row(
                context,
                '发布通道',
                (update['releaseChannel'] ?? '-').toString(),
              ),
              _row(context, '发布者', (update['publisherId'] ?? '-').toString()),
              _row(context, '包大小', _formatBytes(update['packageSize'])),
              _row(context, '发布时间', (update['publishedAt'] ?? '-').toString()),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  FilledButton.icon(
                    onPressed: _extensionBusy || preview.isNotEmpty
                        ? null
                        : _downloadExtensionUpdate,
                    icon: const Icon(Icons.download),
                    label: const Text('下载并预览'),
                  ),
                  if (downloading)
                    OutlinedButton(
                      onPressed: _cancelExtensionDownload,
                      child: const Text('取消下载'),
                    ),
                ],
              ),
            ],
            if (download.isNotEmpty) ...[
              SizedBox(height: AppSpacing.md),
              LinearProgressIndicator(value: progress),
              SizedBox(height: AppSpacing.xs),
              Text(
                '$downloadStatus · ${_formatBytes(downloadedSize)} / ${_formatBytes(expectedSize)}'
                '${(download['error'] ?? '').toString().isEmpty ? '' : ' · ${download['error']}'}',
                style: AppTypography.caption(context),
              ),
            ],
            if (preview.isNotEmpty) ...[
              SizedBox(height: AppSpacing.lg),
              Text('Kernel 包预览', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.sm),
              _row(
                context,
                '扩展',
                (preview['name'] ?? preview['id'] ?? '').toString(),
              ),
              _row(context, '目标版本', (preview['version'] ?? '').toString()),
              _row(
                context,
                '当前版本',
                (preview['currentVersion'] ?? '').toString(),
              ),
              _row(
                context,
                '签名',
                ((preview['signature'] as Map?)?['status'] ?? '未提供').toString(),
              ),
              _row(context, '兼容性', preview['compatible'] == true ? '通过' : '失败'),
              if (requiredConfirmations.isNotEmpty)
                _row(context, '必需确认', requiredConfirmations.join('、')),
              if ((preview['errors'] as List?)?.isNotEmpty == true)
                Text(
                  (preview['errors'] as List).join('；'),
                  style: AppTypography.caption(
                    context,
                  ).copyWith(color: context.error),
                ),
              CheckboxListTile(
                contentPadding: EdgeInsets.zero,
                value: _extensionConfirmed,
                onChanged: preview['installable'] == true
                    ? (value) =>
                          setState(() => _extensionConfirmed = value == true)
                    : null,
                title: const Text('我已检查签名、权限、迁移和回滚信息，确认执行真实更新'),
              ),
              FilledButton.icon(
                onPressed:
                    _extensionBusy ||
                        preview['installable'] != true ||
                        !_extensionConfirmed
                    ? null
                    : _installExtensionPreview,
                icon: const Icon(Icons.system_update_alt),
                label: const Text('确认更新'),
              ),
            ],
            if (op.isNotEmpty) ...[
              SizedBox(height: AppSpacing.lg),
              Text('Kernel 更新操作', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.sm),
              _row(
                context,
                'Operation ID',
                (op['id'] ?? op['operationId'] ?? '').toString(),
              ),
              _row(context, '状态', (op['status'] ?? '').toString()),
              _row(
                context,
                '版本',
                (op['targetVersion'] ?? update['version'] ?? '').toString(),
              ),
              _row(context, '当前步骤', (op['currentStep'] ?? '-').toString()),
              if ((op['errorCode'] ?? '').toString().isNotEmpty)
                _row(context, '错误', op['errorCode'].toString()),
              Row(
                children: [
                  TextButton.icon(
                    onPressed: _refreshExtensionOperation,
                    icon: const Icon(Icons.refresh),
                    label: const Text('刷新状态'),
                  ),
                  TextButton.icon(
                    onPressed: _showVersionHistory,
                    icon: const Icon(Icons.history),
                    label: const Text('版本历史'),
                  ),
                ],
              ),
            ],
            if (steps.isNotEmpty) ...[
              const Divider(),
              Text('执行步骤', style: AppTypography.cardTitle(context)),
              ...steps.map(
                (step) => ListTile(
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  leading: Icon(_stepIcon((step['status'] ?? '').toString())),
                  title: Text((step['name'] ?? '').toString()),
                  subtitle: Text(
                    '${step['status'] ?? ''}${(step['errorCode'] ?? '').toString().isEmpty ? '' : ' · ${step['errorCode']}'}',
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Future<void> _checkExtensionUpdate() async {
    setState(() => _extensionBusy = true);
    try {
      final result = await ref
          .read(extensionServiceProvider)
          .checkExtensionUpdate(_selectedExtension);
      final rawUpdate = result['update'];
      final update = rawUpdate is Map
          ? Map<String, dynamic>.from(rawUpdate)
          : <String, dynamic>{};
      if (!mounted) return;
      setState(() {
        _extensionUpdate = update;
        _extensionDownload = const {};
        _extensionPreview = const {};
        _extensionOperation = const {};
        _extensionConfirmed = false;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            update.isEmpty ? '该扩展已是最新版本' : '发现扩展更新 v${update['version']}',
          ),
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('检查扩展更新失败: $e')));
      }
    } finally {
      if (mounted) setState(() => _extensionBusy = false);
    }
  }

  Future<void> _downloadExtensionUpdate() async {
    final version = (_extensionUpdate['version'] ?? '').toString();
    if (version.isEmpty) return;
    setState(() {
      _extensionBusy = true;
      _extensionPreview = const {};
      _extensionConfirmed = false;
    });
    try {
      final service = ref.read(extensionServiceProvider);
      var download = await service.startExtensionUpdateDownload(
        _selectedExtension,
        version,
      );
      if (mounted) setState(() => _extensionDownload = download);
      for (var attempt = 0; attempt < 720; attempt += 1) {
        final payload = await service.extensionUpdateDownload(
          (download['downloadId'] ?? '').toString(),
        );
        final next = payload['download'];
        if (next is Map) {
          download = Map<String, dynamic>.from(next);
          if (mounted) setState(() => _extensionDownload = download);
        }
        final status = (download['status'] ?? '').toString();
        if (status == 'completed') break;
        if (status == 'failed' || status == 'cancelled') {
          throw StateError(
            (download['error'] ?? '').toString().isEmpty
                ? '扩展包下载状态 $status'
                : download['error'].toString(),
          );
        }
        await Future<void>.delayed(const Duration(milliseconds: 500));
      }
      if ((download['status'] ?? '').toString() != 'completed') {
        throw StateError('扩展包下载等待超时');
      }
      final previewResult = await service.previewExtensionUpdate(
        _selectedExtension,
        (download['downloadId'] ?? '').toString(),
      );
      final rawPreview = previewResult['preview'];
      if (rawPreview is! Map) throw StateError('后端未返回 Kernel 包预览');
      if (mounted) {
        setState(() {
          _extensionPreview = Map<String, dynamic>.from(rawPreview);
          _extensionConfirmed = false;
        });
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('扩展包下载或预览失败: $e')));
      }
    } finally {
      if (mounted) setState(() => _extensionBusy = false);
    }
  }

  Future<void> _cancelExtensionDownload() async {
    final downloadId = (_extensionDownload['downloadId'] ?? '').toString();
    if (downloadId.isEmpty) return;
    try {
      await ref
          .read(extensionServiceProvider)
          .cancelExtensionUpdateDownload(downloadId);
      if (mounted) {
        setState(() {
          _extensionDownload = {..._extensionDownload, 'status': 'cancelled'};
        });
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('取消下载失败: $e')));
      }
    }
  }

  Future<void> _installExtensionPreview() async {
    if (_extensionPreview.isEmpty || !_extensionConfirmed) return;
    await _withExtensionBusy(() async {
      final confirmations = <String, bool>{};
      for (final item
          in (_extensionPreview['requiredConfirmations'] as List?) ??
              const []) {
        final key = item.toString();
        if (key.isNotEmpty) confirmations[key] = true;
      }
      final result = await ref
          .read(extensionServiceProvider)
          .commitPackagePreview(
            _extensionPreview,
            confirmations,
            expectedExtensionId: _selectedExtension,
          );
      final operationId = (result['operationId'] ?? '').toString();
      if (operationId.isEmpty) throw StateError('后端未返回更新操作 ID');
      if (mounted) setState(() => _extensionOperation = {'id': operationId});
      await _refreshExtensionOperation();
      await _loadExtensions();
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('扩展更新已提交并由 Kernel 完成')));
      }
    });
  }

  Future<void> _refreshExtensionOperation() async {
    final operationId =
        (_extensionOperation['id'] ?? _extensionOperation['operationId'] ?? '')
            .toString();
    if (operationId.isEmpty) return;
    final op = await ref
        .read(extensionServiceProvider)
        .kernelPackageOperation(operationId);
    if (mounted) setState(() => _extensionOperation = op);
  }

  Future<void> _showVersionHistory() async {
    final versions = await ref
        .read(extensionServiceProvider)
        .kernelPackageVersions(_selectedExtension);
    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(22)),
      ),
      builder: (sheetContext) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            Text('不可变版本历史', style: AppTypography.cardTitle(sheetContext)),
            SizedBox(height: AppSpacing.md),
            if (versions.isEmpty)
              const AmitiaEmptyState(icon: Icons.history, title: '暂无版本历史')
            else
              ...versions.map(
                (item) => ListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text('v${item['version'] ?? ''}'),
                  subtitle: Text(
                    item['active'] == true
                        ? '当前版本'
                        : (item['versionState'] ?? '').toString(),
                  ),
                  trailing: item['active'] == true
                      ? null
                      : OutlinedButton(
                          onPressed: () {
                            Navigator.pop(sheetContext);
                            _rollbackVersion(item);
                          },
                          child: const Text('回滚'),
                        ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Future<void> _rollbackVersion(Map<String, dynamic> version) async {
    final targetVersion = (version['version'] ?? '').toString();
    if (targetVersion.isEmpty) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('真实回滚'),
        content: Text('将从当前版本回滚到 v$targetVersion，并恢复文件、配置、权限和迁移状态。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('确认回滚'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    await _withExtensionBusy(() async {
      final service = ref.read(extensionServiceProvider);
      final preview = await service.previewPackageRollback(
        _selectedExtension,
        targetVersion,
      );
      if (preview['installable'] != true) throw StateError('该版本当前不可回滚');
      if (preview['manualRequired'] == true) {
        throw StateError((preview['manualReason'] ?? '该回滚需要人工恢复').toString());
      }
      final confirmations = <String, bool>{};
      for (final item
          in (preview['requiredConfirmations'] as List?) ?? const []) {
        final key = item.toString();
        if (key.isNotEmpty) confirmations[key] = true;
      }
      final confirmation = await service.confirmPackageRollback(
        _selectedExtension,
        targetVersion,
        preview,
        confirmations,
      );
      final token = (confirmation['confirmationToken'] ?? '').toString();
      if (token.isEmpty) throw StateError('回滚确认令牌缺失');
      final result = await service.executePackageRollback(
        _selectedExtension,
        targetVersion,
        token,
      );
      final operationId = (result['operationId'] ?? '').toString();
      if (operationId.isNotEmpty) {
        setState(() => _extensionOperation = {'id': operationId});
        await _refreshExtensionOperation();
      }
      await _loadExtensions();
    });
  }

  Future<void> _withExtensionBusy(Future<void> Function() action) async {
    if (mounted) setState(() => _extensionBusy = true);
    try {
      await action();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('扩展更新操作失败: $e')));
      }
    } finally {
      if (mounted) setState(() => _extensionBusy = false);
    }
  }

  String _formatBytes(dynamic value) {
    final bytes = value is num
        ? value.toDouble()
        : double.tryParse(value?.toString() ?? '') ?? 0;
    if (bytes <= 0) return '-';
    if (bytes < 1024) return '${bytes.toStringAsFixed(0)} B';
    if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }

  IconData _stepIcon(String status) {
    switch (status.toLowerCase()) {
      case 'completed':
      case 'success':
        return Icons.check_circle_outline;
      case 'failed':
        return Icons.error_outline;
      case 'running':
        return Icons.sync;
      default:
        return Icons.radio_button_unchecked;
    }
  }

  Widget _row(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          SizedBox(
            width: 110,
            child: Text(label, style: AppTypography.label(context)),
          ),
          Expanded(
            child: Text(
              value.isEmpty ? '-' : value,
              style: AppTypography.bodySmall(context),
            ),
          ),
        ],
      ),
    );
  }

  void _showJson(Map<String, dynamic> item) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('检查详情'),
        content: SelectableText(
          const JsonEncoder.withIndent('  ').convert(item),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('关闭'),
          ),
        ],
      ),
    );
  }
}
