import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class PrivacyPolicyPage extends StatelessWidget {
  const PrivacyPolicyPage({super.key});

  static const _sections = <(String, IconData, List<String>)>[
    ('数据收集与存储', Icons.storage_outlined, [
      'Amitia 会保存维持账号、对话、角色、记忆和运行配置所需的数据。',
      '本地模式的数据由本机 Runtime 处理；云端模式的数据由你配置的 Cloud Core 处理。',
      '聊天、记忆和相关业务数据可通过应用提供的删除、清理或备份功能进行管理。',
    ]),
    ('模型服务', Icons.psychology_outlined, [
      '当你配置第三方模型服务时，请求会按照对应模型配置发送给该服务提供方。',
      'Amitia 不会在界面中把未配置的外部模型服务描述为已经启用。',
    ]),
    ('权限与工具调用', Icons.admin_panel_settings_outlined, [
      '相机、麦克风、文件等系统权限由操作系统授权，并可在“系统权限”中查看。',
      '扩展、MCP、Agent Skills 等工具能力受现有权限与运行时策略约束。',
    ]),
    ('设备与云端协同', Icons.devices_outlined, [
      '启用云端模式并绑定设备后，Device Mesh 会使用设备凭据维护云端协同连接。',
      '你可以在“我的设备”中查看或移除云端登记设备，也可以在设备设置中解除本机云端凭据。',
    ]),
  ];

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: const AmitiaAppBar(title: '隐私政策', navigation: AmitiaAppBarNavigation.back),
      body: ListView(
        padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.md, AppSpacing.pagePadding, AppSpacing.xl),
        children: [
          Text('本页面说明当前应用实际的数据与权限边界。具体部署的数据处理范围以你的本地/云端配置和已启用扩展为准。', style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.lg),
          for (final section in _sections) ...[
            _PolicyCard(title: section.$1, icon: section.$2, paragraphs: section.$3),
            SizedBox(height: AppSpacing.md),
          ],
        ],
      ),
    );
  }
}

class _PolicyCard extends StatelessWidget {
  final String title;
  final IconData icon;
  final List<String> paragraphs;
  const _PolicyCard({required this.title, required this.icon, required this.paragraphs});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.6),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(children: [
            Container(width: 32, height: 32, decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: BorderRadius.circular(11)), child: Icon(icon, size: 17, color: context.textSecondary)),
            const SizedBox(width: 10),
            Expanded(child: Text(title, style: AppTypography.cardTitle(context))),
          ]),
          const SizedBox(height: 10),
          for (final text in paragraphs) Padding(
            padding: const EdgeInsets.only(bottom: 7),
            child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Padding(padding: const EdgeInsets.only(top: 7), child: Container(width: 4, height: 4, decoration: BoxDecoration(color: context.accentPrimary, shape: BoxShape.circle))),
              const SizedBox(width: 8),
              Expanded(child: Text(text, style: AppTypography.bodySmall(context))),
            ]),
          ),
        ],
      ),
    );
  }
}
