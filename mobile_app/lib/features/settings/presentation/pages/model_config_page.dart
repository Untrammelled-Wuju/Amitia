import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class ModelConfigPage extends ConsumerStatefulWidget {
  final String modelType;

  const ModelConfigPage({super.key, required this.modelType});

  @override
  ConsumerState<ModelConfigPage> createState() => _ModelConfigPageState();
}

class _ModelConfigPageState extends ConsumerState<ModelConfigPage> {
  late List<ModelConfig> _configs;
  final Map<String, int> _testStates = {};

  static const _typeLabels = {
    'llm': '文本模型',
    'voice': '语音模型',
    'embedding': '向量模型',
    'vision': '视觉模型',
    'imagegen': '图像生成',
  };

  static const _typeIcons = {
    'llm': Icons.chat_outlined,
    'voice': Icons.record_voice_over_outlined,
    'embedding': Icons.scatter_plot_outlined,
    'vision': Icons.visibility_outlined,
    'imagegen': Icons.image_outlined,
  };

  @override
  void initState() {
    super.initState();
    _configs = MockSettings.modelConfigs.where((c) => c.type.name == widget.modelType).toList();
  }

  String get _typeName => _typeLabels[widget.modelType] ?? '模型配置';

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: _typeName, showBackButton: true),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Row(
              children: [
                Icon(_typeIcons[widget.modelType] ?? Icons.psychology_outlined, size: 20, color: context.accentPrimary),
                const SizedBox(width: 8),
                Text('已配置 ${_configs.length} 个$_typeName', style: AppTypography.caption(context)),
                const Spacer(),
                GestureDetector(
                  onTap: () => _showConfigSheet(null),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                    decoration: BoxDecoration(color: context.accentPrimary, borderRadius: AppRadius.brTag),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(Icons.add, size: 16, color: Colors.white),
                        const SizedBox(width: 4),
                        Text('新建', style: TextStyle(fontSize: 13, color: Colors.white, fontWeight: FontWeight.w500)),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          ..._configs.map((c) => _buildConfigCard(c)),
          if (_configs.isEmpty)
            AmitiaEmptyState(
              icon: Icons.inbox_outlined,
              title: '暂无配置',
              subtitle: '点击右上角新建配置',
            ),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _buildConfigCard(ModelConfig config) {
    final testState = _testStates[config.id] ?? 0;
    return Container(
      margin: const EdgeInsets.only(left: AppSpacing.pagePadding, right: AppSpacing.pagePadding, bottom: AppSpacing.md),
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
              Expanded(
                child: Text(config.name, style: AppTypography.cardTitle(context)),
              ),
              if (config.isActive)
                AmitiaStatusBadge(label: '已激活', type: BadgeType.success)
              else
                AmitiaStatusBadge(label: '未激活', type: BadgeType.neutral),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          _InfoRow(label: '提供商', value: config.provider),
          _InfoRow(label: '模型名', value: config.modelName),
          if (config.assignedScene != null) _InfoRow(label: '场景', value: config.assignedScene!),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Expanded(
                child: AmitiaButton(
                  label: testState == 1 ? '测试中...' : '测试连接',
                  isSecondary: true,
                  icon: Icons.bolt,
                  onPressed: testState == 1 ? null : () => _testConnection(config.id),
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: AmitiaButton(
                  label: '编辑',
                  isSecondary: true,
                  icon: Icons.edit_outlined,
                  onPressed: () => _showConfigSheet(config),
                ),
              ),
            ],
          ),
          if (testState == 2) ...[
            const SizedBox(height: AppSpacing.sm),
            Row(
              children: [
                Icon(Icons.check_circle, size: 14, color: context.success),
                const SizedBox(width: 4),
                Text('连接成功 · 延迟 45ms', style: AppTypography.label(context).copyWith(color: context.success)),
              ],
            ),
          ],
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Expanded(
                child: GestureDetector(
                  onTap: () => _showSceneSheet(config),
                  child: Container(
                    height: 36,
                    decoration: BoxDecoration(
                      border: Border.all(color: context.borderPrimary, width: 0.5),
                      borderRadius: AppRadius.brSmall,
                    ),
                    child: Center(child: Text('场景分配', style: AppTypography.caption(context))),
                  ),
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: GestureDetector(
                  onTap: () => _confirmDelete(config),
                  child: Container(
                    height: 36,
                    decoration: BoxDecoration(
                      border: Border.all(color: context.error.withValues(alpha: 0.3), width: 0.5),
                      borderRadius: AppRadius.brSmall,
                    ),
                    child: Center(child: Text('删除', style: AppTypography.caption(context).copyWith(color: context.error))),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _testConnection(String id) async {
    setState(() => _testStates[id] = 1);
    await Future.delayed(const Duration(milliseconds: 1500));
    if (mounted) setState(() => _testStates[id] = 2);
  }

  void _showConfigSheet(ModelConfig? existing) {
    final nameCtrl = TextEditingController(text: existing?.name ?? '');
    final providerCtrl = TextEditingController(text: existing?.provider ?? '');
    final modelCtrl = TextEditingController(text: existing?.modelName ?? '');
    final baseUrlCtrl = TextEditingController(text: existing?.baseUrl ?? '');
    final apiKeyCtrl = TextEditingController(text: existing?.apiKey ?? '');
    bool isActive = existing?.isActive ?? false;

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheetState) {
          return SafeArea(
            child: Padding(
              padding: EdgeInsets.fromLTRB(AppSpacing.lg, AppSpacing.lg, AppSpacing.lg, MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.lg),
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(existing == null ? '新建配置' : '编辑配置', style: AppTypography.sectionTitle(context)),
                    const SizedBox(height: AppSpacing.lg),
                    _SheetField(label: '配置名称', controller: nameCtrl, hint: '如：GPT-4 主力'),
                    const SizedBox(height: AppSpacing.md),
                    _SheetField(label: '提供商', controller: providerCtrl, hint: '如：OpenAI'),
                    const SizedBox(height: AppSpacing.md),
                    _SheetField(label: '模型名', controller: modelCtrl, hint: '如：gpt-4'),
                    const SizedBox(height: AppSpacing.md),
                    _SheetField(label: 'API 地址', controller: baseUrlCtrl, hint: 'https://api.openai.com/v1'),
                    const SizedBox(height: AppSpacing.md),
                    _SheetField(label: 'API Key', controller: apiKeyCtrl, hint: 'sk-...', obscure: true),
                    const SizedBox(height: AppSpacing.md),
                    AmitiaSwitchTile(
                      title: '激活此配置',
                      value: isActive,
                      onChanged: (v) => setSheetState(() => isActive = v),
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    AmitiaButton(
                      label: '保存',
                      isFullWidth: true,
                      onPressed: () {
                        if (nameCtrl.text.isEmpty) return;
                        setState(() {
                          if (existing != null) {
                            final idx = _configs.indexWhere((c) => c.id == existing.id);
                            _configs[idx] = ModelConfig(
                              id: existing.id,
                              name: nameCtrl.text,
                              type: existing.type,
                              provider: providerCtrl.text,
                              baseUrl: baseUrlCtrl.text,
                              modelName: modelCtrl.text,
                              apiKey: apiKeyCtrl.text,
                              isActive: isActive,
                              assignedScene: existing.assignedScene,
                            );
                          } else {
                            _configs.add(ModelConfig(
                              id: 'mc${DateTime.now().millisecondsSinceEpoch}',
                              name: nameCtrl.text,
                              type: existing?.type ?? ModelType.values.firstWhere((t) => t.name == widget.modelType),
                              provider: providerCtrl.text,
                              baseUrl: baseUrlCtrl.text,
                              modelName: modelCtrl.text,
                              apiKey: apiKeyCtrl.text,
                              isActive: isActive,
                            ));
                          }
                        });
                        Navigator.pop(ctx);
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(existing == null ? '已新建配置' : '已更新配置'), duration: const Duration(seconds: 1)),
                        );
                      },
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  void _showSceneSheet(ModelConfig config) {
    const scenes = ['默认对话', '创意写作', '代码生成', '翻译', '摘要'];
    String? selected = config.assignedScene;

    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheetState) => SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Padding(padding: const EdgeInsets.all(AppSpacing.lg), child: Text('场景分配', style: AppTypography.sectionTitle(context))),
              ListTile(
                leading: Icon(selected == null ? Icons.radio_button_checked : Icons.radio_button_off,
                    size: 20, color: selected == null ? context.accentPrimary : context.textTertiary),
                title: Text('不分配', style: AppTypography.body(context)),
                onTap: () => setSheetState(() => selected = null),
              ),
              ...scenes.map((s) => ListTile(
                    leading: Icon(selected == s ? Icons.radio_button_checked : Icons.radio_button_off,
                        size: 20, color: selected == s ? context.accentPrimary : context.textTertiary),
                    title: Text(s, style: AppTypography.body(context)),
                    onTap: () => setSheetState(() => selected = s),
                  )),
              Padding(
                padding: const EdgeInsets.all(AppSpacing.lg),
                child: AmitiaButton(
                  label: '确定',
                  isFullWidth: true,
                  onPressed: () {
                    setState(() {
                      final idx = _configs.indexWhere((c) => c.id == config.id);
                      _configs[idx] = ModelConfig(
                        id: config.id,
                        name: config.name,
                        type: config.type,
                        provider: config.provider,
                        baseUrl: config.baseUrl,
                        modelName: config.modelName,
                        apiKey: config.apiKey,
                        isActive: config.isActive,
                        assignedScene: selected,
                      );
                    });
                    Navigator.pop(ctx);
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('场景已分配'), duration: Duration(seconds: 1)),
                    );
                  },
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _confirmDelete(ModelConfig config) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('删除配置', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「${config.name}」吗？', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() => _configs.removeWhere((c) => c.id == config.id));
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('已删除'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        children: [
          SizedBox(width: 60, child: Text(label, style: AppTypography.label(context))),
          const SizedBox(width: AppSpacing.md),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }
}

class _SheetField extends StatelessWidget {
  final String label;
  final TextEditingController controller;
  final String hint;
  final bool obscure;
  const _SheetField({required this.label, required this.controller, required this.hint, this.obscure = false});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: AppTypography.label(context)),
        const SizedBox(height: 4),
        AmitiaTextField(hintText: hint, controller: controller, obscureText: obscure),
      ],
    );
  }
}
