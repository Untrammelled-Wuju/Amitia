import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../widgets/onboarding_controls.dart';

class OnboardingRuntimeStep extends StatelessWidget {
  const OnboardingRuntimeStep({
    super.key,
    required this.checks,
    required this.connectionFailed,
    required this.failureDetail,
    required this.onRetry,
    required this.pairingRequired,
    required this.firstDeviceSetupRequired,
    required this.pairingController,
    required this.pairingBusy,
    required this.pairingError,
    required this.onPair,
    required this.pairingVerified,
  });

  final List<OnboardingCheckState> checks;
  final bool connectionFailed;
  final String failureDetail;
  final VoidCallback onRetry;
  final bool pairingRequired;
  final bool firstDeviceSetupRequired;
  final TextEditingController pairingController;
  final bool pairingBusy;
  final String pairingError;
  final VoidCallback onPair;
  final bool pairingVerified;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (final check in checks) OnboardingCheckRow(check: check),
        if (connectionFailed) ...[
          const SizedBox(height: 18),
          OnboardingErrorMessage(
            message: failureDetail.isEmpty ? '连接检查未通过' : failureDetail,
          ),
          const SizedBox(height: 12),
          Align(
            alignment: Alignment.centerLeft,
            child: OnboardingInlineAction(
              label: '重新检查',
              onPressed: onRetry,
            ),
          ),
        ],
        if (pairingRequired) ...[
          const SizedBox(height: 26),
          OnboardingField(
            label: firstDeviceSetupRequired ? '首设备设置码' : '配对二维码',
            controller: pairingController,
            hintText: firstDeviceSetupRequired
                ? '输入首设备设置码'
                : 'amitia://pair?endpoint=...&offer=...',
            enabled: !pairingBusy,
          ),
          const SizedBox(height: 10),
          Text(
            firstDeviceSetupRequired
                ? '这是该统筹中心的第一台可信设备。设置码只在中心设备本地生成，不会通过网络公开。'
                : '请从任意已信任设备生成 amitia://pair?... 二维码，也可以直接粘贴 Offer Token。',
            style: AppTypography.caption(
              context,
            ).copyWith(color: context.textTertiary, height: 1.5),
          ),
          if (pairingError.isNotEmpty) ...[
            const SizedBox(height: 10),
            SelectableText(
              pairingError,
              style: AppTypography.label(
                context,
              ).copyWith(color: context.error),
            ),
          ],
          const SizedBox(height: 14),
          Align(
            alignment: Alignment.centerLeft,
            child: OnboardingInlineAction(
              label: pairingBusy ? '配对中' : '配对当前设备',
              busy: pairingBusy,
              onPressed: onPair,
            ),
          ),
        ],
        if (pairingVerified) ...[
          const SizedBox(height: 22),
          Text(
            '当前设备已通过 DeviceCredential 验证，可以继续下一步。',
            style: AppTypography.caption(
              context,
            ).copyWith(color: context.success, height: 1.5),
          ),
        ],
      ],
    );
  }
}
