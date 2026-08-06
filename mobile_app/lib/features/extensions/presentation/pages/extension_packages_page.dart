import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class ExtensionPackagesPage extends ConsumerStatefulWidget {
  const ExtensionPackagesPage({super.key});

  @override
  ConsumerState<ExtensionPackagesPage> createState() => _ExtensionPackagesPageState();
}

class _ExtensionPackagesPageState extends ConsumerState<ExtensionPackagesPage> {
  List<Map<String, dynamic>> _packages = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadPackages();
  }

  Future<void> _loadPackages() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(extensionServiceProvider);
      final data = await svc.plugins();
      if (mounted) setState(() { _packages = data; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  BadgeType _statusBadgeType(String status) {
    switch (status) {
      case '运行中':
        return BadgeType.success;
      case '已暂停':
        return BadgeType.warning;
      case '已安装':
        return BadgeType.info;
      default:
        return BadgeType.neutral;
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '扩展包', showBackButton: true, fallbackRoute: AppRoutes.extensions, actions: [
          AmitiaIconButton(icon: Icons.download_outlined, onPressed: _showInstallLocalSheet, tooltip: '安装本地包'),
        ]),
        body: SafeArea(top: false, child: const AmitiaLoadingState(message: '加载中...')),
      );
    }
    if (_error != null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '扩展包', showBackButton: true, fallbackRoute: AppRoutes.extensions, actions: [
          AmitiaIconButton(icon: Icons.download_outlined, onPressed: _showInstallLocalSheet, tooltip: '安装本地包'),
        ]),
        body: SafeArea(top: false, child: AmitiaErrorState(message: '加载失败: $_error', onRetry: _loadPackages)),
      );
    }
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '扩展包',
        showBackButton: true,
        fallbackRoute: AppRoutes.extensions,
        actions: [
          AmitiaIconButton(
            icon: Icons.download_outlined,
            onPressed: _showInstallLocalSheet,
            tooltip: '安装本地包',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: ListView.separated(
          padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
          itemCount: _packages.length,
          separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
          itemBuilder: (context, index) => _buildPackageCard(context, _packages[index]),
        ),
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: _showInstallLocalSheet,
        backgroundColor: context.accentPrimary,
        child: const Icon(Icons.add, color: Colors.white),
      ),
    );
  }

  Widget _buildPackageCard(BuildContext context, Map<String, dynamic> pkg) {
    final name = (pkg['name'] ?? '').toString();
    final description = (pkg['description'] ?? '').toString();
    final version = (pkg['version'] ?? '1.0.0').toString();
    final status = (pkg['status'] ?? '已安装').toString();
    final permissions = (pkg['permissions'] as List?)?.map((e) => e.toString()).toList() ?? [];
    final hasUpdate = (pkg['hasUpdate'] as bool?) ?? false;
    final isEnabled = (pkg['isEnabled'] as bool?) ?? ((pkg['enabled'] as int?) == 1);

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
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(Icons.extension_outlined, size: 22, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Text(name, style: AppTypography.cardTitle(context)),
                        const SizedBox(width: 8),
                        Text('v$version', style: AppTypography.label(context)),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text(description, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              AmitiaStatusBadge(label: status, type: _statusBadgeType(status)),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: permissions.map((p) => Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Text(p, style: AppTypography.label(context)),
            )).toList(),
          ),
          const SizedBox(height: AppSpacing.md),
          if (hasUpdate)
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Row(
                children: [
                  Icon(Icons.system_update, size: 16, color: context.accentPrimary),
                  const SizedBox(width: 6),
                  Text('有新版本可用', style: AppTypography.label(context).copyWith(color: context.accentPrimary)),
                ],
              ),
            ),
          const SizedBox(height: AppSpacing.sm),
          _buildActionButtons(context, pkg),
        ],
      ),
    );
  }

  Widget _buildActionButtons(BuildContext context, Map<String, dynamic> pkg) {
    final status = (pkg['status'] ?? '已安装').toString();
    final name = (pkg['name'] ?? '').toString();
    final isEnabled = (pkg['isEnabled'] as bool?) ?? ((pkg['enabled'] as int?) == 1);
    final hasUpdate = (pkg['hasUpdate'] as bool?) ?? false;
    final isRunning = status == '运行中';
    final isPaused = status == '已暂停';

    return Wrap(
      spacing: AppSpacing.sm,
      runSpacing: AppSpacing.sm,
      children: [
        if (hasUpdate)
          _MiniButton(
            label: '更新',
            icon: Icons.system_update,
            color: context.accentPrimary,
            onTap: () => _showUpdateDialog(pkg),
          ),
        if (isRunning)
          _MiniButton(
            label: '暂停',
            icon: Icons.pause_circle_outline,
            color: context.warning,
            onTap: () => _showPauseConfirm(pkg),
          ),
        if (isPaused)
          _MiniButton(
            label: '恢复',
            icon: Icons.play_circle_outline,
            color: context.success,
            onTap: () => _togglePlugin(pkg),
          ),
        if (isEnabled && !isPaused)
          _MiniButton(
            label: '暂停',
            icon: Icons.pause_circle_outline,
            color: context.warning,
            onTap: () => _togglePlugin(pkg),
          ),
        if (!isEnabled)
          _MiniButton(
            label: '启用',
            icon: Icons.play_circle_outline,
            color: context.success,
            onTap: () => _togglePlugin(pkg),
          ),
        _MiniButton(
          label: '卸载',
          icon: Icons.delete_outline,
          color: context.error,
          onTap: () => _showUninstallConfirm(pkg),
        ),
      ],
    );
  }

  void _showInstallLocalSheet() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (context) => _InstallPreviewSheet(onConfirm: () {
        Navigator.pop(context);
        ScaffoldMessenger.of(this.context).showSnackBar(
          SnackBar(content: const Text('安装预览已确认，正在安装...'), backgroundColor: context.accentPrimary),
        );
      }),
    );
  }

  void _showExtensionDetail(Map<String, dynamic> pkg) {
    showDialog(
      context: context,
      builder: (context) => _ExtensionDetailDialog(pkg: pkg),
    );
  }

  void _showUpdateDialog(Map<String, dynamic> pkg) {
    final name = (pkg['name'] ?? '').toString();
    final version = (pkg['version'] ?? '1.0.0').toString();
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('更新扩展', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('$name 有新版本可用', style: AppTypography.bodySmall(context)),
            const SizedBox(height: 12),
            _DetailRow(label: '当前版本', value: 'v$version'),
            _DetailRow(label: '新版本', value: 'v1.3.0'),
            _DetailRow(label: '更新大小', value: '1.2 MB'),
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Text('更新日志：\n- 优化性能\n- 修复已知问题\n- 新增 API 支持', style: AppTypography.label(context).copyWith(color: context.accentPrimary)),
            ),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('稍后', style: TextStyle(color: context.textSecondary))),
          TextButton(onPressed: () {
            Navigator.pop(context);
            ScaffoldMessenger.of(this.context).showSnackBar(
              SnackBar(content: Text('$name 已更新到最新版本'), backgroundColor: context.success),
            );
          }, child: Text('立即更新', style: TextStyle(color: context.accentPrimary))),
        ],
      ),
    );
  }

  void _showPauseConfirm(Map<String, dynamic> pkg) {
    final name = (pkg['name'] ?? '').toString();
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('暂停扩展', style: AppTypography.cardTitle(context)),
        content: Text('确定要暂停「$name」吗？暂停后该扩展将停止运行，相关功能将不可用。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              _togglePlugin(pkg);
            },
            child: Text('暂停', style: TextStyle(color: context.warning)),
          ),
        ],
      ),
    );
  }

  Future<void> _togglePlugin(Map<String, dynamic> pkg) async {
    final id = (pkg['id'] ?? '').toString();
    final name = (pkg['name'] ?? '').toString();
    final isEnabled = (pkg['isEnabled'] as bool?) ?? ((pkg['enabled'] as int?) == 1);
    try {
      final svc = ref.read(extensionServiceProvider);
      if (isEnabled) {
        await svc.disablePlugin(id);
      } else {
        await svc.enablePlugin(id);
      }
      _loadPackages();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('$name 已${isEnabled ? '停用' : '启用'}'), backgroundColor: context.success),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('操作失败: $e'), backgroundColor: context.error),
        );
      }
    }
  }

  void _showUninstallConfirm(Map<String, dynamic> pkg) {
    final name = (pkg['name'] ?? '').toString();
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('卸载扩展', style: AppTypography.cardTitle(context)),
        content: Text('确定要卸载「$name」吗？此操作不可撤销，相关数据将被清除。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              ScaffoldMessenger.of(this.context).showSnackBar(
                SnackBar(content: Text('$name 已卸载'), backgroundColor: context.error),
              );
            },
            child: Text('卸载', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
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
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.1),
          borderRadius: AppRadius.brTag,
        ),
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

class _InstallPreviewSheet extends StatelessWidget {
  final VoidCallback onConfirm;

  const _InstallPreviewSheet({required this.onConfirm});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 34),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Center(
            child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
          ),
          const SizedBox(height: 20),
          Text('安装本地扩展包', style: AppTypography.pageTitle(context)),
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brMedium,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(Icons.archive_outlined, size: 28, color: context.accentPrimary),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text('example-extension-1.0.0.zip', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                          const SizedBox(height: 2),
                          Text('2.4 MB · 选择文件', style: AppTypography.label(context)),
                        ],
                      ),
                    ),
                    GestureDetector(
                      onTap: () => amitiaComingSoon(context, '文件选择'),
                      child: Container(
                        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                        decoration: BoxDecoration(
                          color: context.accentSoft,
                          borderRadius: AppRadius.brTag,
                        ),
                        child: Text('选择', style: TextStyle(fontSize: 13, color: context.accentPrimary, fontWeight: FontWeight.w500)),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          Text('安装预览', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: 12),
          _PreviewRow(label: '扩展名称', value: '示例扩展'),
          _PreviewRow(label: '版本', value: '1.0.0'),
          _PreviewRow(label: '所需权限', value: '文件读写、网络访问'),
          _PreviewRow(label: '依赖项', value: '无'),
          const SizedBox(height: 20),
          Row(
            children: [
              Expanded(
                child: AmitiaButton(label: '取消', isSecondary: true, onPressed: () => Navigator.pop(context)),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: AmitiaButton(label: '确认安装', onPressed: onConfirm),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _PreviewRow extends StatelessWidget {
  final String label;
  final String value;

  const _PreviewRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          SizedBox(width: 80, child: Text(label, style: AppTypography.label(context))),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }
}

class _ExtensionDetailDialog extends StatelessWidget {
  final Map<String, dynamic> pkg;

  const _ExtensionDetailDialog({required this.pkg});

  @override
  Widget build(BuildContext context) {
    final name = (pkg['name'] ?? '').toString();
    final description = (pkg['description'] ?? '').toString();
    final version = (pkg['version'] ?? '1.0.0').toString();
    final status = (pkg['status'] ?? '已安装').toString();
    final permissions = (pkg['permissions'] as List?)?.map((e) => e.toString()).toList() ?? [];

    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      title: Row(
        children: [
          Icon(Icons.extension_outlined, color: context.accentPrimary, size: 24),
          const SizedBox(width: 10),
          Expanded(child: Text(name, style: AppTypography.cardTitle(context))),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(description, style: AppTypography.bodySmall(context)),
          const SizedBox(height: 16),
          _DetailRow(label: '版本', value: 'v$version'),
          _DetailRow(label: '状态', value: status),
          _DetailRow(label: '权限', value: permissions.join('、')),
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brSmall,
            ),
            child: Row(
              children: [
                Icon(Icons.info_outline, size: 16, color: context.textTertiary),
                const SizedBox(width: 8),
                Expanded(child: Text('扩展运行于隔离沙箱中，权限受系统管控', style: AppTypography.label(context))),
              ],
            ),
          ),
        ],
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: Text('关闭', style: TextStyle(color: context.textSecondary))),
      ],
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
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(width: 60, child: Text(label, style: AppTypography.label(context))),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }
}
