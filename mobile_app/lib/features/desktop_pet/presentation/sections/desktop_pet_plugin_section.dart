import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../infrastructure/desktop_pet_package_picker.dart';
import '../controllers/desktop_pet_plugin_controller_provider.dart';
import '../../infrastructure/desktop_pet_plugin_dto.dart';

class DesktopPetPluginSection extends ConsumerStatefulWidget {
  const DesktopPetPluginSection({super.key});

  @override
  ConsumerState<DesktopPetPluginSection> createState() =>
      _DesktopPetPluginSectionState();
}

class _DesktopPetPluginSectionState
    extends ConsumerState<DesktopPetPluginSection> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(desktopPetPluginControllerProvider.notifier).load();
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(desktopPetPluginControllerProvider);
    final controller = ref.read(desktopPetPluginControllerProvider.notifier);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            const Expanded(child: AmitiaSectionHeader(title: '已安装桌宠插件')),
            if (!state.loading)
              TextButton.icon(
                onPressed: state.installing
                    ? null
                    : () => _pickAndPreviewPackage(
                        context,
                        controller,
                        pickPackage: ref.read(desktopPetPackagePickerProvider),
                      ),
                icon: const Icon(Icons.add, size: 16),
                label: Text(state.installing ? '安装中...' : '安装'),
              ),
          ],
        ),
        SizedBox(height: AppSpacing.sm),
        _buildContent(context, state, controller),
      ],
    );
  }

  Widget _buildContent(
    BuildContext context,
    DesktopPetPluginState state,
    DesktopPetPluginController controller,
  ) {
    if (state.loading) {
      return Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        child: AmitiaCard(
          child: Center(
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: 28),
              child: AmitiaLoadingState(message: '加载插件中...'),
            ),
          ),
        ),
      );
    }

    if (state.error != null && state.plugins.isEmpty) {
      return Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        child: AmitiaCard(
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: 24),
            child: AmitiaErrorState(
              message: '加载失败',
              onRetry: () => controller.load(),
            ),
          ),
        ),
      );
    }

    if (state.plugins.isEmpty) {
      return Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        child: AmitiaCard(
          child: AmitiaEmptyState(
            icon: Icons.extension_outlined,
            title: '尚未安装桌宠插件',
            subtitle: '安装后即可在此管理',
            actionText: '安装插件',
            onAction: () => _pickAndPreviewPackage(
              context,
              controller,
              pickPackage: ref.read(desktopPetPackagePickerProvider),
            ),
          ),
        ),
      );
    }

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: Column(
        children: [
          if (state.refreshing)
            Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.xs),
              child: LinearProgressIndicator(minHeight: 2),
            ),
          if (state.error != null)
            Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.xs),
              child: AmitiaCard(
                backgroundColor: context.error.withValues(alpha: 0.08),
                child: Padding(
                  padding: EdgeInsets.all(AppSpacing.sm),
                  child: Row(
                    children: [
                      Icon(
                        Icons.warning_amber_rounded,
                        size: 16,
                        color: context.error,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          state.error!,
                          style: AppTypography.caption(
                            context,
                          ).copyWith(color: context.error),
                        ),
                      ),
                      GestureDetector(
                        onTap: () => controller.refresh(),
                        child: Icon(
                          Icons.refresh,
                          size: 16,
                          color: context.accentPrimary,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          AmitiaCard(
            padding: EdgeInsets.zero,
            child: Column(
              children: [
                for (int i = 0; i < state.plugins.length; i++) ...[
                  _buildPluginItem(
                    context,
                    state.plugins[i],
                    state,
                    controller,
                  ),
                  if (i < state.plugins.length - 1)
                    Divider(height: 1, color: context.borderSecondary),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPluginItem(
    BuildContext context,
    DesktopPetPluginSummaryRef plugin,
    DesktopPetPluginState state,
    DesktopPetPluginController controller,
  ) {
    final isOperating = controller.hasOperation(plugin.pluginId);
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => _showDetailSheet(context, plugin, controller),
      child: Padding(
        padding: EdgeInsets.symmetric(
          horizontal: AppSpacing.cardPadding,
          vertical: 10,
        ),
        child: Row(
          children: [
            Container(
              width: 36,
              height: 36,
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.extension_outlined,
                size: 18,
                color: context.accentPrimary,
              ),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    plugin.name.isNotEmpty ? plugin.name : plugin.pluginId,
                    style: AppTypography.body(context),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 2),
                  Text(
                    'v${plugin.version}',
                    style: AppTypography.label(context),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
            SizedBox(width: AppSpacing.xs),
            AmitiaStatusBadge(
              label: plugin.enabled ? '已启用' : '已禁用',
              type: plugin.enabled ? BadgeType.success : BadgeType.neutral,
            ),
            SizedBox(width: AppSpacing.sm),
            if (isOperating)
              SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: context.accentPrimary,
                ),
              )
            else
              Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showDetailSheet(
    BuildContext context,
    DesktopPetPluginSummaryRef plugin,
    DesktopPetPluginController controller,
  ) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) => _PluginDetailSheet(
        hostContext: context,
        pluginId: plugin.pluginId,
        extensionId: plugin.extensionId,
        name: plugin.name,
        description: plugin.description,
        version: plugin.version,
        enabled: plugin.enabled,
        installState: plugin.installState,
        controller: controller,
        pickPackage: ref.read(desktopPetPackagePickerProvider),
      ),
    );
  }
}

Future<void> _pickAndPreviewPackage(
  BuildContext context,
  DesktopPetPluginController controller, {
  DesktopPetPackagePicker? pickPackage,
  String expectedExtensionId = '',
}) async {
  String? path;
  try {
    path = await (pickPackage ?? pickDesktopPetPackagePath)();
  } catch (error) {
    if (!context.mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(error.toString())));
    return;
  }
  if (path == null || !context.mounted) return;

  final preview = await controller.previewPackage(
    path,
    expectedExtensionId: expectedExtensionId,
  );
  if (!context.mounted) return;
  if (preview == null) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(controller.current.error ?? '桌宠插件包预览失败')),
    );
    return;
  }

  final accepted = await showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (_) => _DesktopPetPackagePreviewDialog(
      fileName: path!.split(RegExp(r'[/\\]')).last,
      preview: preview,
      isUpdate:
          expectedExtensionId.trim().isNotEmpty ||
          (preview['currentVersion'] ?? '').toString().trim().isNotEmpty,
    ),
  );
  if (accepted != true || !context.mounted) return;

  final ok = await controller.commitPackagePreview(
    preview,
    expectedExtensionId: expectedExtensionId,
  );
  if (!context.mounted) return;
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      content: Text(
        ok
            ? (expectedExtensionId.trim().isEmpty ? '桌宠插件安装成功' : '桌宠插件更新成功')
            : (controller.current.error ?? '桌宠插件安装失败'),
      ),
    ),
  );
}

class _DesktopPetPackagePreviewDialog extends StatefulWidget {
  final String fileName;
  final Map<String, dynamic> preview;
  final bool isUpdate;

  const _DesktopPetPackagePreviewDialog({
    required this.fileName,
    required this.preview,
    required this.isUpdate,
  });

  @override
  State<_DesktopPetPackagePreviewDialog> createState() =>
      _DesktopPetPackagePreviewDialogState();
}

class _DesktopPetPackagePreviewDialogState
    extends State<_DesktopPetPackagePreviewDialog> {
  bool _acknowledged = false;

  bool get _requiresAcknowledgement {
    final preview = widget.preview;
    final signature = preview['signature'];
    final signatureStatus = signature is Map
        ? (signature['status'] ?? '').toString()
        : '';
    final diff = preview['upgradeDiff'];
    return signatureStatus == 'unsigned' ||
        ((preview['scripts'] as num?)?.toInt() ?? 0) > 0 ||
        (((preview['highRiskCapabilities'] as List?) ?? const []).isNotEmpty) ||
        (((preview['warnings'] as List?) ?? const []).isNotEmpty) ||
        (((preview['capabilityConfirmations'] as List?) ?? const [])
            .isNotEmpty) ||
        (diff is Map &&
            (diff['signerChanged'] == true ||
                diff['configMigrationRequired'] == true));
  }

  @override
  Widget build(BuildContext context) {
    final preview = widget.preview;
    final name = (preview['name'] ?? preview['id'] ?? '桌宠插件').toString();
    final version = (preview['version'] ?? '').toString();
    final extensionId = (preview['id'] ?? '').toString();
    final signature = preview['signature'];
    final signatureStatus = signature is Map
        ? (signature['status'] ?? '').toString()
        : '';
    final warnings = ((preview['warnings'] as List?) ?? const [])
        .map((item) => item.toString())
        .toList();
    final errors = ((preview['errors'] as List?) ?? const [])
        .map((item) => item.toString())
        .toList();
    final risks = ((preview['highRiskCapabilities'] as List?) ?? const [])
        .map((item) => item.toString())
        .toList();
    return AlertDialog(
      title: Text(widget.isUpdate ? '确认更新桌宠插件' : '确认安装桌宠插件'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(name, style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 4),
            Text('文件：${widget.fileName}'),
            Text('扩展 ID：$extensionId'),
            Text('版本：$version'),
            Text('管理目标：${preview['managementTarget'] ?? ''}'),
            Text('签名：${signatureStatus.isEmpty ? 'unknown' : signatureStatus}'),
            Text('兼容性：${preview['compatible'] == true ? '通过' : '不通过'}'),
            Text(
              '资源：${(preview['assets'] as num?)?.toInt() ?? 0} 个，'
              '文件 ${(preview['fileCount'] as num?)?.toInt() ?? 0} 个',
            ),
            if (risks.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text(
                '高风险能力：${risks.join('、')}',
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ],
            for (final warning in warnings)
              Text(
                '警告：$warning',
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            for (final error in errors)
              Text(
                '错误：$error',
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            const SizedBox(height: 8),
            ExpansionTile(
              tilePadding: EdgeInsets.zero,
              title: const Text('查看完整预览信息'),
              children: [
                SelectableText(
                  const JsonEncoder.withIndent('  ').convert(preview),
                ),
              ],
            ),
            if (_requiresAcknowledgement)
              CheckboxListTile(
                contentPadding: EdgeInsets.zero,
                value: _acknowledged,
                onChanged: (value) =>
                    setState(() => _acknowledged = value == true),
                title: const Text('我已查看签名、权限、风险和完整预览信息，并确认继续'),
              ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context, false),
          child: const Text('取消'),
        ),
        FilledButton(
          onPressed:
              (_requiresAcknowledgement && !_acknowledged) || errors.isNotEmpty
              ? null
              : () => Navigator.pop(context, true),
          child: Text(widget.isUpdate ? '确认更新' : '确认安装'),
        ),
      ],
    );
  }
}

class _PluginDetailSheet extends StatefulWidget {
  final BuildContext hostContext;
  final String pluginId;
  final String extensionId;
  final String name;
  final String description;
  final String version;
  final bool enabled;
  final String installState;
  final DesktopPetPluginController controller;
  final DesktopPetPackagePicker pickPackage;

  const _PluginDetailSheet({
    required this.hostContext,
    required this.pluginId,
    required this.extensionId,
    required this.name,
    required this.description,
    required this.version,
    required this.enabled,
    required this.installState,
    required this.controller,
    required this.pickPackage,
  });

  @override
  State<_PluginDetailSheet> createState() => _PluginDetailSheetState();
}

class _PluginDetailSheetState extends State<_PluginDetailSheet> {
  DesktopPetPluginDetail? _detail;
  bool _loading = false;
  String? _detailError;

  @override
  void initState() {
    super.initState();
    _loadDetail();
  }

  Future<void> _loadDetail() async {
    setState(() {
      _loading = true;
      _detailError = null;
    });
    try {
      final d = await widget.controller.detail(widget.pluginId);
      if (mounted) {
        setState(() {
          _detail = d;
          _loading = false;
        });
      }
    } catch (error) {
      if (mounted) {
        setState(() {
          _detailError = error.toString();
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final hasOp = widget.controller.hasOperation(widget.pluginId);

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(widget.name, style: AppTypography.pageTitle(context)),
            const SizedBox(height: 4),
            Text(widget.description, style: AppTypography.caption(context)),
            const SizedBox(height: 16),
            _buildInfoRow(context, '插件ID', widget.pluginId),
            _buildInfoRow(context, '版本', 'v${widget.version}'),
            _buildInfoRow(context, '状态', widget.enabled ? '已启用' : '已禁用'),
            _buildInfoRow(
              context,
              '安装状态',
              _installStateLabel(widget.installState),
            ),
            if (_loading)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 8),
                child: Center(
                  child: SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: context.accentPrimary,
                    ),
                  ),
                ),
              ),
            if (_detail != null && _detail!.permissionSummary != null) ...[
              const SizedBox(height: 8),
              Text('权限摘要', style: AppTypography.label(context)),
              const SizedBox(height: 4),
              if (_detail!.permissionSummary!.declared.isNotEmpty)
                Text(
                  '声明: ${_detail!.permissionSummary!.declared.join(", ")}',
                  style: AppTypography.caption(context),
                ),
              if (_detail!.permissionSummary!.granted.isNotEmpty)
                Text(
                  '授权: ${_detail!.permissionSummary!.granted.join(", ")}',
                  style: AppTypography.caption(context),
                ),
            ],
            if (_detailError != null) ...[
              const SizedBox(height: 8),
              SelectableText(
                _detailError!,
                style: AppTypography.caption(
                  context,
                ).copyWith(color: context.error),
              ),
            ],
            const SizedBox(height: 20),
            Row(
              children: [
                Expanded(
                  child: widget.enabled
                      ? AmitiaButtonOutline(
                          label: hasOp ? '处理中...' : '禁用',
                          onPressed: hasOp
                              ? null
                              : () async {
                                  final ok = await widget.controller.disable(
                                    widget.pluginId,
                                    widget.extensionId,
                                  );
                                  if (mounted) {
                                    Navigator.pop(context);
                                    _showSnackBar(
                                      context,
                                      ok
                                          ? '禁用成功'
                                          : (widget.controller.current.error ??
                                                '操作失败'),
                                    );
                                  }
                                },
                        )
                      : AmitiaButton(
                          label: hasOp ? '处理中...' : '启用',
                          isFullWidth: true,
                          onPressed: hasOp
                              ? null
                              : () async {
                                  final ok = await widget.controller.enable(
                                    widget.pluginId,
                                    widget.extensionId,
                                  );
                                  if (mounted) {
                                    Navigator.pop(context);
                                    _showSnackBar(
                                      context,
                                      ok
                                          ? '启用成功'
                                          : (widget.controller.current.error ??
                                                '操作失败'),
                                    );
                                  }
                                },
                        ),
                ),
                SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButtonOutline(
                    label: hasOp ? '处理中...' : '更新',
                    onPressed: hasOp
                        ? null
                        : () {
                            Navigator.pop(context);
                            _pickAndPreviewPackage(
                              widget.hostContext,
                              widget.controller,
                              pickPackage: widget.pickPackage,
                              expectedExtensionId: widget.extensionId,
                            );
                          },
                  ),
                ),
              ],
            ),
            SizedBox(height: AppSpacing.sm),
            Row(
              children: [
                Expanded(
                  child: AmitiaButtonOutline(
                    label: hasOp ? '处理中...' : '卸载插件',
                    onPressed: hasOp
                        ? null
                        : () {
                            Navigator.pop(context);
                            _showUninstallConfirm(context);
                          },
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showSnackBar(BuildContext context, String message) {
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  void _showUninstallConfirm(BuildContext context) {
    showDialog(
      context: context,
      builder: (dialogCtx) => AlertDialog(
        title: const Text('确认卸载'),
        content: Text('确定要卸载插件 "${widget.name}" 吗？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogCtx),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () async {
              Navigator.pop(dialogCtx);
              final ok = await widget.controller.uninstall(
                widget.pluginId,
                widget.extensionId,
              );
              if (context.mounted) {
                final message = ok
                    ? '卸载成功'
                    : (widget.controller.current.error ?? '卸载失败');
                ScaffoldMessenger.of(
                  context,
                ).showSnackBar(SnackBar(content: Text(message)));
              }
            },
            child: const Text('卸载'),
          ),
        ],
      ),
    );
  }

  Widget _buildInfoRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: AppTypography.caption(context)),
          Flexible(
            child: Text(
              value,
              style: AppTypography.body(context),
              textAlign: TextAlign.right,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }

  String _installStateLabel(String state) {
    switch (state) {
      case 'installed':
        return '已安装';
      case 'installing':
        return '安装中';
      case 'failed':
        return '安装失败';
      case 'uninstalling':
        return '卸载中';
      default:
        return state.isEmpty ? '未知' : state;
    }
  }
}
