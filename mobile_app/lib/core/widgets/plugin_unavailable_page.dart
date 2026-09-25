import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../app/app_routes.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_typography.dart';
import 'amitia_button.dart';
import 'amitia_scaffold.dart';

class PluginUnavailablePage extends StatelessWidget {
  const PluginUnavailablePage({
    super.key,
    required this.title,
    required this.description,
    required this.fallbackRoute,
  });

  final String title;
  final String description;
  final String fallbackRoute;

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: title,
        showBackButton: true,
        fallbackRoute: fallbackRoute,
      ),
      body: Center(
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 64,
                  height: 64,
                  decoration: BoxDecoration(
                    color: context.surfaceSecondary,
                    borderRadius: BorderRadius.circular(18),
                  ),
                  child: Icon(
                    Icons.extension_off_outlined,
                    size: 30,
                    color: context.textSecondary,
                  ),
                ),
                SizedBox(height: AppSpacing.lg),
                Text(
                  '$title插件未安装或不可用',
                  style: AppTypography.sectionTitle(context),
                  textAlign: TextAlign.center,
                ),
                SizedBox(height: AppSpacing.sm),
                Text(
                  description,
                  style: AppTypography.bodySmall(
                    context,
                  ).copyWith(color: context.textSecondary),
                  textAlign: TextAlign.center,
                ),
                SizedBox(height: AppSpacing.lg),
                AmitiaButton(
                  label: '前往扩展中心',
                  isFullWidth: true,
                  onPressed: () => context.push(AppRoutes.extensions),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
