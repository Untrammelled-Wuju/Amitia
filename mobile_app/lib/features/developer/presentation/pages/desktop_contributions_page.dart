import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class DesktopContributionsPage extends ConsumerStatefulWidget {
  const DesktopContributionsPage({super.key});

  @override
  ConsumerState<DesktopContributionsPage> createState() => _DesktopContributionsPageState();
}

class _DesktopContributionsPageState extends ConsumerState<DesktopContributionsPage> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _contributions = const [];
  List<Map<String, dynamic>> _conflicts = const [];

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
      final api = ref.read(backendServiceProvider);
      final contributionResponse = await api.get<dynamic>('/api/extensions/desktop/contributions');
      final conflictResponse = await api.get<dynamic>('/api/extensions/desktop/conflicts');
      if (!mounted) return;
      setState(() {
        _contributions = _items(contributionResponse);
        _conflicts = _items(conflictResponse);
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

  List<Map<String, dynamic>> _items(dynamic response) {
    dynamic source = response;
    if (response is Map) source = response['items'] ?? const [];
    if (source is! List) return const [];
    return source.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
  }

  Map<String, dynamic> _definition(Map<String, dynamic> contribution) {
    final value = contribution['definition'];
    return value is Map ? Map<String, dynamic>.from(value) : const {};
  }

  String _type(Map<String, dynamic> c) => (_definition(c)['desktopType'] ?? '').toString();
  String _id(Map<String, dynamic> c) => (_definition(c)['contributionId'] ?? c['contributionId'] ?? c['id'] ?? '').toString();
  String _label(Map<String, dynamic> c) {
    final effective = (c['effectiveLabel'] ?? '').toString().trim();
    if (effective.isNotEmpty) return effective;
    final label = _definition(c)['label'];
    if (label is Map) return (label['default'] ?? _id(c)).toString();
    return label?.toString() ?? _id(c);
  }

  String _accelerator(Map<String, dynamic> c) {
    final shortcut = _definition(c)['shortcut'];
    return shortcut is Map ? (shortcut['accelerator'] ?? '').toString() : '';
  }

  bool _enabled(Map<String, dynamic> c) {
    final status = (c['status'] ?? '').toString().toLowerCase();
    return status == 'enabled' || status == 'active' || status == 'registered';
  }

  List<Map<String, dynamic>> get _unresolvedConflicts => _conflicts.where((e) => e['resolved'] != true).toList();

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);

    final groups = <String, List<Map<String, dynamic>>>{};
    for (final contribution in _contributions) {
      groups.putIfAbsent(_type(contribution), () => []).add(contribution);
    }

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '桌面贡献中心',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(
            icon: Icons.monitor_heart_outlined,
            onPressed: _showSystemState,
            color: context.accentPrimary,
            tooltip: '桌面贡献运行状态',
          ),
          AmitiaIconButton(icon: Icons.refresh, onPressed: _load, color: context.textSecondary),
        ],
      ),
      body: SafeArea(
        top: false,
        child: RefreshIndicator(
          onRefresh: _load,
          child: ListView(
            padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
            children: [
              if (_unresolvedConflicts.isNotEmpty) _buildConflictWarning(context),
              if (_contributions.isEmpty)
                const SizedBox(height: 360, child: AmitiaEmptyState(icon: Icons.desktop_windows_outlined, title: '暂无桌面贡献')),
              for (final entry in groups.entries) ...[
                AmitiaSectionHeader(title: _typeLabel(entry.key)),
                SizedBox(height: AppSpacing.sm),
                ...entry.value.map((c) => _buildContributionCard(context, c)),
                SizedBox(height: AppSpacing.xl),
              ],
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildConflictWarning(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.lg),
      child: AmitiaCard(
        backgroundColor: context.warning.withValues(alpha: .06),
        border: Border.all(color: context.warning.withValues(alpha: .3), width: .5),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(children: [
              Icon(Icons.warning_amber_rounded, size: 20, color: context.warning),
              const SizedBox(width: 8),
              Expanded(child: Text('检测到 ${_unresolvedConflicts.length} 个未解决的桌面贡献冲突', style: AppTypography.bodySmall(context).copyWith(color: context.warning))),
            ]),
            const SizedBox(height: 8),
            ..._unresolvedConflicts.take(5).map((c) => ListTile(
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  title: Text('${c['type'] ?? '冲突'} · ${c['target'] ?? c['accelerator'] ?? ''}', style: AppTypography.label(context)),
                  subtitle: Text('${c['existingContribId'] ?? ''} ↔ ${c['newContribId'] ?? ''}', style: AppTypography.caption(context)),
                  trailing: TextButton(onPressed: () => _resolveConflict(c), child: const Text('解决')),
                )),
          ],
        ),
      ),
    );
  }

  Widget _buildContributionCard(BuildContext context, Map<String, dynamic> contribution) {
    final type = _type(contribution);
    final label = _label(contribution);
    final accelerator = _accelerator(contribution);
    final status = (contribution['status'] ?? 'unknown').toString();
    final isShortcut = type.contains('shortcut');
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
              child: Icon(_getIcon(type), size: 22, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text(label, style: AppTypography.cardTitle(context)),
                const SizedBox(height: 2),
                Text('${_typeLabel(type)} · $status', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
              ]),
            ),
            if (isShortcut) ...[
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
                decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brTag),
                child: Text(accelerator.isEmpty ? '未绑定' : accelerator, style: AppTypography.bodySmall(context).copyWith(fontFamily: 'monospace')),
              ),
              const SizedBox(width: 4),
              AmitiaIconButton(icon: Icons.edit_outlined, onPressed: () => _showEditShortcutDialog(context, contribution), color: context.accentPrimary),
            ],
            Switch.adaptive(value: _enabled(contribution), onChanged: (_) => _toggleContribution(contribution)),
          ],
        ),
      ),
    );
  }

  Future<void> _toggleContribution(Map<String, dynamic> contribution) async {
    final id = _id(contribution);
    if (id.isEmpty) return;
    try {
      final api = ref.read(backendServiceProvider);
      final action = _enabled(contribution) ? 'disable' : 'enable';
      await api.post<dynamic>('/api/extensions/desktop/contributions/$id/$action');
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败：$e')));
    }
  }

  Future<void> _showEditShortcutDialog(BuildContext context, Map<String, dynamic> contribution) async {
    final controller = TextEditingController(text: _accelerator(contribution));
    final newValue = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: dialogContext.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('修改快捷键', style: AppTypography.cardTitle(dialogContext)),
        content: AmitiaTextField(hintText: '例如: Ctrl+Shift+B', controller: controller),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, controller.text.trim()), child: const Text('保存')),
        ],
      ),
    );
    controller.dispose();
    if (newValue == null || newValue.isEmpty) return;
    final id = _id(contribution);
    try {
      final api = ref.read(backendServiceProvider);
      await api.post<dynamic>('/api/extensions/desktop/shortcuts/$id/rebind', data: {'accelerator': newValue});
      await _load();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('快捷键已重绑')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('重绑失败：$e')));
    }
  }

  Future<void> _resolveConflict(Map<String, dynamic> conflict) async {
    final controller = TextEditingController(text: (conflict['resolution'] ?? '').toString());
    final resolution = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: dialogContext.surfacePrimary,
        title: const Text('解决桌面贡献冲突'),
        content: AmitiaTextField(hintText: '填写解决方案，例如 keep-existing', controller: controller),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, controller.text.trim()), child: const Text('提交')),
        ],
      ),
    );
    controller.dispose();
    if (resolution == null || resolution.isEmpty) return;
    try {
      final api = ref.read(backendServiceProvider);
      await api.post<dynamic>('/api/extensions/desktop/conflicts/${conflict['conflictId']}/resolve', data: {'resolution': resolution});
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('解决失败：$e')));
    }
  }

  Future<void> _showSystemState() async {
    try {
      final api = ref.read(backendServiceProvider);
      final results = <String, dynamic>{};
      for (final entry in const <String, String>{
        'snapshot': '/api/extensions/desktop/snapshot',
        'contracts': '/api/extensions/desktop/contracts',
        'permissions': '/api/extensions/desktop/permissions',
        'resources': '/api/extensions/desktop/resources',
        'circuit': '/api/extensions/desktop/circuit/status',
      }.entries) {
        results[entry.key] = await api.get<dynamic>(entry.value);
      }
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          backgroundColor: dialogContext.surfacePrimary,
          title: const Text('桌面贡献运行状态'),
          content: SizedBox(
            width: 680,
            child: SingleChildScrollView(
              child: SelectableText(
                const JsonEncoder.withIndent('  ').convert(results),
                style: AppTypography.caption(dialogContext).copyWith(fontFamily: 'monospace'),
              ),
            ),
          ),
          actions: [
            TextButton(
              onPressed: () async {
                try {
                  await api.post<dynamic>('/api/extensions/desktop/circuit/reset');
                  if (dialogContext.mounted) Navigator.pop(dialogContext);
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('桌面贡献熔断器已重置')),
                    );
                  }
                } catch (e) {
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('重置失败：$e')),
                    );
                  }
                }
              },
              child: const Text('重置熔断器'),
            ),
            TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('关闭')),
          ],
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('读取运行状态失败：$e')));
      }
    }
  }

  String _typeLabel(String type) {
    switch (type) {
      case 'menu.item':
        return '菜单项';
      case 'menu.submenu':
        return '菜单子项';
      case 'tray.item':
        return '托盘项';
      case 'tray.submenu':
        return '托盘子项';
      case 'shortcut.app':
        return '应用快捷键';
      case 'shortcut.global':
        return '全局快捷键';
      default:
        return type.isEmpty ? '其他贡献' : type;
    }
  }

  IconData _getIcon(String type) {
    if (type.contains('shortcut')) return Icons.keyboard_outlined;
    if (type.contains('menu')) return Icons.menu_outlined;
    if (type.contains('tray')) return Icons.apps_outlined;
    return Icons.widgets_outlined;
  }
}
