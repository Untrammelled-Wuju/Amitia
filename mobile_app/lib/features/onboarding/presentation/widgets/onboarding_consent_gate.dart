import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../privacy/presentation/widgets/privacy_sections.dart';
import 'onboarding_controls.dart';

class OnboardingConsentGate extends StatelessWidget {
  const OnboardingConsentGate({
    super.key,
    required this.agreed,
    required this.onToggleAgreed,
    required this.onContinue,
  });

  final bool agreed;
  final VoidCallback onToggleAgreed;
  final VoidCallback onContinue;

  static const _points = <({IconData icon, String title, String description})>[
    (
      icon: Icons.lock_outline,
      title: '数据由你自己掌控',
      description: '本产品由你自行部署，所有数据保存在你自己的设备或服务器上，没有第三方可以访问你的数据。',
    ),
    (
      icon: Icons.warning_amber_outlined,
      title: '模型 API 数据提醒',
      description: '使用 AI 聊天时，对话内容会发送给你配置的模型服务商。请不要发送验证码、密码、银行卡号等敏感信息。',
    ),
    (
      icon: Icons.chat_bubble_outline,
      title: 'AI 是虚拟角色',
      description: 'AI 陪伴角色由大语言模型驱动，不是真人。请理性看待 AI 的回复，不要将情感完全寄托于 AI。',
    ),
    (
      icon: Icons.delete_outline,
      title: '你可以随时删除数据',
      description: '聊天记录、记忆、导入记录均可在设置中一键清空。',
    ),
  ];

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      color: context.surfacePrimary,
      child: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: SingleChildScrollView(
                padding: const EdgeInsets.fromLTRB(20, 28, 20, 24),
                child: Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(
                      maxWidth: onboardingContentMaxWidth,
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        const OnboardingStepHeader(
                          title: '欢迎使用阿米提亚',
                          subtitle: '使用本产品前，请阅读并确认以下隐私说明和使用边界。',
                        ),
                        const SizedBox(height: 30),
                        for (var index = 0; index < _points.length; index++)
                          _ConsentPoint(
                            icon: _points[index].icon,
                            title: _points[index].title,
                            description: _points[index].description,
                            last: index == _points.length - 1,
                          ),
                        const SizedBox(height: 20),
                        Align(
                          alignment: Alignment.centerLeft,
                          child: GestureDetector(
                            behavior: HitTestBehavior.opaque,
                            onTap: () => _openTerms(context),
                            child: Text(
                              '查看完整隐私说明与使用边界',
                              style: AppTypography.label(context).copyWith(
                                color: context.accentPrimary,
                                decoration: TextDecoration.underline,
                                decorationColor: context.accentPrimary,
                              ),
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 0, 20, 18),
              child: Column(
                children: [
                  GestureDetector(
                    behavior: HitTestBehavior.opaque,
                    onTap: onToggleAgreed,
                    child: Row(
                      children: [
                        Icon(
                          agreed
                              ? Icons.check_box
                              : Icons.check_box_outline_blank,
                          size: 20,
                          color: agreed
                              ? context.accentPrimary
                              : context.textTertiary,
                        ),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Text(
                            '我已阅读并同意隐私说明和使用边界',
                            style: AppTypography.label(context),
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 14),
                  Align(
                    alignment: Alignment.centerRight,
                    child: OnboardingPillButton(
                      label: '开始使用',
                      primary: true,
                      onPressed: agreed ? onContinue : null,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _openTerms(BuildContext context) {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfaceSecondary,
      builder: (sheetContext) {
        return FractionallySizedBox(
          heightFactor: 0.92,
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 16, 12, 8),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        '隐私与使用边界',
                        style: AppTypography.cardTitle(sheetContext),
                      ),
                    ),
                    IconButton(
                      icon: const Icon(Icons.close, size: 20),
                      onPressed: () => Navigator.of(sheetContext).pop(),
                    ),
                  ],
                ),
              ),
              Expanded(
                child: PrivacySectionsList(
                  padding: const EdgeInsets.fromLTRB(20, 4, 20, 24),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _ConsentPoint extends StatelessWidget {
  const _ConsentPoint({
    required this.icon,
    required this.title,
    required this.description,
    required this.last,
  });

  final IconData icon;
  final String title;
  final String description;
  final bool last;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 14),
      decoration: BoxDecoration(
        border: last
            ? null
            : Border(
                bottom: BorderSide(color: context.borderSecondary, width: 1),
              ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 20, color: context.textSecondary),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: AppTypography.bodySmall(context).copyWith(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 5),
                Text(
                  description,
                  style: AppTypography.caption(
                    context,
                  ).copyWith(color: context.textTertiary, height: 1.55),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
