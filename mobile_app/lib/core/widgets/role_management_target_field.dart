import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_typography.dart';
import '../backend_transport/role_management_target.dart';
import '../runtime/backend/mobile_backend_providers.dart';
import '../runtime/backend/mobile_deployment_mode.dart';
import '../services/providers.dart';

class RoleManagementTargetField extends ConsumerWidget {
  const RoleManagementTargetField({
    super.key,
    required this.noun,
    required this.value,
    required this.onChanged,
    this.enabled = true,
  });

  final String noun;
  final RoleManagementTarget value;
  final ValueChanged<RoleManagementTarget> onChanged;
  final bool enabled;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (ref.watch(mobileDeploymentConfigProvider).mode !=
        MobileDeploymentMode.cloud) {
      return const SizedBox.shrink();
    }
    final coordination = ref.watch(deviceCoordinationProvider).valueOrNull;
    final centerAvailable = roleManagementCenterAvailable(coordination);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('创建位置', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.xs),
        for (final target in RoleManagementTarget.values)
          if (target == RoleManagementTarget.local || centerAvailable)
            RadioListTile<RoleManagementTarget>(
              value: target,
              groupValue: value,
              onChanged: enabled ? (next) => onChanged(next ?? value) : null,
              contentPadding: EdgeInsets.zero,
              dense: true,
              title: Text(
                target == RoleManagementTarget.local
                    ? '在本机创建$noun'
                    : '在统筹中心创建$noun',
                style: AppTypography.body(context),
              ),
              subtitle: Text(
                target.description,
                style: AppTypography.caption(
                  context,
                ).copyWith(color: context.textSecondary),
              ),
            ),
        if (!centerAvailable)
          Text(
            '未获得统筹中心管理员授权，只能在本机创建$noun',
            style: AppTypography.caption(
              context,
            ).copyWith(color: context.textTertiary),
          ),
      ],
    );
  }
}
