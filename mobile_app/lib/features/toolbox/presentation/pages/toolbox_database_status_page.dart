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

class ToolboxDatabaseStatusPage extends ConsumerStatefulWidget {
  const ToolboxDatabaseStatusPage({super.key});

  @override
  ConsumerState<ToolboxDatabaseStatusPage> createState() => _ToolboxDatabaseStatusPageState();
}

class _ToolboxDatabaseStatusPageState extends ConsumerState<ToolboxDatabaseStatusPage> {
  List<Map<String, dynamic>> _databases = [];
  bool _loading = true;
  String? _error;
  Map<String, dynamic> _chatStats = {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  IconData _iconForName(String name) {
    final n = name.toLowerCase();
    if (n.contains('sqlite') || n.contains('surreal') || n.contains('database') || n.contains('db')) return Icons.storage;
    if (n.contains('qdrant') || n.contains('vector') || n.contains('embed')) return Icons.scatter_plot_outlined;
    return Icons.table_rows_outlined;
  }

  BadgeType _badgeForStatus(String status) {
    final s = status.toLowerCase();
    if (s == 'running' || s == 'ok' || s == 'ready' || s == 'up') return BadgeType.success;
    if (s == 'warn' || s == 'warning') return BadgeType.warning;
    if (s == 'error' || s == 'stopped' || s == 'down') return BadgeType.error;
    return BadgeType.neutral;
  }

  String _sizeInfo(String name) {
    final n = name.toLowerCase();
    if (n.contains('sqlite') || n.contains('db')) {
      final total = _chatStats['totalConversations'] ?? _chatStats['conversationCount'];
      if (total != null) return '$total 条会话';
    }
    if (n.contains('qdrant') || n.contains('vector')) {
      final vecs = _chatStats['vectorCount'] ?? _chatStats['totalVectors'];
      if (vecs != null) return '$vecs 向量';
    }
    return '';
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(systemServiceProvider);
      final healthResult = await svc.health();
      final healthData = healthResult as Map<String, dynamic>? ?? {};
      List<Map<String, dynamic>> dbComponents = [];
      final components = (healthData['components'] as List?)?.cast<Map<String, dynamic>>() ??
          (healthData['snapshots'] as List?)?.cast<Map<String, dynamic>>() ?? [];
      for (final c in components) {
        final name = (c['name'] ?? c['component'] ?? '').toString().toLowerCase();
        if (name.contains('db') || name.contains('database') || name.contains('sqlite') ||
            name.contains('surreal') || name.contains('qdrant') || name.contains('vector') ||
            name.contains('storage')) {
          dbComponents.add(c);
        }
      }

      Map<String, dynamic> stats = {};
      try {
        final statsResult = await svc.chatStats();
        stats = statsResult as Map<String, dynamic>? ?? {};
      } catch (_) {}

      if (mounted) {
        setState(() {
          _databases = dbComponents;
          _chatStats = stats;
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState(message: '正在加载数据库状态...');
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);

    final now = DateTime.now();
    final timeStr = '${now.hour.toString().padLeft(2, '0')}:${now.minute.toString().padLeft(2, '0')}';

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '数据库状态', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4),
            child: Text('最后检查：刚刚 · 自动每 5 分钟刷新', style: AppTypography.caption(context)),
          ),
          const SizedBox(height: AppSpacing.md),
          if (_databases.isEmpty)
            const AmitiaEmptyState(icon: Icons.storage, title: '暂无数据库组件', subtitle: '未检测到数据库服务')
          else
            ..._databases.map((d) {
              final name = (d['name'] ?? d['component'] ?? 'Unknown').toString();
              final status = (d['status'] ?? d['state'] ?? '').toString();
              final detail = (d['version'] ?? d['detail'] ?? d['info'] ?? '').toString();
              final size = _sizeInfo(name);
              return Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.md),
                child: Container(
                  padding: const EdgeInsets.all(AppSpacing.cardPadding),
                  decoration: BoxDecoration(
                    color: context.surfacePrimary,
                    borderRadius: AppRadius.brMedium,
                    border: Border.all(color: context.borderPrimary, width: 0.5),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Container(
                            width: 40,
                            height: 40,
                            decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                            child: Icon(_iconForName(name), size: 20, color: context.accentPrimary),
                          ),
                          const SizedBox(width: 12),
                          Expanded(child: Text(name, style: AppTypography.cardTitle(context))),
                          AmitiaStatusBadge(label: status.isEmpty ? '未知' : status, type: _badgeForStatus(status)),
                        ],
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      Row(
                        children: [
                          Text('详情', style: AppTypography.label(context)),
                          const SizedBox(width: 6),
                          Text(detail.isEmpty ? (size.isEmpty ? '—' : size) : detail, style: AppTypography.bodySmall(context)),
                          const Spacer(),
                          Text('最后检查', style: AppTypography.label(context)),
                          const SizedBox(width: 6),
                          Text('$timeStr 前', style: AppTypography.bodySmall(context)),
                        ],
                      ),
                    ],
                  ),
                ),
              );
            }),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}
