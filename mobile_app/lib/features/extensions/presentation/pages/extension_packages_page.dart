import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/artifact/artifact_providers.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/services/error_utils.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ExtensionPackagesPage extends ConsumerStatefulWidget {
  const ExtensionPackagesPage({super.key});

  @override
  ConsumerState<ExtensionPackagesPage> createState() => _ExtensionPackagesPageState();
}

class _ExtensionPackagesPageState extends ConsumerState<ExtensionPackagesPage> {
  List<Map<String, dynamic>> _packages = const [];
  bool _loading = true;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadPackages();
  }

  Future<void> _loadPackages() async {
    if (mounted) setState(() { _loading = true; _error = null; });
    try {
      final data = await ref.read(extensionServiceProvider).kernelExtensions();
      if (mounted) setState(() { _packages = data; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = safeErrorMessage(e); _loading = false; });
    }
  }

  Future<Dio> _dio() async {
    final availability = await ref.read(backendConnectionProvider.future);
    if (availability is! BackendConnectionAvailable) throw StateError('后端当前不可用');
    return createAuthenticatedDio(availability.config);
  }

  void _toast(String message, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), backgroundColor: error ? context.error : null),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '扩展包',
        showBackButton: true,
        fallbackRoute: AppRoutes.extensions,
        actions: [
          AmitiaIconButton(icon: Icons.refresh, onPressed: _busy ? null : _loadPackages, tooltip: '刷新'),
          AmitiaIconButton(icon: Icons.download_outlined, onPressed: _busy ? null : _showInstallLocalSheet, tooltip: '安装本地包'),
        ],
      ),
      body: SafeArea(top: false, child: _body()),
      floatingActionButton: _loading
          ? null
          : FloatingActionButton(
              onPressed: _busy ? null : _showInstallLocalSheet,
              backgroundColor: context.accentPrimary,
              child: _busy
                  ? const SizedBox(width: 22, height: 22, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
                  : const Icon(Icons.add, color: Colors.white),
            ),
    );
  }

  Widget _body() {
    if (_loading) return const AmitiaLoadingState(message: '加载已安装扩展...');
    if (_error != null) return AmitiaErrorState(message: '加载失败: $_error', onRetry: _loadPackages);
    if (_packages.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.inventory_2_outlined,
        title: '暂无扩展包',
        subtitle: '安装 Manifest v2 扩展包后会显示在这里',
        actionText: '安装本地包',
        onAction: _showInstallLocalSheet,
      );
    }
    return RefreshIndicator(
      onRefresh: _loadPackages,
      child: ListView.separated(
        padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
        itemCount: _packages.length,
        separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
        itemBuilder: (context, index) => _buildPackageCard(_packages[index]),
      ),
    );
  }

  Widget _buildPackageCard(Map<String, dynamic> pkg) {
    final id = (pkg['extensionId'] ?? '').toString();
    final version = (pkg['version'] ?? '').toString();
    final state = (pkg['state'] ?? '').toString();
    final enablement = (pkg['enablement'] ?? '').toString();
    final enabled = enablement == 'enabled';
    final statusText = enabled ? '已启用' : (state.isEmpty ? '已停用' : state);

    return AmitiaCard(
      onTap: () => _showExtensionDetail(pkg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                child: Icon(Icons.extension_outlined, size: 22, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(id.isEmpty ? '未命名扩展' : id, style: AppTypography.cardTitle(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                    const SizedBox(height: 4),
                    Text(version.isEmpty ? '版本未知' : 'v$version', style: AppTypography.caption(context)),
                  ],
                ),
              ),
              AmitiaStatusBadge(label: statusText, type: enabled ? BadgeType.success : BadgeType.neutral),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              _MiniButton(
                label: enabled ? '停用' : '启用',
                icon: enabled ? Icons.pause_circle_outline : Icons.play_circle_outline,
                color: enabled ? context.warning : context.success,
                onTap: () => _toggleExtension(pkg, !enabled),
              ),
              _MiniButton(
                label: '详情',
                icon: Icons.info_outline,
                color: context.accentPrimary,
                onTap: () => _showExtensionDetail(pkg),
              ),
              _MiniButton(
                label: '卸载',
                icon: Icons.delete_outline,
                color: context.error,
                onTap: () => _showUninstallConfirm(pkg),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _toggleExtension(Map<String, dynamic> pkg, bool enabled) async {
    if (_busy) return;
    final id = (pkg['extensionId'] ?? '').toString();
    if (id.isEmpty) return;
    setState(() => _busy = true);
    try {
      await ref.read(extensionServiceProvider).setKernelExtensionEnabled(id, enabled);
      await _loadPackages();
      _toast('$id 已${enabled ? '启用' : '停用'}');
    } catch (e) {
      _toast('操作失败: ${safeErrorMessage(e)}', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _showExtensionDetail(Map<String, dynamic> pkg) async {
    final id = (pkg['extensionId'] ?? '').toString();
    if (id.isEmpty) return;
    try {
      final detail = await ref.read(extensionServiceProvider).kernelExtension(id);
      if (!mounted) return;
      showDialog(context: context, builder: (dialogContext) => _ExtensionDetailDialog(detail: detail));
    } catch (e) {
      _toast('读取扩展详情失败: ${safeErrorMessage(e)}', error: true);
    }
  }

  Future<void> _showInstallLocalSheet() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['amitiax', 'zip'],
      withData: false,
    );
    if (picked == null || picked.files.isEmpty) return;
    final file = picked.files.first;
    if (file.path == null || file.path!.isEmpty) {
      _toast('无法读取所选文件', error: true);
      return;
    }

    Dio? dio;
    if (mounted) setState(() => _busy = true);
    try {
      dio = await _dio();
      final previewResponse = await dio.post(
        '/api/extensions/packages/artifacts',
        data: FormData.fromMap({
          'scopeType': 'global',
          'scopeId': '',
          'file': await MultipartFile.fromFile(file.path!, filename: file.name),
        }),
      );
      dynamic raw = previewResponse.data;
      if (raw is! Map || raw['preview'] is! Map) throw StateError('后端未返回扩展包预览');
      final preview = Map<String, dynamic>.from(raw['preview'] as Map);
      if (!mounted) return;

      final accepted = await showModalBottomSheet<bool>(
        context: context,
        isScrollControlled: true,
        backgroundColor: context.surfacePrimary,
        shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
        builder: (sheetContext) => _PackagePreviewSheet(fileName: file.name, preview: preview),
      );
      if (accepted != true) return;

      final sessionId = (preview['sessionId'] ?? '').toString();
      if (sessionId.isEmpty) throw StateError('预览会话无效');
      final confirmations = _buildInstallConfirmations(preview);
      final confirmResponse = await dio.post(
        '/api/extensions/packages/previews/${Uri.encodeComponent(sessionId)}/confirm',
        data: {
          'scopeType': (preview['scopeType'] ?? 'global').toString(),
          'scopeId': (preview['scopeId'] ?? '').toString(),
          'confirmations': confirmations,
        },
      );
      dynamic confirmed = confirmResponse.data;
      if (confirmed is Map && confirmed['data'] is Map) confirmed = confirmed['data'];
      if (confirmed is! Map) throw StateError('安装确认失败');
      final token = (confirmed['confirmationToken'] ?? '').toString();
      if (token.isEmpty) throw StateError('安装确认令牌缺失');

      final isUpdate = (preview['currentVersion'] ?? '').toString().isNotEmpty;
      final extensionId = (preview['id'] ?? '').toString();
      final operationResponse = await dio.post(
        isUpdate ? '/api/extensions/packages/operations/update' : '/api/extensions/packages/operations/install',
        data: {
          'sessionId': sessionId,
          'scopeType': (preview['scopeType'] ?? 'global').toString(),
          'scopeId': (preview['scopeId'] ?? '').toString(),
          'confirmationToken': token,
          if (isUpdate && extensionId.isNotEmpty) 'expectedExtensionId': extensionId,
          'idempotencyKey': 'mobile-package-${DateTime.now().microsecondsSinceEpoch}',
        },
      );
      dynamic operation = operationResponse.data;
      if (operation is Map && operation['data'] is Map) operation = operation['data'];
      final operationId = operation is Map ? (operation['operationId'] ?? '').toString() : '';
      await _loadPackages();
      _toast(operationId.isEmpty ? '扩展包操作已提交' : '扩展包操作已提交 · $operationId');
    } catch (e) {
      _toast('安装失败: ${safeErrorMessage(e)}', error: true);
    } finally {
      dio?.close(force: true);
      if (mounted) setState(() => _busy = false);
    }
  }

  Map<String, bool> _buildInstallConfirmations(Map<String, dynamic> preview) {
    final result = <String, bool>{};
    for (final value in (preview['capabilityConfirmations'] as List?) ?? const []) {
      final key = value.toString();
      if (key.isNotEmpty) result[key] = true;
    }
    final signature = preview['signature'];
    final signatureStatus = signature is Map ? (signature['status'] ?? '').toString() : '';
    if (signatureStatus == 'unsigned') result['confirm.unsigned_dev'] = true;
    final scriptCount = (preview['scripts'] as num?)?.toInt() ?? 0;
    if (scriptCount > 0) result['confirm.scripts'] = true;
    if ((preview['currentVersion'] ?? '').toString().isNotEmpty) result['confirm.version_change'] = true;
    if (((preview['highRiskCapabilities'] as List?) ?? const []).isNotEmpty) result['confirm.permission_escalation'] = true;
    if (preview['upgradeDiff'] is Map) {
      final diff = preview['upgradeDiff'] as Map;
      if (diff['signerChanged'] == true) result['confirm.signer_change'] = true;
      if (diff['configMigrationRequired'] == true) result['confirm.config_migration'] = true;
    }
    return result;
  }

  Future<void> _showUninstallConfirm(Map<String, dynamic> pkg) async {
    if (_busy) return;
    final id = (pkg['extensionId'] ?? '').toString();
    if (id.isEmpty) return;
    setState(() => _busy = true);
    try {
      final svc = ref.read(extensionServiceProvider);
      final preview = await svc.previewKernelUninstall(id);
      if (!mounted) return;
      final dependents = ((preview['dependents'] as List?) ?? const []).map((e) => e.toString()).toList(growable: false);
      final required = ((preview['requiredConfirmations'] as List?) ?? const []).map((e) => e.toString()).toList(growable: false);
      final allowed = preview['uninstallable'] != false;
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          backgroundColor: dialogContext.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
          title: Text('卸载扩展', style: AppTypography.cardTitle(dialogContext)),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('扩展：$id', style: AppTypography.bodySmall(dialogContext)),
              const SizedBox(height: 6),
              Text('当前版本：${preview['currentVersion'] ?? pkg['version'] ?? ''}', style: AppTypography.label(dialogContext)),
              Text('制品策略：${preview['artifactPolicy'] ?? 'unknown'}', style: AppTypography.label(dialogContext)),
              if (dependents.isNotEmpty) ...[
                const SizedBox(height: 8),
                Text('依赖此扩展：${dependents.join('、')}', style: AppTypography.label(dialogContext).copyWith(color: dialogContext.warning)),
              ],
              if (!allowed) ...[
                const SizedBox(height: 8),
                Text('后端判定当前不可卸载。', style: AppTypography.label(dialogContext).copyWith(color: dialogContext.error)),
              ],
            ],
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
            FilledButton(
              onPressed: allowed ? () => Navigator.pop(dialogContext, true) : null,
              child: const Text('确认卸载'),
            ),
          ],
        ),
      );
      if (confirmed != true) return;
      final confirmation = await svc.confirmKernelUninstall(id, {for (final key in required) key: true});
      final token = (confirmation['confirmationToken'] ?? '').toString();
      if (token.isEmpty) throw StateError('卸载确认令牌缺失');
      final result = await svc.uninstallKernelExtension(id, token);
      await _loadPackages();
      final operationId = (result['operationId'] ?? '').toString();
      _toast(operationId.isEmpty ? '$id 卸载操作已提交' : '$id 卸载操作已提交 · $operationId');
    } catch (e) {
      _toast('卸载失败: ${safeErrorMessage(e)}', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}

class _MiniButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color color;
  final VoidCallback onTap;

  const _MiniButton({required this.label, required this.icon, required this.color, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
        decoration: BoxDecoration(color: color.withValues(alpha: 0.1), borderRadius: AppRadius.brTag),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 15, color: color),
            const SizedBox(width: 5),
            Text(label, style: TextStyle(fontSize: 13, color: color, fontWeight: FontWeight.w500)),
          ],
        ),
      ),
    );
  }
}

class _PackagePreviewSheet extends StatelessWidget {
  final String fileName;
  final Map<String, dynamic> preview;

  const _PackagePreviewSheet({required this.fileName, required this.preview});

  @override
  Widget build(BuildContext context) {
    final errors = ((preview['errors'] as List?) ?? const []).map((e) => e.toString()).toList(growable: false);
    final warnings = ((preview['warnings'] as List?) ?? const []).map((e) => e.toString()).toList(growable: false);
    final risks = ((preview['risks'] as List?) ?? const []).whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
    final compatible = preview['compatible'] != false && errors.isEmpty;
    final currentVersion = (preview['currentVersion'] ?? '').toString();
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
            const SizedBox(height: 20),
            Text(currentVersion.isEmpty ? '安装扩展包' : '更新扩展包', style: AppTypography.pageTitle(context)),
            const SizedBox(height: 14),
            _DetailRow(label: '文件', value: fileName),
            _DetailRow(label: '扩展 ID', value: (preview['id'] ?? '').toString()),
            _DetailRow(label: '名称', value: (preview['name'] ?? '').toString()),
            _DetailRow(label: '版本', value: (preview['version'] ?? '').toString()),
            if (currentVersion.isNotEmpty) _DetailRow(label: '当前版本', value: currentVersion),
            _DetailRow(label: '签名', value: preview['signature'] is Map ? ((preview['signature'] as Map)['status'] ?? 'unknown').toString() : 'unknown'),
            _DetailRow(label: '兼容性', value: (preview['compatibility'] ?? (compatible ? 'compatible' : 'blocked')).toString()),
            if (warnings.isNotEmpty) ...[
              const SizedBox(height: 10),
              Text('警告', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: 4),
              ...warnings.take(5).map((item) => Text('• $item', style: AppTypography.label(context))),
            ],
            if (risks.isNotEmpty) ...[
              const SizedBox(height: 10),
              Text('风险', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: 4),
              ...risks.take(5).map((item) => Text('• ${item['message'] ?? item['code'] ?? item}', style: AppTypography.label(context).copyWith(color: context.warning))),
            ],
            if (errors.isNotEmpty) ...[
              const SizedBox(height: 10),
              ...errors.take(5).map((item) => Text('• $item', style: AppTypography.label(context).copyWith(color: context.error))),
            ],
            const SizedBox(height: 18),
            Row(
              children: [
                Expanded(child: OutlinedButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消'))),
                const SizedBox(width: 10),
                Expanded(child: FilledButton(onPressed: compatible ? () => Navigator.pop(context, true) : null, child: Text(currentVersion.isEmpty ? '确认安装' : '确认更新'))),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _ExtensionDetailDialog extends StatelessWidget {
  final Map<String, dynamic> detail;

  const _ExtensionDetailDialog({required this.detail});

  @override
  Widget build(BuildContext context) {
    final modules = ((detail['modules'] as List?) ?? const []).whereType<Map>().toList(growable: false);
    final contributions = ((detail['contributions'] as List?) ?? const []).whereType<Map>().toList(growable: false);
    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      title: Text((detail['extensionId'] ?? '扩展详情').toString(), style: AppTypography.cardTitle(context)),
      content: SizedBox(
        width: double.maxFinite,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _DetailRow(label: '版本', value: (detail['version'] ?? '').toString()),
              _DetailRow(label: '状态', value: (detail['state'] ?? '').toString()),
              _DetailRow(label: '启用状态', value: (detail['enablement'] ?? '').toString()),
              _DetailRow(label: '安装 ID', value: (detail['installationId'] ?? '').toString()),
              _DetailRow(label: 'Generation', value: (detail['generation'] ?? '').toString()),
              _DetailRow(label: '模块数量', value: modules.length.toString()),
              _DetailRow(label: '贡献数量', value: contributions.length.toString()),
              if (modules.isNotEmpty) ...[
                const SizedBox(height: 8),
                Text('模块', style: AppTypography.sectionTitle(context)),
                ...modules.take(12).map((module) => Padding(
                      padding: const EdgeInsets.only(top: 5),
                      child: Text('${module['id'] ?? ''} · ${module['type'] ?? ''} · ${module['runtime'] ?? ''}', style: AppTypography.label(context)),
                    )),
              ],
            ],
          ),
        ),
      ),
      actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('关闭'))],
    );
  }
}

class _DetailRow extends StatelessWidget {
  final String label;
  final String value;

  const _DetailRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(width: 84, child: Text(label, style: AppTypography.label(context))),
          Expanded(child: Text(value.isEmpty ? '-' : value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }
}
