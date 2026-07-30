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
              padding: const EdgeInsets.all(AppSpacing.pagePadding),
              children: [
                _Section(
                  icon: Icons.shield_outlined,
                  title: '一、数据收集与存储',
                  color: context.accentPrimary,
                  paragraphs: [
                    'Amitia 采用本地优先架构，你的对话记录、记忆数据、角色设定等核心数据均存储在本地设备的加密数据库中。',
                    '在本地部署模式下，所有数据不会离开你的设备，我们无法访问你的任何数据。',
                    '在云端部署模式下，数据将通过端到端加密传输至你指定的服务器，传输过程使用 TLS 1.3 加密协议。',
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),
                _Section(
                  icon: Icons.psychology_outlined,
                  title: '二、AI 模型调用',
                  color: context.info,
                  paragraphs: [
                    'Amitia 在对话和任务处理过程中会调用你配置的 AI 模型（如 OpenAI GPT-4、Anthropic Claude 等）。',
                    '当你发送消息时，相关的对话上下文将被发送至模型服务商进行推理。请勿在对话中输入敏感的个人信息（如身份证号、银行卡号等）。',
                    '模型服务商的数据处理遵循其各自的隐私政策，Amitia 不控制也不对模型服务商的数据处理行为承担责任。',
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),
                _Section(
                  icon: Icons.build_outlined,
                  title: '三、Agent 模式与工具调用',
                  color: context.warning,
                  paragraphs: [
                    'Agent 模式下，Amitia 可以执行文件操作、系统命令、网络请求等高权限操作。',
                    '所有高风险操作在执行前都需要你的明确确认。你可以选择"此次允许"、"始终允许"或"拒绝"。',
                    '建议你定期在设置 > 系统权限中审查已授权的权限列表，及时撤销不再需要的权限。',
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),
                _Section(
                  icon: Icons.memory,
                  title: '四、记忆系统',
                  color: context.success,
                  paragraphs: [
                    'Amitia 会自动从对话中提取关键信息形成记忆，包括你的偏好、习惯、重要事项等。',
                    '记忆数据使用向量数据库进行加密存储，仅用于提供更个性化的服务。',
                    '你可以随时在记忆管理页面查看、编辑或删除任何记忆条目，也可以导出全部记忆数据。',
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),
                _Section(
                  icon: Icons.devices_other,
                  title: '五、渠道连接（微信/QQ）',
                  color: context.accentSecondary,
                  paragraphs: [
                    '当连接微信或 QQ 渠道时，Amitia 需要获取相应的会话权限以接收和发送消息。',
                    '渠道连接使用官方提供的接口或协议，Amitia 不会存储你的登录密码或敏感凭证。',
                    '你可以随时在渠道管理页面断开连接，断开后 Amitia 将不再接收或发送该渠道的消息。',
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),
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
                const SizedBox(height: AppSpacing.lg),
                _Section(
                  icon: Icons.update,
                  title: '七、条款更新',
                  color: context.textSecondary,
                  paragraphs: [
                    '本隐私与使用边界条款可能随产品迭代而更新。重大变更时，Amitia 将在启动时通知你重新确认。',
                    '你可以在设置 > 关于 中随时查看最新版本的条款。',
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),
                if (!_hasScrolledToBottom)
                  Center(
                    child: Padding(
                      padding: const EdgeInsets.all(AppSpacing.md),
                      child: Column(
                        children: [
                          Icon(Icons.arrow_downward, size: 20, color: context.textTertiary),
                          const SizedBox(height: AppSpacing.xs),
                          Text('向下滚动查看完整条款', style: AppTypography.label(context)),
                        ],
                      ),
                    ),
                  ),
              ],
            ),
          ),
          Container(
            padding: const EdgeInsets.all(AppSpacing.pagePadding),
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
                        const SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: Text(
                            '我已阅读并同意以上隐私条款与使用边界',
                            style: AppTypography.bodySmall(context),
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: AppSpacing.md),
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
      padding: const EdgeInsets.all(AppSpacing.cardPadding),
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
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Text(title, style: AppTypography.cardTitle(context)),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          ...paragraphs.map((p) => Padding(
            padding: const EdgeInsets.only(bottom: AppSpacing.sm),
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
