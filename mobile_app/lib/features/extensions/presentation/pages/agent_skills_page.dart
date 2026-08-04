import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_extensions.dart';

class AgentSkillsPage extends ConsumerStatefulWidget {
  const AgentSkillsPage({super.key});

  @override
  ConsumerState<AgentSkillsPage> createState() => _AgentSkillsPageState();
}

class _AgentSkillsPageState extends ConsumerState<AgentSkillsPage> {
  late List<AgentSkill> _skills;

  @override
  void initState() {
    super.initState();
    _skills = List.from(MockExtensions.agentSkills);
  }

  BadgeType _compatibilityBadgeType(String compatibility) {
    switch (compatibility) {
      case '完全兼容':
        return BadgeType.success;
      case '兼容':
        return BadgeType.info;
      case '部分兼容':
        return BadgeType.warning;
      default:
        return BadgeType.neutral;
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'Agent Skills',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: _skills.isEmpty
            ? AmitiaEmptyState(
                icon: Icons.auto_awesome_outlined,
                title: '暂无 Agent Skill',
                subtitle: '点击右下角导入 Agent Skill',
                actionText: '导入',
                onAction: _showImportSheet,
              )
            : ListView.separated(
                padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
                itemCount: _skills.length,
                separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
                itemBuilder: (context, index) => _buildSkillCard(context, _skills[index]),
              ),
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: _showImportSheet,
        backgroundColor: context.accentPrimary,
        child: const Icon(Icons.file_download_outlined, color: Colors.white),
      ),
    );
  }

  Widget _buildSkillCard(BuildContext context, AgentSkill skill) {
    return AmitiaCard(
      onTap: () => _showDetailSheet(skill),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(Icons.auto_awesome, size: 22, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Text(skill.name, style: AppTypography.cardTitle(context)),
                        const SizedBox(width: 8),
                        Text('v${skill.version}', style: AppTypography.label(context)),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text(skill.description, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                  ],
                ),
              ),
              AmitiaStatusBadge(
                label: skill.isEnabled ? '已启用' : '已停用',
                type: skill.isEnabled ? BadgeType.success : BadgeType.neutral,
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Container(
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brSmall,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(Icons.description_outlined, size: 14, color: context.textTertiary),
                    const SizedBox(width: 4),
                    Text('SKILL.md', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600)),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  skill.skillMd,
                  style: AppTypography.label(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: [
              if (skill.requiredMcp.isNotEmpty)
                ...skill.requiredMcp.map((mcp) => Container(
                      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                      decoration: BoxDecoration(
                        color: context.info.withValues(alpha: 0.1),
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(Icons.link, size: 12, color: context.info),
                          const SizedBox(width: 4),
                          Text(mcp, style: TextStyle(fontSize: 11, color: context.info, fontWeight: FontWeight.w500)),
                        ],
                      ),
                    )),
              Builder(builder: (context) {
                final badgeType = _compatibilityBadgeType(skill.compatibility);
                final color = badgeType == BadgeType.success
                    ? context.success
                    : badgeType == BadgeType.warning
                        ? context.warning
                        : context.info;
                return Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                  decoration: BoxDecoration(
                    color: color.withValues(alpha: 0.1),
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.check_circle_outline, size: 12, color: color),
                      const SizedBox(width: 4),
                      Text(skill.compatibility, style: TextStyle(fontSize: 11, color: color, fontWeight: FontWeight.w500)),
                    ],
                  ),
                );
              }),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              GestureDetector(
                onTap: () => _toggleSkill(skill),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
                  decoration: BoxDecoration(
                    color: skill.isEnabled ? context.warning.withValues(alpha: 0.1) : context.success.withValues(alpha: 0.1),
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(skill.isEnabled ? Icons.pause_circle_outline : Icons.play_circle_outline, size: 15, color: skill.isEnabled ? context.warning : context.success),
                      const SizedBox(width: 5),
                      Text(skill.isEnabled ? '停用' : '启用', style: TextStyle(fontSize: 13, color: skill.isEnabled ? context.warning : context.success, fontWeight: FontWeight.w500)),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 8),
              GestureDetector(
                onTap: () => _showDetailSheet(skill),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.info_outline, size: 15, color: context.accentPrimary),
                      const SizedBox(width: 5),
                      Text('详情', style: TextStyle(fontSize: 13, color: context.accentPrimary, fontWeight: FontWeight.w500)),
                    ],
                  ),
                ),
              ),
              const Spacer(),
              GestureDetector(
                onTap: () => _showRemoveConfirm(skill),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
                  decoration: BoxDecoration(
                    color: context.error.withValues(alpha: 0.1),
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.delete_outline, size: 15, color: context.error),
                      const SizedBox(width: 5),
                      Text('移除', style: TextStyle(fontSize: 13, color: context.error, fontWeight: FontWeight.w500)),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  void _showImportSheet() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (context) => _ImportSkillSheet(onConfirm: () {
        Navigator.pop(context);
        setState(() {
          _skills.add(AgentSkill(
            id: 'as${_skills.length + 1}',
            name: '新导入 Skill',
            description: '通过导入文件添加的 Agent Skill',
            skillMd: '# 新 Skill\n\n描述待补充',
            requiredMcp: ['文件系统 MCP'],
            compatibility: '兼容',
            isEnabled: false,
            version: '1.0.0',
          ));
        });
        ScaffoldMessenger.of(this.context).showSnackBar(
          SnackBar(content: const Text('Agent Skill 导入成功'), backgroundColor: context.success),
        );
      }),
    );
  }

  void _showDetailSheet(AgentSkill skill) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (context) => _SkillDetailSheet(skill: skill),
    );
  }

  void _toggleSkill(AgentSkill skill) {
    setState(() {
      final index = _skills.indexWhere((s) => s.id == skill.id);
      _skills[index] = AgentSkill(
        id: skill.id,
        name: skill.name,
        description: skill.description,
        skillMd: skill.skillMd,
        requiredMcp: skill.requiredMcp,
        compatibility: skill.compatibility,
        isEnabled: !skill.isEnabled,
        version: skill.version,
      );
    });
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('${skill.name} 已${skill.isEnabled ? '停用' : '启用'}'),
        backgroundColor: context.accentPrimary,
      ),
    );
  }

  void _showRemoveConfirm(AgentSkill skill) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('移除 Agent Skill', style: AppTypography.cardTitle(context)),
        content: Text('确定要移除「${skill.name}」吗？此操作不可撤销。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              setState(() {
                _skills.removeWhere((s) => s.id == skill.id);
              });
              ScaffoldMessenger.of(this.context).showSnackBar(
                SnackBar(content: Text('${skill.name} 已移除'), backgroundColor: context.error),
              );
            },
            child: Text('移除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }
}

class _ImportSkillSheet extends StatelessWidget {
  final VoidCallback onConfirm;

  const _ImportSkillSheet({required this.onConfirm});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 34),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Center(
            child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
          ),
          const SizedBox(height: 20),
          Text('导入 Agent Skill', style: AppTypography.pageTitle(context)),
          const SizedBox(height: 16),
          GestureDetector(
            onTap: () => amitiaComingSoon(context, '导入Agent Skill'),
            child: Container(
              padding: const EdgeInsets.all(24),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brMedium,
                border: Border.all(color: context.borderPrimary, width: 1, strokeAlign: BorderSide.strokeAlignOutside),
              ),
              child: Column(
                children: [
                  Icon(Icons.cloud_upload_outlined, size: 40, color: context.accentPrimary),
                  const SizedBox(height: 8),
                  Text('点击选择文件', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                  const SizedBox(height: 4),
                  Text('支持 .skill.md, .zip, .tar.gz 格式', style: AppTypography.label(context)),
                ],
              ),
            ),
          ),
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Row(
              children: [
                Icon(Icons.info_outline, size: 16, color: context.accentPrimary),
                const SizedBox(width: 8),
                Expanded(child: Text('导入后将自动解析 SKILL.md 并验证兼容性', style: AppTypography.label(context).copyWith(color: context.accentPrimary))),
              ],
            ),
          ),
          const SizedBox(height: 20),
          AmitiaButton(label: '确认导入', isFullWidth: true, icon: Icons.file_download_done, onPressed: onConfirm),
        ],
      ),
    );
  }
}

class _SkillDetailSheet extends StatelessWidget {
  final AgentSkill skill;

  const _SkillDetailSheet({required this.skill});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 34),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Center(
            child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
          ),
          const SizedBox(height: 20),
          Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(Icons.auto_awesome, size: 22, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(skill.name, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text('v${skill.version}', style: AppTypography.label(context)),
                  ],
                ),
              ),
              AmitiaStatusBadge(
                label: skill.isEnabled ? '已启用' : '已停用',
                type: skill.isEnabled ? BadgeType.success : BadgeType.neutral,
              ),
            ],
          ),
          const SizedBox(height: 16),
          Text(skill.description, style: AppTypography.bodySmall(context)),
          const SizedBox(height: 16),
          Text('SKILL.md 内容', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: 8),
          Container(
            width: double.infinity,
            constraints: const BoxConstraints(maxHeight: 200),
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brSmall,
            ),
            child: SingleChildScrollView(
              child: Text(skill.skillMd, style: AppTypography.label(context).copyWith(fontFamily: 'monospace', height: 1.6)),
            ),
          ),
          const SizedBox(height: 16),
          Text('所需 MCP', style: AppTypography.caption(context).copyWith(fontWeight: FontWeight.w600)),
          const SizedBox(height: 6),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: skill.requiredMcp.map((mcp) => Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
              decoration: BoxDecoration(
                color: context.info.withValues(alpha: 0.1),
                borderRadius: AppRadius.brTag,
              ),
              child: Text(mcp, style: TextStyle(fontSize: 12, color: context.info, fontWeight: FontWeight.w500)),
            )).toList(),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              Expanded(child: Text('兼容性', style: AppTypography.caption(context))),
              AmitiaStatusBadge(label: skill.compatibility, type: skill.compatibility == '完全兼容' ? BadgeType.success : (skill.compatibility == '部分兼容' ? BadgeType.warning : BadgeType.info)),
            ],
          ),
          const SizedBox(height: 20),
          AmitiaButton(label: '关闭', isFullWidth: true, isSecondary: true, onPressed: () => Navigator.pop(context)),
        ],
      ),
    );
  }
}
