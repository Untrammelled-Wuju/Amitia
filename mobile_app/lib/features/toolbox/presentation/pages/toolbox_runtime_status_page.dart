import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class ToolboxRuntimeStatusPage extends ConsumerStatefulWidget {
  const ToolboxRuntimeStatusPage({super.key});

  @override
  ConsumerState<ToolboxRuntimeStatusPage> createState() => _ToolboxRuntimeStatusPageState();
}

class _ToolboxRuntimeStatusPageState extends ConsumerState<ToolboxRuntimeStatusPage> {
  List<Map<String, dynamic>> _components = [];
  bool _loading = true;
  String? _error;
  int _readyCount = 0;
  int _totalCount = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(systemServiceProvider);
      final result = await svc.health();
      final data = result as Map<String, dynamic>? ?? {};
      if (mounted) {
        final comps = (data['components'] as List?)?.cast<Map<String, dynamic>>() ??
            (data['snapshots'] as List?)?.cast<Map<String, dynamic>>() ?? [];
        setState(() {
          _components = comps;
          _totalCount = data['totalCount'] as int? ?? comps.length;
          _readyCount = data['readyCount'] as int? ??
              comps.where((c) {
                final s = (c['status'] ?? c['state'] ?? '').toString().toLowerCase();
                return s == 'running' || s == 'ok' || s == 'ready' || s == 'up';
              }).length;
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  BadgeType _badgeForType(String status) {
    final s = status.toLowerCase();
    if (s == 'running' || s == 'ok' || s == 'ready' || s == 'up') return BadgeType.success;
    if (s == 'warn' || s == 'warning' || s == 'degraded') return BadgeType.warning;
    if (s == 'error' || s == 'stopped' || s == 'down' || s == 'failed') return BadgeType.error;
    return BadgeType.neutral;
  }

  String _iconNameToData(String? name) {
    if (name == null) return 'extension_outlined';
    final n = name.toLowerCase();
    if (n.contains('go') || n.contains('backend') || n.contains('server')) return 'terminal';
    if (n.contains('surreal') || n.contains('database') || n.contains('db') || n.contains('sqlite')) return 'storage';
    if (n.contains('qdrant') || n.contains('vector') || n.contains('embed')) return 'scatter_plot_outlined';
    if (n.contains('mcp')) return 'extension_outlined';
    return 'extension_outlined';
  }

  IconData _resolveIcon(String semanticIcon) {
    switch (semanticIcon) {
      case 'terminal': return Icons.terminal;
      case 'storage': return Icons.storage;
      case 'scatter_plot_outlined': return Icons.scatter_plot_outlined;
      default: return Icons.extension_outlined;
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState(message: '正在检查运行时状态...');
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);

    final allOk = _readyCount >= _totalCount && _totalCount > 0;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: 'Runtime 状态', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Container(
            padding: EdgeInsets.all(AppSpacing.cardPadding),
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brMedium,
            ),
            child: Row(
              children: [
                Icon(allOk ? Icons.check_circle : Icons.warning_amber_rounded,
                    color: allOk ? context.success : context.warning, size: 28),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(allOk ? '运行时整体健康' : '部分组件异常', style: AppTypography.cardTitle(context)),
                      Text('$_readyCount/$_totalCount 组件运行中', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
              ],
            ),
          ),
          SizedBox(height: AppSpacing.lg),
          if (_components.isEmpty)
            const AmitiaEmptyState(icon: Icons.extension_outlined, title: '暂无组件数据', subtitle: '服务可能尚未就绪')
          else
            ..._components.map((c) {
              final name = (c['name'] ?? c['component'] ?? 'Unknown').toString();
              final status = (c['status'] ?? c['state'] ?? '').toString();
              final version = (c['version'] ?? c['detail'] ?? c['info'] ?? '').toString();
              final iconKey = _iconNameToData(name);
              return Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.md),
                child: Container(
                  padding: EdgeInsets.all(AppSpacing.cardPadding),
                  decoration: BoxDecoration(
                    color: context.surfacePrimary,
                    borderRadius: AppRadius.brMedium,
                    border: Border.all(color: context.borderPrimary, width: 0.5),
                  ),
                  child: Row(
                    children: [
                      Container(
                        width: 44,
                        height: 44,
                        decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                        child: Icon(_resolveIcon(iconKey), size: 22, color: context.accentPrimary),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(name, style: AppTypography.cardTitle(context)),
                            const SizedBox(height: 2),
                            Text(version, style: AppTypography.caption(context)),
                          ],
                        ),
                      ),
                      AmitiaStatusBadge(
                        label: status.isEmpty ? '未知' : status,
                        type: _badgeForType(status),
                      ),
                    ],
                  ),
                ),
              );
            }),
          SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}
