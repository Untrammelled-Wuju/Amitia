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

class PrivacyPage extends ConsumerStatefulWidget {
  const PrivacyPage({super.key});

  @override
  ConsumerState<PrivacyPage> createState() => _PrivacyPageState();
}

class _PrivacyPageState extends ConsumerState<PrivacyPage> {
  final _scrollController = ScrollController();
  bool _hasScrolledToBottom = false;
  bool _agreed = false;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(() {
      final maxScroll = _scrollController.position.maxScrollExtent;
      final currentScroll = _scrollController.position.pixels;
      if (maxScroll - currentScroll <= 50 && !_hasScrolledToBottom) {
        setState(() => _hasScrolledToBottom = true);
      }
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _agree() {
    if (!_agreed) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: const Text('请先勾选同意条款'),
          duration: const Duration(seconds: 1),
          backgroundColor: context.warning,
        ),
      );
      return;
    }
    context.go(AppRoutes.chat);
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '隐私与使用边界', showBackButton: true),
      body: Column(
        children: [
          Expanded(
            child: ListView(
              controller: _scrollController,
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              children: [
                _Section(
                  icon: Icons.shield_outlined,
                  title: '一、数据收集与存储',
                  color: context.accentPrimary,
                  paragraphs: [
                    'Amitia 支持本地模式和云端模式。实际数据存放位置取决于你选择的部署方式、Core 地址以及启用的扩展能力。',
                    '本地模式由当前设备上的 Runtime/Core 提供服务；云端模式会连接你配置的远程 Core，并可通过 Device Mesh 协同设备能力。',
                    '客户端不会在这里承诺源码未强制实现的“端到端加密”或固定 TLS 版本；网络安全边界以你的实际部署、反向代理和服务配置为准。',
                  ],
                ),
                SizedBox(height: AppSpacing.lg),
                _Section(
                  icon: Icons.psychology_outlined,
                  title: '二、AI 模型调用',
                  color: context.info,
                  paragraphs: [
                    'Amitia 会调用你在“模型设置”中实际配置并启用的文本、视觉、语音、向量或图像生成服务。',
                    '完成推理所需的消息、上下文、图片、音频或其他输入可能发送给对应模型服务商；具体范围取决于当前能力和模型配置。',
                    '第三方模型服务的数据处理规则由对应服务商决定，请根据你的数据敏感度选择本地服务或合适的远程服务。',
                  ],
                ),
                SizedBox(height: AppSpacing.lg),
                _Section(
                  icon: Icons.build_outlined,
                  title: '三、Agent 模式与工具调用',
                  color: context.warning,
                  paragraphs: [
                    'Agent、MCP、Skills、扩展包和设备能力可以触发工具调用；实际可执行范围由当前 Runtime、扩展注册和权限策略共同决定。',
                    '需要审批的能力会通过现有权限/审批链处理；并非所有工具都具有相同的风险等级或授权方式。',
                    '建议定期在“系统权限”和扩展管理中检查已授予的能力，并关闭不再需要的扩展或设备权限。',
                  ],
                ),
                SizedBox(height: AppSpacing.lg),
                _Section(
                  icon: Icons.memory,
                  title: '四、记忆系统',
                  color: context.success,
                  paragraphs: [
                    'Amitia 的记忆系统包含长期记忆、情景记忆、用户画像、世界书、时间线和向量检索等数据能力。',
                    '记忆由后端数据库和检索组件管理；是否位于本机或云端取决于当前部署，不在客户端 UI 中虚构额外的加密属性。',
                    '现有记忆管理页面提供查看、创建、编辑、删除和相关检索能力，具体以当前后端接口支持范围为准。',
                  ],
                ),
                SizedBox(height: AppSpacing.lg),
                _Section(
                  icon: Icons.devices_other,
                  title: '五、渠道连接（微信/QQ）',
                  color: context.accentSecondary,
                  paragraphs: [
                    '微信、QQ 等渠道只有在你完成对应渠道配置后才会参与消息收发。',
                    '渠道所需的 Token、密钥或连接参数会按当前后端配置方式处理；客户端不额外承诺源码没有实现的凭据存储策略。',
                    '你可以在渠道中心查看真实连接状态，并通过后端已有的连接/断开接口管理渠道。',
                  ],
                ),
                SizedBox(height: AppSpacing.lg),
                _Section(
                  icon: Icons.security,
                  title: '六、使用边界与免责声明',
                  color: context.error,
                  paragraphs: [
                    'Amitia 是一个 AI 辅助工具，不提供医疗、法律、金融等专业建议。请勿将 Amitia 的输出作为专业决策的唯一依据。',
                    '你应对使用 Amitia 产生的所有操作和后果承担最终责任。',
                    'Amitia 不对因使用本软件而产生的任何直接或间接损失承担责任。',
                    '请遵守当地法律法规，不得将 Amitia 用于任何违法或侵权的活动。',
                  ],
                ),
                SizedBox(height: AppSpacing.lg),
                _Section(
                  icon: Icons.update,
                  title: '七、条款更新',
                  color: context.textSecondary,
                  paragraphs: [
                    '本隐私与使用边界条款可能随产品迭代而更新。重大变更时，Amitia 将在启动时通知你重新确认。',
                    '你可以在设置 > 关于 中随时查看最新版本的条款。',
                  ],
                ),
                SizedBox(height: AppSpacing.lg),
                if (!_hasScrolledToBottom)
                  Center(
                    child: Padding(
                      padding: EdgeInsets.all(AppSpacing.md),
                      child: Column(
                        children: [
                          Icon(Icons.arrow_downward, size: 20, color: context.textTertiary),
                          SizedBox(height: AppSpacing.xs),
                          Text('向下滚动查看完整条款', style: AppTypography.label(context)),
                        ],
                      ),
                    ),
                  ),
              ],
            ),
          ),
          Container(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              border: Border(top: BorderSide(color: context.borderSecondary, width: 1)),
            ),
            child: SafeArea(
              top: false,
              child: Column(
                children: [
                  GestureDetector(
                    onTap: () => setState(() => _agreed = !_agreed),
                    child: Row(
                      children: [
                        Icon(
                          _agreed ? Icons.check_box : Icons.check_box_outline_blank,
                          size: 22,
                          color: _agreed ? context.accentPrimary : context.textTertiary,
                        ),
                        SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: Text(
                            '我已阅读并同意以上隐私条款与使用边界',
                            style: AppTypography.bodySmall(context),
                          ),
                        ),
                      ],
                    ),
                  ),
                  SizedBox(height: AppSpacing.md),
                  AmitiaButton(
                    label: '同意并继续',
                    icon: Icons.check_circle_outline,
                    isFullWidth: true,
                    onPressed: _agreed ? _agree : null,
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _Section extends StatelessWidget {
  final IconData icon;
  final String title;
  final Color color;
  final List<String> paragraphs;

  const _Section({
    required this.icon,
    required this.title,
    required this.color,
    required this.paragraphs,
  });

  @override
  Widget build(BuildContext context) {
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
                child: Icon(icon, size: 20, color: color),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Text(title, style: AppTypography.cardTitle(context)),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          ...paragraphs.map((p) => Padding(
            padding: EdgeInsets.only(bottom: AppSpacing.sm),
            child: Text(
              p,
              style: AppTypography.bodySmall(context).copyWith(height: 1.6),
            ),
          )),
        ],
      ),
    );
  }
}
