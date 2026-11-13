import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';

enum PrivacySectionTone { accent, info, warning, success, secondary, error, muted }

class PrivacySectionData {
  const PrivacySectionData({
    required this.icon,
    required this.title,
    required this.paragraphs,
    required this.tone,
  });

  final IconData icon;
  final String title;
  final List<String> paragraphs;
  final PrivacySectionTone tone;
}

const privacySections = <PrivacySectionData>[
  PrivacySectionData(
    icon: Icons.shield_outlined,
    title: '一、数据收集与存储',
    tone: PrivacySectionTone.accent,
    paragraphs: [
      'Amitia 采用本地优先架构，你的对话记录、记忆数据、角色设定等核心数据均存储在本地设备的加密数据库中。',
      '本机作为统筹中心时，统筹数据与模型配置保存在当前设备。',
      '使用远程统筹中心时，统筹数据由你指定的中心设备管理。',
    ],
  ),
  PrivacySectionData(
    icon: Icons.psychology_outlined,
    title: '二、AI 模型调用',
    tone: PrivacySectionTone.info,
    paragraphs: [
      'Amitia 在对话和任务处理过程中会调用你配置的 AI 模型（如 OpenAI GPT-4、Anthropic Claude 等）。',
      '当你发送消息时，相关的对话上下文将被发送至模型服务商进行推理。请勿在对话中输入敏感的个人信息（如身份证号、银行卡号等）。',
      '模型服务商的数据处理遵循其各自的隐私政策，Amitia 不控制也不对模型服务商的数据处理行为承担责任。',
    ],
  ),
  PrivacySectionData(
    icon: Icons.build_outlined,
    title: '三、Agent 模式与工具调用',
    tone: PrivacySectionTone.warning,
    paragraphs: [
      'Agent 模式下，Amitia 可以执行文件操作、系统命令、网络请求等高权限操作。',
      '所有高风险操作在执行前都需要你的明确确认。你可以选择"此次允许"、"始终允许"或"拒绝"。',
      '建议你定期在设置 > 系统权限中审查已授权的权限列表，及时撤销不再需要的权限。',
    ],
  ),
  PrivacySectionData(
    icon: Icons.memory,
    title: '四、记忆系统',
    tone: PrivacySectionTone.success,
    paragraphs: [
      'Amitia 会自动从对话中提取关键信息形成记忆，包括你的偏好、习惯、重要事项等。',
      '记忆数据使用向量数据库进行加密存储，仅用于提供更个性化的服务。',
      '你可以随时在记忆管理页面查看、编辑或删除任何记忆条目，也可以导出全部记忆数据。',
    ],
  ),
  PrivacySectionData(
    icon: Icons.devices_other,
    title: '五、插件渠道连接',
    tone: PrivacySectionTone.secondary,
    paragraphs: [
      '当你安装并启用渠道插件时，Amitia 只会按插件声明授予必要的会话与网络权限。',
      '渠道凭据由插件管理，宿主不会保存插件未声明的额外敏感凭证。',
      '你可以随时停用或卸载渠道插件，停用后该渠道不再接收或发送消息。',
    ],
  ),
  PrivacySectionData(
    icon: Icons.security,
    title: '六、使用边界与免责声明',
    tone: PrivacySectionTone.error,
    paragraphs: [
      'Amitia 是一个 AI 辅助工具，不提供医疗、法律、金融等专业建议。请勿将 Amitia 的输出作为专业决策的唯一依据。',
      '你应对使用 Amitia 产生的所有操作和后果承担最终责任。',
      'Amitia 不对因使用本软件而产生的任何直接或间接损失承担责任。',
      '请遵守当地法律法规，不得将 Amitia 用于任何违法或侵权的活动。',
    ],
  ),
  PrivacySectionData(
    icon: Icons.update,
    title: '七、条款更新',
    tone: PrivacySectionTone.muted,
    paragraphs: [
      '本隐私与使用边界条款可能随产品迭代而更新。重大变更时，Amitia 将在启动时通知你重新确认。',
      '你可以在设置 > 关于 中随时查看最新版本的条款。',
    ],
  ),
];

Color privacySectionColor(BuildContext context, PrivacySectionTone tone) {
  return switch (tone) {
    PrivacySectionTone.accent => context.accentPrimary,
    PrivacySectionTone.info => context.info,
    PrivacySectionTone.warning => context.warning,
    PrivacySectionTone.success => context.success,
    PrivacySectionTone.secondary => context.accentSecondary,
    PrivacySectionTone.error => context.error,
    PrivacySectionTone.muted => context.textSecondary,
  };
}

class PrivacySectionCard extends StatelessWidget {
  const PrivacySectionCard({super.key, required this.data});

  final PrivacySectionData data;

  @override
  Widget build(BuildContext context) {
    final color = privacySectionColor(context, data.tone);
    return Container(
      padding: EdgeInsets.all(AppSpacing.cardPadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(
                  color: color.withValues(alpha: 0.12),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(data.icon, size: 20, color: color),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Text(
                  data.title,
                  style: AppTypography.cardTitle(context),
                ),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          ...data.paragraphs.map(
            (paragraph) => Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.sm),
              child: Text(
                paragraph,
                style: AppTypography.bodySmall(context).copyWith(height: 1.6),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class PrivacySectionsList extends StatelessWidget {
  const PrivacySectionsList({super.key, this.padding, this.controller});

  final EdgeInsetsGeometry? padding;
  final ScrollController? controller;

  @override
  Widget build(BuildContext context) {
    return ListView(
      controller: controller,
      padding: padding ?? EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        for (var index = 0; index < privacySections.length; index++) ...[
          if (index > 0) SizedBox(height: AppSpacing.lg),
          PrivacySectionCard(data: privacySections[index]),
        ],
      ],
    );
  }
}
