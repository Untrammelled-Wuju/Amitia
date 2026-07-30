import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';

class NotFoundPage extends ConsumerWidget {
  final String? attemptedPath;

  const NotFoundPage({super.key, this.attemptedPath});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return AmitiaScaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Spacer(),
              Container(
                width: 120,
                height: 120,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.travel_explore_outlined,
                  size: 60,
                  color: context.accentPrimary,
                ),
              ),
              const SizedBox(height: AppSpacing.xl),
              Text(
                '404',
                style: TextStyle(
                  fontSize: 48,
                  fontWeight: FontWeight.w700,
                  color: context.accentPrimary,
                  height: 1.0,
                ),
              ),
              const SizedBox(height: AppSpacing.md),
              Text(
                '无法找到页面',
                style: AppTypography.pageLargeTitle(context),
              ),
              const SizedBox(height: AppSpacing.sm),
              Text(
                '你访问的页面可能已被移动、删除或从未存在',
                style: AppTypography.body(context).copyWith(color: context.textSecondary),
                textAlign: TextAlign.center,
              ),
              if (attemptedPath != null) ...[
                const SizedBox(height: AppSpacing.md),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: AppSpacing.sm),
                  decoration: BoxDecoration(
                    color: context.surfaceSecondary,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Text(
                    '路径: $attemptedPath',
                    style: AppTypography.label(context).copyWith(
                      fontFamily: 'monospace',
                      color: context.textTertiary,
                    ),
                  ),
                ),
              ],
              const SizedBox(height: AppSpacing.xxl),
              AmitiaButton(
                label: '返回上一页',
                icon: Icons.arrow_back,
                isFullWidth: true,
                onPressed: () {
                  if (context.canPop()) {
                    context.pop();
                  } else {
                    context.go(AppRoutes.chat);
                  }
                },
              ),
              const SizedBox(height: AppSpacing.md),
              AmitiaButton(
                label: '返回聊天页',
                icon: Icons.chat_bubble_outline,
                isSecondary: true,
                isFullWidth: true,
                onPressed: () => context.go(AppRoutes.chat),
              ),
              const Spacer(flex: 2),
            ],
          ),
        ),
      ),
    );
  }
}
