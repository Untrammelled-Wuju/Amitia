import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class RuntimePage extends ConsumerStatefulWidget {
  const RuntimePage({super.key});

  @override
  ConsumerState<RuntimePage> createState() => _RuntimePageState();
}

class _RuntimePageState extends ConsumerState<RuntimePage> {
  late String _status;
  late String _backendStatus;
  late List<RuntimeComponent> _components;

  @override
  void initState() {
    super.initState();
    final info = MockData.runtimeInfo;
    _status = info.status;
    _backendStatus = info.backendStatus;
    _components = List.of(info.components);
  }

  bool get _isRunning => _status == '运行中';

  void _start() {
    setState(() {
      _status = '运行中';
      _backendStatus = '已连接';
      _components = _components
          .map((c) => RuntimeComponent(name: c.name, status: '运行中'))
          .toList();
    });
  }

  void _stop() {
    setState(() {
      _status = '已停止';
      _backendStatus = '已断开';
      _components = _components
          .map((c) => RuntimeComponent(name: c.name, status: '已停止'))
          .toList();
    });
  }

  void _snack(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), duration: const Duration(seconds: 1)),
    );
  }

  @override
  Widget build(BuildContext context) {
    final info = MockData.runtimeInfo;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: 'Ubuntu Runtime', showBackButton: true),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          _StatusCard(
            status: _status,
            version: info.version,
            backendStatus: _backendStatus,
            storageUsage: info.storageUsage,
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
            child: Column(
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
            type: backendStatus == '已连接'
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
