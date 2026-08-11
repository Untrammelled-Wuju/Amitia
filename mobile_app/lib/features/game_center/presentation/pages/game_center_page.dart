import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/center_navigation.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';

class GameCenterPage extends ConsumerWidget {
  const GameCenterPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '游戏中心',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            Expanded(
              child: AmitiaEmptyState(
                icon: Icons.sports_esports_outlined,
                title: '暂无已连接的游戏',
                subtitle: '游戏中心功能即将上线，敬请期待',
              ),
            ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
              child: AmitiaButton(
                label: '插件管理',
                icon: Icons.extension_outlined,
                isFullWidth: true,
                isSecondary: true,
                onPressed: () => CenterNavigation.openExtensionCenter(context),
              ),
            ),
            const SizedBox(height: AppSpacing.sectionGap),
          ],
        ),
      ),
    );
  }
}
