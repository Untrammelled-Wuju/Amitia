import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../widgets/onboarding_controls.dart';

class OnboardingModelStep extends StatelessWidget {
  const OnboardingModelStep({
    super.key,
    required this.providerController,
    required this.baseUrlController,
    required this.modelController,
    required this.keyController,
    required this.detecting,
    required this.statusText,
    required this.statusOk,
    required this.detectedModels,
    required this.onDetect,
    required this.onSelectModel,
    required this.onFieldChanged,
    required this.showKey,
    required this.onToggleKey,
  });

  final TextEditingController providerController;
  final TextEditingController baseUrlController;
  final TextEditingController modelController;
  final TextEditingController keyController;
  final bool detecting;
  final String statusText;
  final bool statusOk;
  final List<String> detectedModels;
  final ValueChanged<String> onSelectModel;
  final VoidCallback onDetect;
  final VoidCallback onFieldChanged;
  final bool showKey;
  final VoidCallback onToggleKey;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        OnboardingField(
          label: '服务商',
          controller: providerController,
          hintText: '例如 DeepSeek、OpenAI',
          onChanged: (_) => onFieldChanged(),
        ),
        const SizedBox(height: 16),
        OnboardingField(
          label: '服务地址',
          controller: baseUrlController,
          hintText: 'https://api.deepseek.com/v1',
          keyboardType: TextInputType.url,
          onChanged: (_) => onFieldChanged(),
        ),
        const SizedBox(height: 16),
        OnboardingField(
          label: 'API Key',
          controller: keyController,
          hintText: 'sk-...',
          obscureText: !showKey,
          onChanged: (_) => onFieldChanged(),
        ),
        const SizedBox(height: 6),
        Align(
          alignment: Alignment.centerRight,
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: onToggleKey,
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: 6),
              child: Text(
                showKey ? '隐藏密钥' : '显示密钥',
                style: AppTypography.label(
                  context,
                ).copyWith(color: context.accentPrimary),
              ),
            ),
          ),
        ),
        const SizedBox(height: 10),
        OnboardingField(
          label: '模型名称',
          controller: modelController,
          hintText: '例如 deepseek-chat',
          onChanged: (_) => onFieldChanged(),
        ),
        const SizedBox(height: 14),
        Row(
          children: [
            OnboardingInlineAction(
              label: statusOk ? '重新检测' : '检测模型',
              busy: detecting,
              onPressed: onDetect,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                statusText,
                style: AppTypography.caption(context).copyWith(
                  color: statusOk
                      ? context.success
                      : statusText.isEmpty
                      ? context.textTertiary
                      : context.textSecondary,
                  height: 1.45,
                ),
              ),
            ),
          ],
        ),
        if (detectedModels.isNotEmpty) ...[
          const SizedBox(height: 16),
          Text(
            '检测到的模型',
            style: AppTypography.label(context).copyWith(
              color: context.textSecondary,
            ),
          ),
          const SizedBox(height: 10),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final model in detectedModels)
                _ModelChip(
                  label: model,
                  selected: modelController.text.trim() == model,
                  onTap: () => onSelectModel(model),
                ),
            ],
          ),
        ],
      ],
    );
  }
}

class _ModelChip extends StatelessWidget {
  const _ModelChip({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: selected ? context.accentSoft : context.surfaceSecondary,
          borderRadius: BorderRadius.circular(onboardingControlRadius),
          border: Border.all(
            color: selected ? context.accentPrimary : context.borderPrimary,
          ),
        ),
        child: Text(
          label,
          style: AppTypography.label(context).copyWith(
            color: selected ? context.accentPrimary : context.textPrimary,
          ),
        ),
      ),
    );
  }
}
