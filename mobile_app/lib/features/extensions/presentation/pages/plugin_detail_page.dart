import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class PluginDetailPage extends ConsumerStatefulWidget {
  final String pluginId;

  const PluginDetailPage({super.key, required this.pluginId});

  @override
  ConsumerState<PluginDetailPage> createState() => _PluginDetailPageState();
}

class _PluginDetailPageState extends ConsumerState<PluginDetailPage> {
  Map<String, dynamic>? _plugin;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadPlugin();
  }

  Future<void> _loadPlugin() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(extensionServiceProvider);
      final plugins = await svc.plugins();
      final found = plugins.where((p) => (p['id'] ?? '').toString() == widget.pluginId).firstOrNull;
      if (mounted) setState(() { _plugin = found; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '插件详情', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: const AmitiaLoadingState(message: '加载中...')),
      );
    }
    if (_error != null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '插件详情', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: AmitiaErrorState(message: _error!, onRetry: _loadPlugin)),
      );
    }
    final plugin = _plugin;
    if (plugin == null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '插件详情', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: Center(child: Text('未找到插件', style: AppTypography.body(context)))),
      );
    }

    final name = (plugin['name'] ?? '').toString();
    final description = (plugin['description'] ?? '').toString();
    final isInstalled = (plugin['isInstalled'] as bool?) ?? ((plugin['installed'] as int?) == 1);
    final isEnabled = (plugin['isEnabled'] as bool?) ?? ((plugin['enabled'] as int?) == 1);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: name, showBackButton: true, fallbackRoute: AppRoutes.extensions),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Container(
                        width: 48,
                        height: 48,
                        decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                        child: Icon(Icons.extension_outlined, size: 24, color: context.accentPrimary),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(name, style: AppTypography.cardTitle(context)),
                            Text(description, style: AppTypography.caption(context)),
                          ],
                        ),
                      ),
                    ],
                  ),
                  SizedBox(height: AppSpacing.md),
                  _InfoRow(label: '类型', value: 'Plugin'),
                  _InfoRow(label: '状态', value: isInstalled ? '已安装' : '未安装'),
                  _InfoRow(label: '启用', value: isEnabled ? '已启用' : '已禁用'),
                ],
              ),
            ),
            SizedBox(height: AppSpacing.md),
            AmitiaButton(
              label: isInstalled ? '卸载' : '安装',
              isFullWidth: true,
              onPressed: () async {
                try {
                  final svc = ref.read(extensionServiceProvider);
                  if (isInstalled) {
                    await svc.disablePlugin(widget.pluginId);
                  } else {
                    await svc.enablePlugin(widget.pluginId);
                  }
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(isInstalled ? '已卸载' : '已安装'), backgroundColor: context.success),
                    );
                    context.pop();
                  }
                } catch (e) {
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('操作失败: $e'), backgroundColor: context.error),
                    );
                  }
                }
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;

  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: AppTypography.caption(context)),
          Text(value, style: AppTypography.body(context)),
        ],
      ),
    );
  }
}
