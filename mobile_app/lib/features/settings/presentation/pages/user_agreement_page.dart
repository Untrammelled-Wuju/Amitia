import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

final _agreementAboutProvider = FutureProvider.autoDispose<Map<String, dynamic>?>((ref) {
  return ref.read(systemServiceProvider).about();
});

class UserAgreementPage extends ConsumerWidget {
  const UserAgreementPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final about = ref.watch(_agreementAboutProvider).asData?.value;
    final license = (about?['license'] ?? 'AGPL-3.0-only').toString();
    final sections = <(String, IconData, String)>[
      ('服务使用', Icons.check_circle_outline, '你可以在本地模式或自己配置的 Cloud Core 中使用 Amitia。具体可用能力取决于当前部署、模型、扩展和设备运行状态。'),
      ('个人空间与设备', Icons.person_outline, 'Amitia 不创建产品账号。Space ID 用于数据归属，Device ID / Runtime ID 用于执行路由，云端设备通过一次性配对签发的 Device Credential 鉴权。'),
      ('扩展与工具', Icons.extension_outlined, '扩展包、MCP、Agent Skills 及其它工具可能访问你授权的数据或设备能力。启用前应确认来源和权限范围。'),
      ('开源许可', Icons.code_outlined, '当前后端返回的开源许可为 $license。第三方依赖分别遵循各自许可证。'),
    ];
    return AmitiaScaffold(
      appBar: const AmitiaAppBar(title: '用户协议', navigation: AmitiaAppBarNavigation.back),
      body: ListView(
        padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.md, AppSpacing.pagePadding, AppSpacing.xl),
        children: [
          Text('继续使用 Amitia 代表你同意按当前部署能力和权限边界使用服务。', style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.lg),
          for (final section in sections) ...[
            Container(
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(color: context.surfacePrimary, borderRadius: AppRadius.brMedium, border: Border.all(color: context.borderPrimary, width: 0.6)),
              child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Container(width: 32, height: 32, decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: BorderRadius.circular(11)), child: Icon(section.$2, size: 17, color: context.textSecondary)),
                const SizedBox(width: 11),
                Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Text(section.$1, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 5),
                  Text(section.$3, style: AppTypography.bodySmall(context)),
                ])),
              ]),
            ),
            SizedBox(height: AppSpacing.md),
          ],
        ],
      ),
    );
  }
}
