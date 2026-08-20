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

class TrustedServicesPage extends ConsumerStatefulWidget {
  const TrustedServicesPage({super.key});

  @override
  ConsumerState<TrustedServicesPage> createState() => _TrustedServicesPageState();
}

class _TrustedServicesPageState extends ConsumerState<TrustedServicesPage> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _services = [];

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
      final svc = ref.read(systemServiceProvider);
      final data = await svc.diagnostics();
      if (mounted) {
        if (data != null) {
          final services = data['trusted_services'];
          if (services is List) {
            _services = services.map((e) => Map<String, dynamic>.from(e as Map)).toList();
          }
        }
        setState(() {
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
      appBar: AmitiaAppBar(
        title: '可信服务',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(
            icon: Icons.add,
            onPressed: () => _showRegisterSheet(context),
            color: context.accentPrimary,
            tooltip: '注册服务',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _services.isEmpty
            ? AmitiaEmptyState(
                icon: Icons.verified_user_outlined,
                title: '暂无可信服务',
                subtitle: '点击右上角注册新服务',
              )
            : ListView.builder(
                padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
                itemCount: _services.length,
                itemBuilder: (context, index) => _buildServiceCard(context, _services[index]),
              ),
      ),
    );
  }

  Widget _buildServiceCard(BuildContext context, Map<String, dynamic> service) {
    final name = service['name'] as String? ?? '';
    final id = service['id'] as String? ?? '';
    final runStatus = service['run_status'] as String? ?? '已停止';
    final isolated = service['isolated'] as bool? ?? false;
    final isRunning = runStatus == '已启动';

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: isolated ? context.error.withValues(alpha: 0.1) : context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(
                    Icons.verified_user_outlined,
                    size: 22,
                    color: isolated ? context.error : context.accentPrimary,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(name, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text('ID: $id', style: AppTypography.label(context)),
                    ],
                  ),
                ),
                Column(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    AmitiaStatusBadge(
                      label: runStatus,
                      type: isRunning ? BadgeType.success : BadgeType.neutral,
                    ),
                    if (isolated) ...[
                      const SizedBox(height: 4),
                      const AmitiaStatusBadge(label: '已隔离', type: BadgeType.error),
                    ],
                  ],
                ),
              ],
            ),
            SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.sm,
              children: _buildActionButtons(context, service, isRunning, isolated),
            ),
          ],
        ),
      ),
    );
  }

  List<Widget> _buildActionButtons(BuildContext context, Map<String, dynamic> service, bool isRunning, bool isolated) {
    final buttons = <Widget>[];

    if (isolated) {
      buttons.add(_miniButton(
        context,
        label: '解除隔离',
        icon: Icons.lock_open_outlined,
        color: context.success,
        onPressed: () => _showUnisolatedConfirm(context, service),
      ));
    } else {
      if (isRunning) {
        buttons.add(_miniButton(
          context,
          label: '停止',
          icon: Icons.stop,
          color: context.warning,
          onPressed: () => _toggleService(context, service, '已停止'),
        ));
      } else {
        buttons.add(_miniButton(
          context,
          label: '启动',
          icon: Icons.play_arrow,
          color: context.success,
          onPressed: () => _toggleService(context, service, '已启动'),
        ));
      }
      buttons.add(_miniButton(
        context,
        label: '调用',
        icon: Icons.call_made,
        color: context.accentPrimary,
        onPressed: () => _showCallResult(context, service),
      ));
      buttons.add(_miniButton(
        context,
        label: '注销',
        icon: Icons.delete_outline,
        color: context.error,
        onPressed: () => _showUnregisterConfirm(context, service),
      ));
    }

    return buttons;
  }

  Widget _miniButton(BuildContext context, {required String label, required IconData icon, required Color color, required VoidCallback onPressed}) {
    return GestureDetector(
      onTap: onPressed,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.08),
          borderRadius: AppRadius.brTag,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 16, color: color),
            const SizedBox(width: 6),
            Text(label, style: AppTypography.label(context).copyWith(color: color, fontWeight: FontWeight.w500)),
          ],
        ),
      ),
    );
  }

  void _showRegisterSheet(BuildContext context) {
    final nameController = TextEditingController();
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (context) {
        return Padding(
          padding: EdgeInsets.fromLTRB(20, 0, 20, MediaQuery.of(context).viewInsets.bottom + 34),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const SizedBox(height: 8),
              Center(
                child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
              ),
              const SizedBox(height: 20),
              Text('注册可信服务', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 20),
              Text('服务名称', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
              const SizedBox(height: 6),
              AmitiaTextField(hintText: '请输入服务名称', controller: nameController),
              const SizedBox(height: 20),
              AmitiaButton(
                label: '注册服务',
                isFullWidth: true,
                icon: Icons.check,
                onPressed: () {
                  final name = nameController.text.trim();
                  if (name.isEmpty) {
                    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('请输入服务名称')));
                    return;
                  }
                  setState(() {
                    _services.add({
                      'id': 'ts${_services.length + 1}',
                      'name': name,
                      'run_status': '已停止',
                      'isolated': false,
                    });
                  });
                  Navigator.pop(context);
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已注册服务：$name')));
                },
              ),
            ],
          ),
        );
      },
    );
  }

  void _toggleService(BuildContext context, Map<String, dynamic> service, String newStatus) {
    setState(() {
      final idx = _services.indexWhere((s) => s['id'] == service['id']);
      if (idx >= 0) {
        _services[idx]['run_status'] = newStatus;
      }
    });
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('服务「${service['name']}」已${newStatus == '已启动' ? '启动' : '停止'}')));
  }

  void _showCallResult(BuildContext context, Map<String, dynamic> service) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('调用结果', style: AppTypography.cardTitle(context)),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('服务: ${service['name']}', style: AppTypography.body(context)),
              const SizedBox(height: 8),
              Container(
                width: double.infinity,
                padding: EdgeInsets.all(AppSpacing.md),
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Text(
                  '{\n  "status": "ok",\n  "latency": "12ms",\n  "result": "操作成功"\n}',
                  style: AppTypography.bodySmall(context).copyWith(fontFamily: 'monospace'),
                ),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('关闭', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showUnregisterConfirm(BuildContext context, Map<String, dynamic> service) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('注销服务', style: AppTypography.cardTitle(context)),
          content: Text('确定要注销服务「${service['name']}」吗？注销后相关功能将不可用。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  _services.removeWhere((s) => s['id'] == service['id']);
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已注销服务：${service['name']}')));
              },
              child: Text('注销', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }

  void _showUnisolatedConfirm(BuildContext context, Map<String, dynamic> service) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('解除隔离', style: AppTypography.cardTitle(context)),
          content: Text('确定要解除服务「${service['name']}」的隔离状态吗？解除后服务将恢复可用。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  final idx = _services.indexWhere((s) => s['id'] == service['id']);
                  if (idx >= 0) {
                    _services[idx]['run_status'] = '已停止';
                    _services[idx]['isolated'] = false;
                  }
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已解除隔离：${service['name']}')));
              },
              child: Text('解除', style: TextStyle(color: context.success)),
            ),
          ],
        );
      },
    );
  }
}
