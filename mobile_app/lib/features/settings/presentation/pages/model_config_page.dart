import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/model_config.dart';

class ModelConfigPage extends ConsumerStatefulWidget {
  final String modelType;

  const ModelConfigPage({super.key, required this.modelType});

  @override
  ConsumerState<ModelConfigPage> createState() => _ModelConfigPageState();
}

class _ModelConfigPageState extends ConsumerState<ModelConfigPage> {
  final Map<String, int> _testStates = {};

  static const _typeLabels = {
    'text': '文本模型',
    'vision': '视觉模型',
    'voice': '语音模型',
    'vector': '向量模型',
    'image': '图像生成',
  };


  @override
  void initState() {
    super.initState();
  }

  String get _typeName => _typeLabels[widget.modelType] ?? '模型配置';

  List<ModelConfigDto> _filterConfigs(List<ModelConfigDto> configs) {
    final typeLabel = _typeLabels[widget.modelType] ?? '文本模型';
    return configs.where((c) => c.name.contains(typeLabel) || widget.modelType == 'text').toList();
  }

  @override
  Widget build(BuildContext context) {
    final configsAsync = ref.watch(modelConfigListProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: _typeName, showBackButton: true),
      body: configsAsync.when(
        data: (allConfigs) {
          final configs = _filterConfigs(allConfigs);
          return ListView(
            padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
            children: [
              Padding(
                padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                child: Row(
                  children: [
                    Icon(Icons.psychology_outlined, size: 20, color: context.accentPrimary),
                    const SizedBox(width: 8),
                    Text('已配置 ${configs.length} 个$_typeName', style: AppTypography.caption(context)),
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
              SizedBox(height: AppSpacing.md),
              ...configs.map((c) => _buildConfigCard(c)),
              if (configs.isEmpty)
                AmitiaEmptyState(
                  icon: Icons.inbox_outlined,
                  title: '暂无配置',
                  subtitle: '点击右上角新建配置',
                ),
              SizedBox(height: AppSpacing.xl),
            ],
          );
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(Icons.error_outline, size: 48, color: context.error),
              SizedBox(height: AppSpacing.md),
              Text('加载失败', style: AppTypography.body(context)),
              Text(err.toString(), style: AppTypography.caption(context)),
              SizedBox(height: AppSpacing.md),
              AmitiaButton(
                label: '重试',
                isSecondary: true,
                onPressed: () => ref.invalidate(modelConfigListProvider),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildConfigCard(ModelConfigDto config) {
    final testState = _testStates[config.id] ?? 0;
    return Container(
      margin: EdgeInsets.only(left: AppSpacing.pagePadding, right: AppSpacing.pagePadding, bottom: AppSpacing.md),
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
              Expanded(
                child: Text(config.name, style: AppTypography.cardTitle(context)),
              ),
              if (config.isActive == 1)
                AmitiaStatusBadge(label: '已激活', type: BadgeType.success)
              else
                AmitiaStatusBadge(label: '未激活', type: BadgeType.neutral),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          _InfoRow(label: '提供商', value: config.provider),
          _InfoRow(label: '模型名', value: config.model),
          SizedBox(height: AppSpacing.md),
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
              SizedBox(width: AppSpacing.sm),
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
            SizedBox(height: AppSpacing.sm),
            Row(
              children: [
                Icon(Icons.check_circle, size: 14, color: context.success),
                const SizedBox(width: 4),
                Text('连接成功', style: AppTypography.label(context).copyWith(color: context.success)),
              ],
            ),
          ],
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
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
    try {
      final svc = ref.read(modelConfigServiceProvider);
      await svc.test(id);
      if (mounted) setState(() => _testStates[id] = 2);
    } catch (e) {
      if (mounted) setState(() => _testStates[id] = 0);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('测试失败: $e'), duration: const Duration(seconds: 2)),
      );
    }
  }

  void _showConfigSheet(ModelConfigDto? existing) {
    final nameCtrl = TextEditingController(text: existing?.name ?? '');
    final providerCtrl = TextEditingController(text: existing?.provider ?? '');
    final modelCtrl = TextEditingController(text: existing?.model ?? '');
    final baseUrlCtrl = TextEditingController(text: existing?.baseUrl ?? '');
    bool isActive = existing?.isActive == 1;

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
                    SizedBox(height: AppSpacing.lg),
                    _SheetField(label: '配置名称', controller: nameCtrl, hint: '如：GPT-4 主力'),
                    SizedBox(height: AppSpacing.md),
                    _SheetField(label: '提供商', controller: providerCtrl, hint: '如：OpenAI'),
                    SizedBox(height: AppSpacing.md),
                    _SheetField(label: '模型名', controller: modelCtrl, hint: '如：gpt-4'),
                    SizedBox(height: AppSpacing.md),
                    _SheetField(label: 'API 地址', controller: baseUrlCtrl, hint: 'https://api.openai.com/v1'),
                    SizedBox(height: AppSpacing.md),
                    AmitiaSwitchTile(
                      title: '激活此配置',
                      value: isActive,
                      onChanged: (v) => setSheetState(() => isActive = v),
                    ),
                    SizedBox(height: AppSpacing.lg),
                    AmitiaButton(
                      label: '保存',
                      isFullWidth: true,
                      onPressed: () async {
                        if (nameCtrl.text.isEmpty) return;
                        try {
                          final svc = ref.read(modelConfigServiceProvider);
                          final data = {
                            'name': nameCtrl.text,
                            'provider': providerCtrl.text,
                            'model': modelCtrl.text,
                            'baseUrl': baseUrlCtrl.text,
                            'isActive': isActive ? 1 : 0,
                          };
                          if (existing != null) {
                            await svc.update(existing.id, data);
                          } else {
                            await svc.create(data);
                          }
                          if (mounted) {
                            ref.invalidate(modelConfigListProvider);
                            Navigator.pop(ctx);
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text(existing == null ? '已新建配置' : '已更新配置'), duration: const Duration(seconds: 1)),
                            );
                          }
                        } catch (e) {
                          if (mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text('保存失败: $e'), duration: const Duration(seconds: 2)),
                            );
                          }
                        }
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

  void _confirmDelete(ModelConfigDto config) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('删除配置', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「${config.name}」吗？', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              try {
                final svc = ref.read(modelConfigServiceProvider);
                await svc.delete(config.id);
                ref.invalidate(modelConfigListProvider);
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('已删除'), duration: Duration(seconds: 1)),
                  );
                }
              } catch (e) {
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('删除失败: $e'), duration: const Duration(seconds: 2)),
                  );
                }
              }
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
          SizedBox(width: AppSpacing.md),
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
