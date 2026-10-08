import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/services/core_configuration_guard.dart';

class ModelSettingsPage extends ConsumerWidget {
  const ModelSettingsPage({super.key});

  static const _types = <(IconData, String, String, String)>[
    (Icons.chat_outlined, '文本模型', '对话与文本生成', 'text'),
    (Icons.visibility_outlined, '视觉模型', '图像理解与描述', 'vision'),
    (Icons.record_voice_over_outlined, '语音模型', '语音合成与音色配置', 'voice'),
    (Icons.transcribe_outlined, '语音识别', '语音转文字与识别模型', 'asr'),
    (Icons.scatter_plot_outlined, '向量模型', '文本向量化与检索', 'vector'),
    (Icons.image_outlined, '图像生成模型', '文生图与图像编辑', 'image'),
  ];

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final access = ref.watch(coreConfigurationAccessProvider);
    final canConfigure = access.valueOrNull?.canConfigure == true;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '模型设置',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
        actions: [
          IconButton(
            tooltip: '刷新配置权限',
            onPressed: () => ref.invalidate(coreConfigurationAccessProvider),
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Text(
            access.when(
              loading: () => '正在确认模型配置权限…',
              error: (_, __) => '无法确认模型配置权限，请恢复 Core 连接后重试',
              data: (intent) => intent.canConfigure
                  ? '选择模型类型进行配置'
                  : 'AI 服务由云端 Core 全权提供，请在 Core 控制页面配置模型。管理员设备需开启统筹模式。',
            ),
            style: AppTypography.caption(context),
          ),
          SizedBox(height: AppSpacing.lg),
          ..._types.map(
            (t) => Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.md),
              child: _ModelTypeCard(
                icon: t.$1,
                title: t.$2,
                subtitle: t.$3,
                onTap: !canConfigure
                    ? null
                    : () => context.push(
                        t.$4 == 'asr'
                            ? AppRoutes.settingsAsr
                            : AppRoutes.modelConfig(t.$4),
                      ),
              ),
            ),
          ),
          SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}

class _ModelTypeCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback? onTap;

  const _ModelTypeCard({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: EdgeInsets.all(AppSpacing.cardPadding),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(icon, size: 22, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(subtitle, style: AppTypography.caption(context)),
                ],
              ),
            ),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }
}
