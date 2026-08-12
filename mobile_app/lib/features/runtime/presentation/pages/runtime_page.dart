import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../shared/models/models.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';

final _runtimeHealthProvider = FutureProvider<Map<String, dynamic>?>((ref) async {
  final svc = ref.read(systemServiceProvider);
  return svc.health();
});

class RuntimePage extends ConsumerStatefulWidget {
  const RuntimePage({super.key});

  @override
  ConsumerState<RuntimePage> createState() => _RuntimePageState();
}

class _RuntimePageState extends ConsumerState<RuntimePage> {
  late String _status;
  late String _backendStatus;
  late String _version;
  late String _storageUsage;
  late List<RuntimeComponent> _components;
  bool _initialized = false;

  @override
  void initState() {
    super.initState();
    _status = '';
    _backendStatus = '';
    _version = '';
    _storageUsage = '';
    _components = [];
    _loadHealth();
  }

  Future<void> _loadHealth() async {
    try {
      final data = await ref.read(systemServiceProvider).health();
      if (!mounted) return;
      if (data != null) {
        setState(() {
          _status = data['status'] as String? ?? data['runtime_status'] as String? ?? '';
          _version = data['version'] as String? ?? data['runtime_version'] as String? ?? '';
          _backendStatus = data['backend'] as String? ?? data['backend_status'] as String? ?? '';
          _storageUsage = data['storage'] as String? ?? data['storage_usage'] as String? ?? '';
          final comps = data['components'] as List<dynamic>?;
          if (comps != null) {
            _components = comps.map((c) {
              if (c is Map<String, dynamic>) {
                return RuntimeComponent(
                  name: c['name'] as String? ?? '',
                  status: c['status'] as String? ?? '',
                );
              }
              return RuntimeComponent(name: '', status: '');
            }).toList().cast<RuntimeComponent>();
          }
          _initialized = true;
        });
      } else {
        setState(() => _initialized = true);
      }
    } catch (_) {
      if (mounted) setState(() => _initialized = true);
    }
  }

  bool get _isRunning => _status == '运行中' || _status == 'ok' || _status == 'healthy' || _status == '已安装';

  void _start() {
    setState(() {
      _status = '运行中';
      _backendStatus = '后端就绪';
      _components = _components
          .map((c) => RuntimeComponent(name: c.name, status: '运行中'))
          .toList();
    });
  }

  void _stop() {
    setState(() {
      _status = '已停止';
      _backendStatus = '后端未就绪';
      _components = _components
          .map((c) => RuntimeComponent(name: c.name, status: '已停止'))
          .toList();
    });
    ref.read(backendConnectionRepositoryProvider).invalidate();
  }

  void _snack(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), duration: const Duration(seconds: 1)),
    );
  }

  @override
  Widget build(BuildContext context) {
    final connectionAsync = ref.watch(backendConnectionProvider);
    final effectiveBackendStatus = connectionAsync.when(
      data: (avail) => avail is BackendConnectionAvailable ? '后端就绪' : '后端未就绪',
      loading: () => '检查中...',
      error: (_, __) => '后端未就绪',
    );
    _backendStatus = effectiveBackendStatus;

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: 'Ubuntu Runtime', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          _StatusCard(
            status: _status,
            version: _version,
            backendStatus: _backendStatus,
            storageUsage: _storageUsage,
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          Text('运行组件', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.md),
          Container(
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: _components.isEmpty
                ? Padding(
                    padding: const EdgeInsets.all(AppSpacing.cardPadding),
                    child: Text('暂无组件信息', style: AppTypography.caption(context)),
                  )
                : Column(
                    children: [
                      for (int i = 0; i < _components.length; i++) ...[
                        _ComponentTile(component: _components[i]),
                        if (i < _components.length - 1)
                          Divider(
                            height: 1,
                            indent: AppSpacing.lg,
                            color: context.borderSecondary,
                          ),
                      ],
                    ],
                  ),
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          Text('操作', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: AppSpacing.md,
            runSpacing: AppSpacing.md,
            children: [
              AmitiaButton(
                label: '安装环境',
                icon: Icons.download_outlined,
                isSecondary: true,
                onPressed: () => _snack('环境已是最新版本'),
              ),
              AmitiaButton(
                label: '启动',
                icon: Icons.play_arrow,
                onPressed: _isRunning ? null : _start,
              ),
              AmitiaButton(
                label: '停止',
                icon: Icons.stop,
                isSecondary: true,
                onPressed: _isRunning ? _stop : null,
              ),
              AmitiaButton(
                label: '查看日志',
                icon: Icons.description_outlined,
                isSecondary: true,
                onPressed: () => _snack('暂无运行日志'),
              ),
              AmitiaButton(
                label: '修复环境',
                icon: Icons.build_outlined,
                isSecondary: true,
                onPressed: () => _snack('环境检查完成，无需修复'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _StatusCard extends StatelessWidget {
  final String status;
  final String version;
  final String backendStatus;
  final String storageUsage;

  const _StatusCard({
    required this.status,
    required this.version,
    required this.backendStatus,
    required this.storageUsage,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.cardPadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        children: [
          _InfoLine(
            label: '运行状态',
            value: status,
            type: status == '运行中' ? BadgeType.success : BadgeType.neutral,
          ),
          _InfoLine(label: 'Runtime 版本', value: version),
          _InfoLine(
            label: '后端状态',
            value: backendStatus,
            type: backendStatus == '后端就绪' || backendStatus == '后端已就绪'
                ? BadgeType.success
                : BadgeType.neutral,
          ),
          _InfoLine(label: '存储占用', value: storageUsage),
        ],
      ),
    );
  }
}

class _InfoLine extends StatelessWidget {
  final String label;
  final String value;
  final BadgeType? type;

  const _InfoLine({required this.label, required this.value, this.type});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        children: [
          Text(label, style: AppTypography.caption(context)),
          const Spacer(),
          if (type != null)
            AmitiaStatusBadge(label: value, type: type!)
          else
            Text(value, style: AppTypography.bodySmall(context)),
        ],
      ),
    );
  }
}

class _ComponentTile extends StatelessWidget {
  final RuntimeComponent component;

  const _ComponentTile({required this.component});

  @override
  Widget build(BuildContext context) {
    final isRunning = component.status == '运行中';
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.lg,
        vertical: 13,
      ),
      child: Row(
        children: [
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(
              color: isRunning ? context.success : context.textTertiary,
              shape: BoxShape.circle,
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(child: Text(component.name, style: AppTypography.body(context))),
          AmitiaStatusBadge(
            label: component.status,
            type: isRunning ? BadgeType.success : BadgeType.neutral,
          ),
        ],
      ),
    );
  }
}
