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
import '../../../../core/services/providers.dart';

class AgentSkillsPage extends ConsumerStatefulWidget {
  const AgentSkillsPage({super.key});

  @override
  ConsumerState<AgentSkillsPage> createState() => _AgentSkillsPageState();
}

class _AgentSkillsPageState extends ConsumerState<AgentSkillsPage> {
  List<Map<String, dynamic>> _skills = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadSkills();
  }

  Future<void> _loadSkills() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(extensionServiceProvider);
      final data = await svc.agentSkills();
      if (mounted) setState(() { _skills = data; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
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
    if (_loading) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: 'Agent Skills', showBackButton: true),
        body: SafeArea(top: false, child: const AmitiaLoadingState(message: '加载中...')),
      );
    }
    if (_error != null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: 'Agent Skills', showBackButton: true),
        body: SafeArea(top: false, child: AmitiaErrorState(message: '加载失败: $_error', onRetry: _loadSkills)),
      );
    }
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

  Widget _buildSkillCard(BuildContext context, Map<String, dynamic> skill) {
    final isEnabled = (skill['isEnabled'] as bool?) ?? ((skill['enabled'] as int?) == 1);
    final name = (skill['name'] ?? '').toString();
    final description = (skill['description'] ?? '').toString();
    final version = (skill['version'] ?? '1.0.0').toString();
    final skillMd = (skill['skillMd'] ?? skill['skill_md'] ?? '').toString();
    final compatibility = (skill['compatibility'] ?? '兼容').toString();
    final requiredMcp = (skill['requiredMcp'] as List?)?.map((e) => e.toString()).toList() ?? [];

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
                        Text(name, style: AppTypography.cardTitle(context)),
                        const SizedBox(width: 8),
                        Text('v$version', style: AppTypography.label(context)),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text(description, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                  ],
                ),
              ),
              AmitiaStatusBadge(
                label: isEnabled ? '已启用' : '已停用',
                type: isEnabled ? BadgeType.success : BadgeType.neutral,
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
                  skillMd,
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
              ...requiredMcp.map((mcp) => Container(
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
                final badgeType = _compatibilityBadgeType(compatibility);
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
                      Text(compatibility, style: TextStyle(fontSize: 11, color: color, fontWeight: FontWeight.w500)),
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
                    color: isEnabled ? context.warning.withValues(alpha: 0.1) : context.success.withValues(alpha: 0.1),
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(isEnabled ? Icons.pause_circle_outline : Icons.play_circle_outline, size: 15, color: isEnabled ? context.warning : context.success),
                      const SizedBox(width: 5),
                      Text(isEnabled ? '停用' : '启用', style: TextStyle(fontSize: 13, color: isEnabled ? context.warning : context.success, fontWeight: FontWeight.w500)),
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
        _loadSkills();
        ScaffoldMessenger.of(this.context).showSnackBar(
          SnackBar(content: const Text('Agent Skill 导入成功'), backgroundColor: context.success),
        );
      }),
    );
  }

  void _showDetailSheet(Map<String, dynamic> skill) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (context) => _SkillDetailSheet(skill: skill),
    );
  }

  Future<void> _toggleSkill(Map<String, dynamic> skill) async {
    final id = (skill['id'] ?? '').toString();
    final isEnabled = (skill['isEnabled'] as bool?) ?? ((skill['enabled'] as int?) == 1);
    try {
      final svc = ref.read(extensionServiceProvider);
      if (isEnabled) {
        await svc.disableAgentSkill(id);
      } else {
        await svc.enableAgentSkill(id);
      }
      _loadSkills();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('${skill['name'] ?? ''} 已${isEnabled ? '停用' : '启用'}'),
            backgroundColor: context.accentPrimary,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('操作失败: $e'), backgroundColor: context.error),
        );
      }
    }
  }

  Future<void> _showRemoveConfirm(Map<String, dynamic> skill) async {
    final id = (skill['id'] ?? '').toString();
    final name = (skill['name'] ?? '').toString();
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('移除 Agent Skill', style: AppTypography.cardTitle(context)),
        content: Text('确定要移除「$name」吗？此操作不可撤销。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () async {
              Navigator.pop(context);
              try {
                final svc = ref.read(extensionServiceProvider);
                await svc.removeAgentSkill(id);
                _loadSkills();
                if (mounted) {
                  ScaffoldMessenger.of(this.context).showSnackBar(
                    SnackBar(content: Text('$name 已移除'), backgroundColor: context.error),
                  );
                }
              } catch (e) {
                if (mounted) {
                  ScaffoldMessenger.of(this.context).showSnackBar(
                    SnackBar(content: Text('移除失败: $e'), backgroundColor: context.error),
                  );
                }
              }
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
  final Map<String, dynamic> skill;

  const _SkillDetailSheet({required this.skill});

  @override
  Widget build(BuildContext context) {
    final isEnabled = (skill['isEnabled'] as bool?) ?? ((skill['enabled'] as int?) == 1);
    final name = (skill['name'] ?? '').toString();
    final description = (skill['description'] ?? '').toString();
    final version = (skill['version'] ?? '1.0.0').toString();
    final skillMd = (skill['skillMd'] ?? skill['skill_md'] ?? '').toString();
    final compatibility = (skill['compatibility'] ?? '兼容').toString();
    final requiredMcp = (skill['requiredMcp'] as List?)?.map((e) => e.toString()).toList() ?? [];

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
                    Text(name, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text('v$version', style: AppTypography.label(context)),
                  ],
                ),
              ),
              AmitiaStatusBadge(
                label: isEnabled ? '已启用' : '已停用',
                type: isEnabled ? BadgeType.success : BadgeType.neutral,
              ),
            ],
          ),
          const SizedBox(height: 16),
          Text(description, style: AppTypography.bodySmall(context)),
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
              child: Text(skillMd, style: AppTypography.label(context).copyWith(fontFamily: 'monospace', height: 1.6)),
            ),
          ),
          const SizedBox(height: 16),
          Text('所需 MCP', style: AppTypography.caption(context).copyWith(fontWeight: FontWeight.w600)),
          const SizedBox(height: 6),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: requiredMcp.map((mcp) => Container(
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
              AmitiaStatusBadge(label: compatibility, type: compatibility == '完全兼容' ? BadgeType.success : (compatibility == '部分兼容' ? BadgeType.warning : BadgeType.info)),
            ],
          ),
          const SizedBox(height: 20),
          AmitiaButton(label: '关闭', isFullWidth: true, isSecondary: true, onPressed: () => Navigator.pop(context)),
        ],
      ),
    );
  }
}
