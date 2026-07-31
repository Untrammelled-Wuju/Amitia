import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/mock_data/mock_data.dart';

class GameCenterPage extends ConsumerWidget {
  const GameCenterPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '游戏中心',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.only(bottom: AppSpacing.xxl),
          children: [
            _buildConnectionCard(context),
            const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '已安装游戏插件'),
            const SizedBox(height: AppSpacing.sm),
            _buildInstalledPluginsCard(context),
            const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '可用游戏插件'),
            const SizedBox(height: AppSpacing.sm),
            ..._buildAvailablePluginCards(context),
            const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '最近游戏任务'),
            const SizedBox(height: AppSpacing.sm),
            _buildTasksCard(context),
            const SizedBox(height: AppSpacing.sectionGap),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
              child: AmitiaButton(
                label: '插件管理',
                icon: Icons.extension_outlined,
                isFullWidth: true,
                isSecondary: true,
                onPressed: () => context.push(AppRoutes.extensions),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildConnectionCard(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                color: context.warning.withValues(alpha: 0.12),
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(Icons.link_off, size: 24, color: context.warning),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('游戏连接', style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text('未连接', style: AppTypography.caption(context)),
                ],
              ),
            ),
            AmitiaButton(
              label: '连接',
              isSecondary: true,
              height: 36,
              onPressed: () => amitiaComingSoon(context, '游戏连接'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInstalledPluginsCard(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Column(
          children: [
            for (int i = 0; i < MockData.gamePlugins.length; i++) ...[
              _buildPluginItem(context, MockData.gamePlugins[i]),
              if (i < MockData.gamePlugins.length - 1)
                Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildPluginItem(BuildContext context, String name) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 10),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brExtraSmall,
            ),
            child: Icon(Icons.sports_esports_outlined, size: 18, color: context.accentPrimary),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Text(name, style: AppTypography.body(context)),
          ),
          Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
        ],
      ),
    );
  }

  List<Widget> _buildAvailablePluginCards(BuildContext context) {
    final plugins = [
      ('游戏数据统计', '记录和分析游戏数据', Icons.analytics_outlined),
      ('自动签到', '自动完成每日签到任务', Icons.check_circle_outline),
      ('游戏攻略', '提供游戏攻略和指南', Icons.menu_book_outlined),
    ];

    return plugins.map((p) {
      return Padding(
        padding: const EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          0,
          AppSpacing.pagePadding,
          AppSpacing.sm,
        ),
        child: AmitiaCard(
          child: Row(
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(p.$3, size: 20, color: context.accentPrimary),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(p.$1, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text(p.$2, style: AppTypography.caption(context)),
                  ],
                ),
              ),
              GestureDetector(
                onTap: () => amitiaComingSoon(context, '安装插件'),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                  decoration: BoxDecoration(
                    color: context.accentPrimary,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: const Text(
                    '安装',
                    style: TextStyle(fontSize: 13, color: Colors.white, fontWeight: FontWeight.w500),
                  ),
                ),
              ),
            ],
          ),
        ),
      );
    }).toList();
  }

  Widget _buildTasksCard(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Column(
          children: [
            for (int i = 0; i < MockData.gameTasks.length; i++) ...[
              _buildTaskItem(context, MockData.gameTasks[i]),
              if (i < MockData.gameTasks.length - 1)
                Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildTaskItem(BuildContext context, String name) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 10),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: context.info.withValues(alpha: 0.12),
              borderRadius: AppRadius.brExtraSmall,
            ),
            child: Icon(Icons.schedule, size: 18, color: context.info),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Text(name, style: AppTypography.body(context)),
          ),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
            decoration: BoxDecoration(
              color: context.borderSecondary,
              borderRadius: AppRadius.brTag,
            ),
            child: Text(
              '待执行',
              style: TextStyle(fontSize: 11, color: context.textTertiary),
            ),
          ),
        ],
      ),
    );
  }
}
