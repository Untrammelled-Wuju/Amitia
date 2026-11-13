import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../widgets/onboarding_controls.dart';

class OnboardingCenterStep extends StatelessWidget {
  const OnboardingCenterStep({
    super.key,
    required this.deployMode,
    required this.remoteCoreController,
    required this.onModeChanged,
    required this.onRemoteCoreChanged,
    required this.detecting,
    required this.statusText,
    required this.onCheckRemote,
  });

  final int deployMode;
  final TextEditingController remoteCoreController;
  final ValueChanged<int> onModeChanged;
  final ValueChanged<String> onRemoteCoreChanged;
  final bool detecting;
  final String statusText;
  final VoidCallback onCheckRemote;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        OnboardingOptionTile(
          icon: Icon(
            Icons.home_outlined,
            size: 22,
            color: context.textPrimary,
          ),
          title: '以本机作为统筹中心',
          description: '当前设备保存统筹数据和模型配置，并管理其他设备',
          selected: deployMode == 0,
          onTap: () => onModeChanged(0),
        ),
        const SizedBox(height: 10),
        OnboardingOptionTile(
          icon: Icon(
            Icons.wifi_tethering,
            size: 22,
            color: context.textPrimary,
          ),
          title: '使用远程统筹中心',
          description: '连接另一台设备提供的统筹中心',
          selected: deployMode == 1,
          onTap: () => onModeChanged(1),
        ),
        if (deployMode == 1) ...[
          const SizedBox(height: 26),
          OnboardingField(
            label: '统筹中心地址',
            controller: remoteCoreController,
            hintText: 'https://amitia.example.com',
            keyboardType: TextInputType.url,
            onChanged: onRemoteCoreChanged,
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              OnboardingInlineAction(
                label: '检测连接',
                busy: detecting,
                onPressed: onCheckRemote,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  statusText,
                  style: AppTypography.caption(context).copyWith(
                    color: statusText == '连接成功'
                        ? context.success
                        : statusText.isEmpty
                        ? context.textTertiary
                        : context.error,
                  ),
                ),
              ),
            ],
          ),
        ],
      ],
    );
  }
}
