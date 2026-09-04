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
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

class HooksPage extends ConsumerStatefulWidget {
  const HooksPage({super.key});

  @override
  ConsumerState<HooksPage> createState() => _HooksPageState();
}

class _HooksPageState extends ConsumerState<HooksPage> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _hooks = [];

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
      final data = await api.get<Map<String, dynamic>>(
        '/api/extensions/hooks/contributions',
        fromJson: (e) => Map<String, dynamic>.from(e as Map),
      );
      final raw = data?['contributions'] as List<dynamic>? ?? const [];
      final hooks = raw.whereType<Map>().map((entry) {
        final item = Map<String, dynamic>.from(entry);
        final enabled = item['enabled'] == true;
        final circuit = (item['circuitState'] ?? '').toString();
        return <String, dynamic>{
          'id': (item['contributionId'] ?? '').toString(),
          'point': (item['hookPointId'] ?? '').toString(),
          'contributor': (item['extensionId'] ?? '').toString(),
          'priority': item['priority'] ?? 0,
          'status': circuit == 'open' ? 'circuit_open' : (enabled ? 'active' : 'inactive'),
          'raw': item,
        };
      }).toList();
      if (mounted) {
        setState(() {
          _hooks = hooks;
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);

    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: 'Hook 中心',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
      ),
      body: SafeArea(
        top: false,
        child: _hooks.isEmpty
            ? AmitiaEmptyState(
                icon: Icons.account_tree_outlined,
                title: '暂无 Hook 点',
                subtitle: 'Hook 点将在扩展注册后自动生成',
              )
            : ListView.builder(
                padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
                itemCount: _hooks.length,
                itemBuilder: (context, index) => _buildHookCard(context, _hooks[index]),
              ),
      ),
    );
  }

  Widget _buildHookCard(BuildContext context, Map<String, dynamic> hook) {
    final point = hook['point'] as String? ?? '';
    final contributor = hook['contributor'] as String? ?? '';
    final priority = hook['priority'] ?? 0;
    final status = hook['status'] as String? ?? '';
    final id = hook['id'] as String? ?? '';

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        onTap: () => _showHookDetailSheet(context, hook),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.account_tree_outlined, size: 22, color: context.accentPrimary),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(point, style: AppTypography.cardTitle(context).copyWith(fontFamily: 'monospace', fontSize: 14)),
                      const SizedBox(height: 2),
                      Text(contributor, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                _buildStatusBadge(status),
              ],
            ),
            SizedBox(height: AppSpacing.md),
            Row(
              children: [
                _buildInfoChip(context, '优先级', '$priority'),
                SizedBox(width: AppSpacing.sm),
                _buildInfoChip(context, '贡献者', contributor),
              ],
            ),
            SizedBox(height: AppSpacing.md),
            Row(
              children: [
                if (status == 'circuit_open')
                  Expanded(
                    child: AmitiaButton(
                      label: '熔断器详情',
                      isSecondary: true,
                      icon: Icons.error_outline,
                      onPressed: () => _showCircuitBreakerSheet(context, hook),
                    ),
                  )
                else
                  Expanded(
                    child: AmitiaButton(
                      label: status == 'active' ? '停用' : '启用',
                      isSecondary: true,
                      icon: status == 'active' ? Icons.pause_circle_outline : Icons.play_circle_outline,
                      onPressed: () => _showToggleConfirm(context, hook),
                    ),
                  ),
                SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '贡献详情',
                    isSecondary: true,
                    icon: Icons.info_outline,
                    onPressed: () => _showHookDetailSheet(context, hook),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildStatusBadge(String status) {
    switch (status) {
      case 'active':
        return const AmitiaStatusBadge(label: '活跃', type: BadgeType.success);
      case 'inactive':
        return const AmitiaStatusBadge(label: '已停用', type: BadgeType.neutral);
      case 'circuit_open':
        return const AmitiaStatusBadge(label: '熔断中', type: BadgeType.error);
      default:
        return const AmitiaStatusBadge(label: '未知', type: BadgeType.neutral);
    }
  }

  Widget _buildInfoChip(BuildContext context, String label, String value) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: context.surfaceSecondary,
        borderRadius: AppRadius.brTag,
      ),
      child: Text('$label: $value', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
    );
  }

  void _showHookDetailSheet(BuildContext context, Map<String, dynamic> hook) {
    final point = hook['point'] as String? ?? '';
    final contributor = hook['contributor'] as String? ?? '';
    final priority = hook['priority'] ?? 0;
    final status = hook['status'] as String? ?? '';
    final id = hook['id'] as String? ?? '';

    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (context) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
                ),
                const SizedBox(height: 20),
                Text('Hook 贡献详情', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                _buildDetailRow(context, 'Hook 点', point),
                _buildDetailRow(context, '贡献者', contributor),
                _buildDetailRow(context, '优先级', '$priority'),
                _buildDetailRow(context, '状态', _statusLabel(status)),
                _buildDetailRow(context, 'Hook ID', id),
                const SizedBox(height: 20),
                AmitiaButton(
                  label: '关闭',
                  isFullWidth: true,
                  isSecondary: true,
                  onPressed: () => Navigator.pop(context),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildDetailRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        children: [
          SizedBox(
            width: 80,
            child: Text(label, style: AppTypography.label(context).copyWith(color: context.textTertiary)),
          ),
          Expanded(child: Text(value, style: AppTypography.body(context))),
        ],
      ),
    );
  }

  void _showCircuitBreakerSheet(BuildContext context, Map<String, dynamic> hook) {
    final point = hook['point'] as String? ?? '';
    final contributor = hook['contributor'] as String? ?? '';

    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (context) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
                ),
                const SizedBox(height: 20),
                Row(
                  children: [
                    Icon(Icons.error_outline, color: context.error, size: 24),
                    const SizedBox(width: 8),
                    Text('熔断器详情', style: AppTypography.pageTitle(context)),
                  ],
                ),
                const SizedBox(height: 16),
                Container(
                  padding: EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: context.error.withValues(alpha: 0.08),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Row(
                    children: [
                      Icon(Icons.warning_amber_rounded, color: context.error, size: 20),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text('Hook 点「$point」的熔断器已打开，贡献者执行持续失败导致熔断。', style: AppTypography.caption(context).copyWith(color: context.error)),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 16),
                _buildDetailRow(context, 'Hook 点', point),
                _buildDetailRow(context, '贡献者', contributor),
                _buildDetailRow(context, '失败次数', '5'),
                _buildDetailRow(context, '熔断阈值', '5'),
                _buildDetailRow(context, '恢复策略', '半开探测'),
                const SizedBox(height: 20),
                AmitiaButton(
                  label: '重置熔断器',
                  isFullWidth: true,
                  icon: Icons.refresh,
                  onPressed: () async {
                    final id = (hook['id'] ?? '').toString();
                    try {
                      await ref.read(backendServiceProvider).post('/api/extensions/hooks/contributions/$id/circuit/reset');
                      if (!context.mounted) return;
                      Navigator.pop(context);
                      await _load();
                      if (mounted) ScaffoldMessenger.of(this.context).showSnackBar(SnackBar(content: Text('已重置熔断器：$point')));
                    } catch (e) {
                      if (context.mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('重置失败：$e')));
                    }
                  },
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  void _showToggleConfirm(BuildContext context, Map<String, dynamic> hook) {
    final point = hook['point'] as String? ?? '';
    final isActive = hook['status'] == 'active';
    final action = isActive ? '停用' : '启用';
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('$action Hook', style: AppTypography.cardTitle(context)),
          content: Text('确定要$action Hook 点「$point」吗？', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () async {
                final id = (hook['id'] ?? '').toString();
                try {
                  final suffix = isActive ? 'disable' : 'enable';
                  await ref.read(backendServiceProvider).post('/api/extensions/hooks/contributions/$id/$suffix');
                  if (!context.mounted) return;
                  Navigator.pop(context);
                  await _load();
                  if (mounted) ScaffoldMessenger.of(this.context).showSnackBar(SnackBar(content: Text('已$action：$point')));
                } catch (e) {
                  if (context.mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('$action失败：$e')));
                }
              },
              child: Text(action, style: TextStyle(color: isActive ? context.warning : context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  String _statusLabel(String status) {
    switch (status) {
      case 'active':
        return '活跃';
      case 'inactive':
        return '已停用';
      case 'circuit_open':
        return '熔断中';
      default:
        return status;
    }
  }
}
