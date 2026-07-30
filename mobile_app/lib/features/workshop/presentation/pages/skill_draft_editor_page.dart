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

class SkillDraftEditorPage extends ConsumerStatefulWidget {
  final String draftId;

  const SkillDraftEditorPage({super.key, required this.draftId});

  @override
  ConsumerState<SkillDraftEditorPage> createState() => _SkillDraftEditorPageState();
}

class _SkillDraftEditorPageState extends ConsumerState<SkillDraftEditorPage> {
  late SkillDraft _draft;
  late TextEditingController _nameController;
  late TextEditingController _descController;
  late TextEditingController _metadataController;
  late TextEditingController _inputSchemaController;
  late TextEditingController _outputSchemaController;
  late TextEditingController _riskController;

  bool _isTesting = false;
  bool _hasTested = false;
  String _testResult = '';
  String _selectedRisk = '低风险';

  final _riskOptions = ['低风险', '中风险', '高风险'];

  @override
  void initState() {
    super.initState();
    SkillDraft? found;
    for (final d in MockWorkshop.skillDrafts) {
      if (d.id == widget.draftId) {
        found = d;
        break;
      }
    }
    _draft = found ?? SkillDraft(
      id: widget.draftId,
      name: '新技能',
      description: '',
      updated: DateTime.now(),
    );
    _nameController = TextEditingController(text: _draft.name);
    _descController = TextEditingController(text: _draft.description);
    _metadataController = TextEditingController(text: _draft.metadata);
    _inputSchemaController = TextEditingController(text: _draft.inputSchema);
    _outputSchemaController = TextEditingController(text: _draft.outputSchema);
    _riskController = TextEditingController(text: _draft.riskAssessment);
    _selectedRisk = _draft.riskAssessment;
    _hasTested = _draft.testResult != '未测试';
    _testResult = _draft.testResult;
  }

  @override
  void dispose() {
    _nameController.dispose();
    _descController.dispose();
    _metadataController.dispose();
    _inputSchemaController.dispose();
    _outputSchemaController.dispose();
    _riskController.dispose();
    super.dispose();
  }

  BadgeType _riskBadgeType(String risk) {
    switch (risk) {
      case '低风险':
        return BadgeType.success;
      case '中风险':
        return BadgeType.warning;
      case '高风险':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'Draft 编辑器',
        showBackButton: true,
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: AppSpacing.sm),
            child: AmitiaIconButton(
              icon: Icons.archive_outlined,
              color: context.warning,
              onPressed: _showArchiveConfirm,
            ),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            Expanded(
              child: ListView(
                padding: const EdgeInsets.all(AppSpacing.pagePadding),
                children: [
                  _buildStatusBanner(context),
                  const SizedBox(height: AppSpacing.sectionGap),
                  _buildMetadataSection(context),
                  const SizedBox(height: AppSpacing.sectionGap),
                  _buildSchemaSection(context, '输入 Schema', _inputSchemaController, '定义技能的输入参数'),
                  const SizedBox(height: AppSpacing.sectionGap),
                  _buildSchemaSection(context, '输出 Schema', _outputSchemaController, '定义技能的输出结果'),
                  const SizedBox(height: AppSpacing.sectionGap),
                  _buildRiskSection(context),
                  const SizedBox(height: AppSpacing.sectionGap),
                  _buildTestSection(context),
                  const SizedBox(height: AppSpacing.sectionGap),
                  _buildInstallPreviewSection(context),
                  const SizedBox(height: AppSpacing.xxl),
                ],
              ),
            ),
            _buildBottomActions(context),
          ],
        ),
      ),
    );
  }

  Widget _buildStatusBanner(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: context.accentSoft,
        borderRadius: AppRadius.brMedium,
      ),
      child: Row(
        children: [
          Icon(Icons.edit_note, size: 22, color: context.accentPrimary),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('编辑模式', style: AppTypography.body(context)),
                const SizedBox(height: 2),
                Text(
                  '修改完成后请提交保存，所有变更将同步到草稿',
                  style: AppTypography.label(context),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMetadataSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('元数据', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('技能名称', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: '输入技能名称',
                controller: _nameController,
              ),
              const SizedBox(height: AppSpacing.md),
              Text('描述', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: '输入技能描述',
                controller: _descController,
                maxLines: 3,
              ),
              const SizedBox(height: AppSpacing.md),
              Text('版本信息', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: '例如：版本 1.0.0',
                controller: _metadataController,
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildSchemaSection(
    BuildContext context,
    String title,
    TextEditingController controller,
    String hint,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(title, style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(hint, style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              Container(
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: AppRadius.brSmall,
                ),
                padding: const EdgeInsets.all(AppSpacing.md),
                child: AmitiaTextField(
                  hintText: '{"type": "object", ...}',
                  controller: controller,
                  maxLines: 6,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildRiskSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('能力风险评估', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Text('风险等级', style: AppTypography.body(context)),
                  const Spacer(),
                  AmitiaStatusBadge(label: _selectedRisk, type: _riskBadgeType(_selectedRisk)),
                ],
              ),
              const SizedBox(height: AppSpacing.sm),
              Row(
                children: _riskOptions.map((risk) {
                  final isSelected = risk == _selectedRisk;
                  return Expanded(
                    child: Padding(
                      padding: const EdgeInsets.only(right: AppSpacing.xs),
                      child: GestureDetector(
                        onTap: () {
                          setState(() {
                            _selectedRisk = risk;
                          });
                        },
                        child: Container(
                          padding: const EdgeInsets.symmetric(vertical: 10),
                          decoration: BoxDecoration(
                            color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                            borderRadius: AppRadius.brSmall,
                          ),
                          child: Center(
                            child: Text(
                              risk,
                              style: TextStyle(
                                fontSize: 13,
                                fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                                color: isSelected ? Colors.white : context.textSecondary,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  );
                }).toList(),
              ),
              const SizedBox(height: AppSpacing.md),
              Text('评估说明', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: '描述风险点和缓解措施',
                controller: _riskController,
                maxLines: 3,
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildTestSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('测试结果', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(Icons.science_outlined, size: 20, color: context.accentPrimary),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Text(
                      _isTesting
                          ? '正在测试...'
                          : _hasTested
                              ? '测试结果：$_testResult'
                              : '尚未测试',
                      style: AppTypography.body(context),
                    ),
                  ),
                  if (_hasTested && !_isTesting)
                    AmitiaStatusBadge(
                      label: _testResult == '通过' ? '通过' : '失败',
                      type: _testResult == '通过' ? BadgeType.success : BadgeType.error,
                    ),
                ],
              ),
              if (_isTesting) ...[
                const SizedBox(height: AppSpacing.sm),
                AmitiaProgressBar(progress: 0.7),
              ],
              const SizedBox(height: AppSpacing.md),
              AmitiaButton(
                label: _isTesting ? '测试中...' : '运行测试',
                icon: Icons.play_arrow,
                isFullWidth: true,
                isSecondary: _isTesting,
                onPressed: _isTesting ? null : _runTest,
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildInstallPreviewSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('安装预览', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildPreviewRow(context, '技能名称', _nameController.text.isEmpty ? '未命名' : _nameController.text),
              const SizedBox(height: AppSpacing.sm),
              _buildPreviewRow(context, '版本', _metadataController.text.isEmpty ? '未设置' : _metadataController.text),
              const SizedBox(height: AppSpacing.sm),
              _buildPreviewRow(context, '风险等级', _selectedRisk),
              const SizedBox(height: AppSpacing.sm),
              _buildPreviewRow(context, '测试状态', _hasTested ? _testResult : '未测试'),
              const SizedBox(height: AppSpacing.sm),
              _buildPreviewRow(context, '输入 Schema', _inputSchemaController.text.isEmpty ? '未定义' : '已定义'),
              const SizedBox(height: AppSpacing.sm),
              _buildPreviewRow(context, '输出 Schema', _outputSchemaController.text.isEmpty ? '未定义' : '已定义'),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildPreviewRow(BuildContext context, String label, String value) {
    return Row(
      children: [
        Text(label, style: AppTypography.caption(context)),
        const Spacer(),
        Text(value, style: AppTypography.bodySmall(context)),
      ],
    );
  }

  Widget _buildBottomActions(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(
          top: BorderSide(color: context.borderPrimary, width: 0.5),
        ),
      ),
      child: Row(
        children: [
          Expanded(
            child: AmitiaButton(
              label: '安装',
              isSecondary: true,
              onPressed: _showInstallConfirm,
            ),
          ),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: AmitiaButton(
              label: '提交',
              icon: Icons.check,
              onPressed: _showSubmitConfirm,
            ),
          ),
        ],
      ),
    );
  }

  void _runTest() {
    setState(() {
      _isTesting = true;
    });
    Future.delayed(const Duration(seconds: 2), () {
      if (mounted) {
        setState(() {
          _isTesting = false;
          _hasTested = true;
          _testResult = '通过';
        });
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('测试通过')),
        );
      }
    });
  }

  void _showSubmitConfirm() {
    if (_nameController.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('请填写技能名称')),
      );
      return;
    }
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('提交草稿', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认提交「${_nameController.text}」？提交后草稿将保存并可以安装使用。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('草稿已提交')),
                );
                Navigator.pop(context);
              },
              child: Text('提交', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showInstallConfirm() {
    if (!_hasTested) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('请先运行测试')),
      );
      return;
    }
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('安装技能', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认安装「${_nameController.text}」到系统？安装后可在扩展中心管理。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('「${_nameController.text}」安装成功')),
                );
                Navigator.pop(context);
              },
              child: Text('安装', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showArchiveConfirm() {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('归档草稿', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认归档「${_nameController.text}」？归档后草稿将不再可编辑。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('草稿已归档')),
                );
                Navigator.pop(context);
              },
              child: Text('归档', style: TextStyle(color: context.warning)),
            ),
          ],
        );
      },
    );
  }
}
