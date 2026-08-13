import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class AiConfigPage extends ConsumerWidget {
  const AiConfigPage({super.key});

  static const _strategies = ['滑动窗口', '摘要压缩', '全量上下文', '向量检索'];

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final providersAsync = ref.watch(modelConfigListProvider);
    final charactersAsync = ref.watch(characterListProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: 'AI 配置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: providersAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                const SizedBox(height: 16),
                Text(
                  '加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                  style: AppTypography.body(context).copyWith(color: context.error),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 16),
                AmitiaButton(
                  label: '重试',
                  onPressed: () => ref.invalidate(modelConfigListProvider),
                ),
              ],
            ),
          ),
        ),
        data: (configs) {
          final activeConfig = configs.where((c) => c.isActive == 1).firstOrNull;
          final configName = activeConfig?.name ?? '';
          final currentStrategy = _strategies.contains(configName) ? configName : _strategies.first;
          final characters = charactersAsync.valueOrNull ?? [];

          return _AiConfigContent(
            currentModel: configName.isEmpty ? (configs.isNotEmpty ? configs.first.name : '') : configName,
            currentStrategy: currentStrategy,
            configs: configs,
            characters: characters,
          );
        },
      ),
    );
  }
}

class _AiConfigContent extends StatefulWidget {
  final String currentModel;
  final String currentStrategy;
  final List<dynamic> configs;
  final List<dynamic> characters;

  const _AiConfigContent({
    required this.currentModel,
    required this.currentStrategy,
    required this.configs,
    required this.characters,
  });

  @override
  State<_AiConfigContent> createState() => _AiConfigContentState();
}

class _AiConfigContentState extends State<_AiConfigContent> {
  static const _strategies = ['滑动窗口', '摘要压缩', '全量上下文', '向量检索'];
  static const _fallbackOptions = ['简单回复', '重试', '切换模型', '静默失败'];

  late String _defaultCharacter;
  late String _defaultModel;
  String _contextStrategy = _strategies.first;
  bool _streamingOutput = true;
  bool _messageSplitting = true;
  bool _toolCalls = true;
  String _errorFallback = _fallbackOptions.first;

  @override
  void initState() {
    super.initState();
    final characterNames = widget.characters.map((c) => c.name.toString()).toList();
    _defaultCharacter = characterNames.isNotEmpty ? characterNames.first : '';
    final modelNames = widget.configs.map((c) => c.name.toString()).toList();
    _defaultModel = modelNames.contains(widget.currentModel) ? widget.currentModel : (modelNames.isNotEmpty ? modelNames.first : '');
    _contextStrategy = _strategies.contains(widget.currentStrategy) ? widget.currentStrategy : _strategies.first;
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
      children: [
        _SectionLabel(text: '全局 AI 行为'),
        const SizedBox(height: AppSpacing.sm),
        _buildCard([
          _buildDropdownTile(
            icon: Icons.person_outline,
            title: '默认角色',
            value: _defaultCharacter,
            options: widget.characters.map((c) => c.name.toString()).toList(),
            onChanged: (v) => setState(() => _defaultCharacter = v),
          ),
          _divider(),
          _buildDropdownTile(
            icon: Icons.psychology_outlined,
            title: '默认模型',
            value: _defaultModel,
            options: widget.configs.map((c) => c.name.toString()).toList(),
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
            options: _fallbackOptions,
            onChanged: (v) => setState(() => _errorFallback = v),
          ),
        ]),
        const SizedBox(height: AppSpacing.xl),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: AmitiaButton(
            label: '管理模型配置',
            icon: Icons.settings,
            isFullWidth: true,
            onPressed: null,
          ),
        ),
        const SizedBox(height: AppSpacing.xl),
      ],
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
