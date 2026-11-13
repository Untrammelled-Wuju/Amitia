import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import 'onboarding_controls.dart';

class OnboardingStepScaffold extends StatelessWidget {
  const OnboardingStepScaffold({
    super.key,
    required this.stepIndex,
    required this.stepCount,
    required this.stepName,
    required this.title,
    required this.subtitle,
    required this.child,
    required this.nextLabel,
    required this.onNext,
    this.onBack,
    this.errorText,
  });

  final int stepIndex;
  final int stepCount;
  final String stepName;
  final String title;
  final String subtitle;
  final Widget child;
  final String nextLabel;
  final VoidCallback? onNext;
  final VoidCallback? onBack;
  final String? errorText;

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      color: context.surfacePrimary,
      child: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 18, 20, 0),
              child: _buildProgress(context),
            ),
            Expanded(
              child: SingleChildScrollView(
                padding: const EdgeInsets.fromLTRB(20, 26, 20, 28),
                child: Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(
                      maxWidth: onboardingContentMaxWidth,
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        OnboardingStepHeader(title: title, subtitle: subtitle),
                        const SizedBox(height: 30),
                        child,
                        if (errorText != null)
                          OnboardingErrorMessage(message: errorText!),
                      ],
                    ),
                  ),
                ),
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 0, 20, 18),
              child: Row(
                children: [
                  if (onBack != null)
                    OnboardingPillButton(
                      label: '上一步',
                      onPressed: onBack,
                    )
                  else
                    const SizedBox.shrink(),
                  const Spacer(),
                  OnboardingPillButton(
                    label: nextLabel,
                    onPressed: onNext,
                    primary: true,
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildProgress(BuildContext context) {
    return Row(
      children: [
        Text(
          '步骤 ${stepIndex + 1}/$stepCount',
          style: AppTypography.label(context),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Row(
            children: List.generate(stepCount, (index) {
              return Expanded(
                child: Padding(
                  padding: EdgeInsets.only(right: index == stepCount - 1 ? 0 : 4),
                  child: Container(
                    height: 2,
                    decoration: BoxDecoration(
                      color: index <= stepIndex
                          ? context.textPrimary
                          : context.borderPrimary,
                      borderRadius: BorderRadius.circular(1),
                    ),
                  ),
                ),
              );
            }),
          ),
        ),
        const SizedBox(width: 12),
        Text(
          stepName,
          style: AppTypography.label(context).copyWith(
            color: context.textSecondary,
          ),
        ),
      ],
    );
  }
}
