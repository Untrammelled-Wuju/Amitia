import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class AiConfigPage extends ConsumerStatefulWidget {
  const AiConfigPage({super.key});

  @override
  ConsumerState<AiConfigPage> createState() => _AiConfigPageState();
}

class _AiConfigPageState extends ConsumerState<AiConfigPage> {
  late String _defaultCharacter;
  late String _defaultModel;
  late String _contextStrategy;
  late bool _streamingOutput;
  late bool _messageSplitting;
  late bool _toolCalls;
  late String _errorFallback;

  static const _characters = ['Amitia', '小雨', 'Epsilon', 'Karin'];
  static const _models = ['GPT-4', 'Claude 3', 'DeepSeek', '本地模型'];
  static const _strategies = ['滑动窗口', '摘要压缩', '全量上下文', '向量检索'];
  static const _fallbacks = ['简单回复', '重试', '切换模型', '静默失败'];

  @override
  void initState() {
    super.initState();
    final config = MockSettings.aiConfig;
    _defaultCharacter = config.defaultCharacter;
    _defaultModel = config.defaultModel;
    _contextStrategy = config.contextStrategy;
    _streamingOutput = config.streamingOutput;
    _messageSplitting = config.messageSplitting;
    _toolCalls = config.toolCalls;
    _errorFallback = config.errorFallback;
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: 'AI 配置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          _SectionLabel(text: '全局 AI 行为'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildDropdownTile(
              icon: Icons.person_outline,
              title: '默认角色',
              value: _defaultCharacter,
              options: _characters,
              onChanged: (v) => setState(() => _defaultCharacter = v),
            ),
            _divider(),
            _buildDropdownTile(
              icon: Icons.psychology_outlined,
              title: '默认模型',
              value: _defaultModel,
              options: _models,
              onChanged: (v) => setState(() => _defaultModel = v),
            ),
            _divider(),
            _buildDropdownTile(
              icon: Icons.memory,
              title: '上下文策略',
              value: _contextStrategy,
              options: _strategies,
              onChanged: (v) => setState(() => _contextStrategy = v),
            ),
          ]),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '输出与调用'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            AmitiaSwitchTile(
              title: '流式输出',
              subtitle: '逐字显示回复内容',
              value: _streamingOutput,
              onChanged: (v) => setState(() => _streamingOutput = v),
            ),
            _divider(),
            AmitiaSwitchTile(
              title: '消息拆分',
              subtitle: '长文本自动分段发送',
              value: _messageSplitting,
              onChanged: (v) => setState(() => _messageSplitting = v),
            ),
            _divider(),
            AmitiaSwitchTile(
              title: '工具调用',
              subtitle: '允许 AI 调用扩展工具',
              value: _toolCalls,
              onChanged: (v) => setState(() => _toolCalls = v),
            ),
          ]),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '异常处理'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildDropdownTile(
              icon: Icons.error_outline,
              title: '错误回落',
              value: _errorFallback,
              options: _fallbacks,
              onChanged: (v) => setState(() => _errorFallback = v),
            ),
          ]),
          const SizedBox(height: AppSpacing.xl),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaButton(
              label: '保存配置',
              icon: Icons.check,
              isFullWidth: true,
              onPressed: () {
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('AI 配置已保存'), duration: Duration(seconds: 1)),
                );
              },
            ),
          ),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _buildCard(List<Widget> children) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(children: children),
    );
  }

  Widget _divider() {
    return Padding(
      padding: const EdgeInsets.only(left: 56),
      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
    );
  }

  Widget _buildDropdownTile({
    required IconData icon,
    required String title,
    required String value,
    required List<String> options,
    required ValueChanged<String> onChanged,
  }) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => _showOptionSheet(title, options, value, onChanged),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
        child: Row(
          children: [
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(color: context.accentSoft, shape: BoxShape.circle),
              child: Icon(icon, size: 17, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(child: Text(title, style: AppTypography.body(context))),
            Text(value, style: AppTypography.caption(context)),
            const SizedBox(width: 4),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showOptionSheet(String title, List<String> options, String current, ValueChanged<String> onChanged) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (ctx) {
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Padding(
                padding: const EdgeInsets.all(AppSpacing.lg),
                child: Text(title, style: AppTypography.sectionTitle(context)),
              ),
              ...options.map((opt) {
                final isSelected = opt == current;
                return ListTile(
                  leading: Icon(
                    isSelected ? Icons.radio_button_checked : Icons.radio_button_off,
                    size: 20,
                    color: isSelected ? context.accentPrimary : context.textTertiary,
                  ),
                  title: Text(opt, style: AppTypography.body(context)),
                  onTap: () {
                    onChanged(opt);
                    Navigator.pop(ctx);
                  },
                );
              }),
              const SizedBox(height: AppSpacing.sm),
            ],
          ),
        );
      },
    );
  }
}

class _SectionLabel extends StatelessWidget {
  final String text;
  const _SectionLabel({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}
