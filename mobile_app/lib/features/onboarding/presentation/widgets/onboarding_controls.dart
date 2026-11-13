import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';

const double onboardingContentMaxWidth = 480;
const double onboardingOptionMinHeight = 72;
const double onboardingFieldHeight = 40;
const double onboardingActionHeight = 40;
const double onboardingInlineActionHeight = 36;
const double onboardingControlRadius = 7;

class OnboardingStepHeader extends StatelessWidget {
  const OnboardingStepHeader({
    super.key,
    required this.title,
    required this.subtitle,
  });

  final String title;
  final String subtitle;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: AppTypography.pageLargeTitle(
            context,
          ).copyWith(letterSpacing: -0.5, height: 1.2),
        ),
        const SizedBox(height: 10),
        Text(
          subtitle,
          style: AppTypography.caption(
            context,
          ).copyWith(height: 1.5, color: context.textTertiary),
        ),
      ],
    );
  }
}

class OnboardingOptionTile extends StatelessWidget {
  const OnboardingOptionTile({
    super.key,
    required this.icon,
    required this.title,
    required this.description,
    required this.selected,
    required this.onTap,
  });

  final Widget icon;
  final String title;
  final String description;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 150),
        constraints: const BoxConstraints(minHeight: onboardingOptionMinHeight),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        decoration: BoxDecoration(
          color: selected ? context.accentSoft : context.surfaceSecondary,
          borderRadius: BorderRadius.circular(onboardingControlRadius),
          border: Border.all(
            color: selected ? context.accentPrimary : context.borderPrimary,
          ),
        ),
        child: Row(
          children: [
            SizedBox(width: 28, height: 28, child: Center(child: icon)),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(title, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 4),
                  Text(
                    description,
                    style: AppTypography.caption(
                      context,
                    ).copyWith(color: context.textTertiary, height: 1.45),
                  ),
                ],
              ),
            ),
            if (selected)
              Icon(Icons.check, size: 18, color: context.accentPrimary),
          ],
        ),
      ),
    );
  }
}

class OnboardingField extends StatelessWidget {
  const OnboardingField({
    super.key,
    required this.label,
    this.controller,
    this.hintText,
    this.obscureText = false,
    this.keyboardType,
    this.onChanged,
    this.errorText,
    this.enabled = true,
  });

  final String label;
  final TextEditingController? controller;
  final String? hintText;
  final bool obscureText;
  final TextInputType? keyboardType;
  final ValueChanged<String>? onChanged;
  final String? errorText;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: AppTypography.label(
            context,
          ).copyWith(color: context.textSecondary),
        ),
        const SizedBox(height: 7),
        Container(
          height: onboardingFieldHeight,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(onboardingControlRadius),
            border: Border.all(
              color: errorText == null ? context.borderPrimary : context.error,
            ),
          ),
          child: TextField(
            controller: controller,
            obscureText: obscureText,
            keyboardType: keyboardType,
            enabled: enabled,
            onChanged: onChanged,
            textAlignVertical: TextAlignVertical.center,
            style: AppTypography.bodySmall(context).copyWith(fontSize: 13),
            decoration: InputDecoration(
              isDense: true,
              border: InputBorder.none,
              contentPadding: const EdgeInsets.symmetric(horizontal: 12),
              hintText: hintText,
              hintStyle: AppTypography.bodySmall(
                context,
              ).copyWith(fontSize: 13, color: context.textTertiary),
            ),
          ),
        ),
        if (errorText != null) ...[
          const SizedBox(height: 6),
          Text(
            errorText!,
            style: AppTypography.label(context).copyWith(color: context.error),
          ),
        ],
      ],
    );
  }
}

enum OnboardingCheckPhase { waiting, running, done, failed }

class OnboardingCheckState {
  const OnboardingCheckState({
    required this.name,
    required this.state,
    required this.stateText,
    this.detail,
  });

  final String name;
  final OnboardingCheckPhase state;
  final String stateText;
  final String? detail;

  static OnboardingCheckState waiting(String name) {
    return OnboardingCheckState(
      name: name,
      state: OnboardingCheckPhase.waiting,
      stateText: '等待',
    );
  }

  OnboardingCheckState copyWith({
    OnboardingCheckPhase? state,
    String? stateText,
    String? detail,
  }) {
    return OnboardingCheckState(
      name: name,
      state: state ?? this.state,
      stateText: stateText ?? this.stateText,
      detail: detail ?? this.detail,
    );
  }
}

class OnboardingCheckRow extends StatelessWidget {
  const OnboardingCheckRow({super.key, required this.check});

  final OnboardingCheckState check;

  @override
  Widget build(BuildContext context) {
    final color = switch (check.state) {
      OnboardingCheckPhase.waiting => context.textTertiary,
      OnboardingCheckPhase.running => context.accentPrimary,
      OnboardingCheckPhase.done => context.success,
      OnboardingCheckPhase.failed => context.error,
    };
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 13),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(color: context.borderSecondary, width: 1),
        ),
      ),
      child: Row(
        children: [
          Container(
            width: 6,
            height: 6,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              check.name,
              style: AppTypography.bodySmall(context).copyWith(fontSize: 13),
            ),
          ),
          const SizedBox(width: 12),
          Text(
            check.stateText,
            style: AppTypography.label(context).copyWith(color: color),
          ),
        ],
      ),
    );
  }
}

class OnboardingInlineAction extends StatelessWidget {
  const OnboardingInlineAction({
    super.key,
    required this.label,
    required this.onPressed,
    this.busy = false,
  });

  final String label;
  final VoidCallback? onPressed;
  final bool busy;

  @override
  Widget build(BuildContext context) {
    final enabled = onPressed != null && !busy;
    return Opacity(
      opacity: enabled ? 1 : 0.45,
      child: SizedBox(
        height: onboardingInlineActionHeight,
        child: Material(
          color: context.surfaceSecondary,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(onboardingControlRadius),
            side: BorderSide(color: context.borderPrimary),
          ),
          child: InkWell(
            borderRadius: BorderRadius.circular(onboardingControlRadius),
            onTap: enabled ? onPressed : null,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 14),
              child: Center(
                widthFactor: 1,
                child: Text(
                  busy ? '处理中' : label,
                  style: AppTypography.bodySmall(
                    context,
                  ).copyWith(fontSize: 13, color: context.textPrimary),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class OnboardingPillButton extends StatelessWidget {
  const OnboardingPillButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.primary = false,
  });

  final String label;
  final VoidCallback? onPressed;
  final bool primary;

  @override
  Widget build(BuildContext context) {
    final enabled = onPressed != null;
    return Opacity(
      opacity: enabled ? 1 : 0.4,
      child: SizedBox(
        height: onboardingActionHeight,
        child: Material(
          color: primary ? context.textPrimary : context.surfacePrimary,
          shape: StadiumBorder(
            side: BorderSide(
              color: primary ? context.textPrimary : context.borderPrimary,
            ),
          ),
          child: InkWell(
            customBorder: const StadiumBorder(),
            onTap: enabled ? onPressed : null,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 20),
              child: Center(
                widthFactor: 1,
                child: Text(
                  label,
                  style: AppTypography.bodySmall(context).copyWith(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: primary
                        ? context.surfacePrimary
                        : context.textPrimary,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class OnboardingErrorMessage extends StatelessWidget {
  const OnboardingErrorMessage({super.key, required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 16),
      child: SelectableText(
        message,
        style: AppTypography.label(context).copyWith(color: context.error),
      ),
    );
  }
}
