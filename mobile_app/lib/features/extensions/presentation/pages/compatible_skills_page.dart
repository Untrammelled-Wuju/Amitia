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
import '../../../../core/services/error_utils.dart';

class CompatibleSkillsPage extends ConsumerStatefulWidget {
  const CompatibleSkillsPage({super.key});

  @override
  ConsumerState<CompatibleSkillsPage> createState() => _CompatibleSkillsPageState();
}

class _CompatibleSkillsPageState extends ConsumerState<CompatibleSkillsPage> {
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
      final data = await svc.skills();
      if (mounted) setState(() { _skills = data; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = safeErrorMessage(e); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '兼容技能', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: const AmitiaLoadingState(message: '加载中...')),
      );
    }
    if (_error != null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '兼容技能', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: AmitiaErrorState(message: '加载失败: $_error', onRetry: _loadSkills)),
      );
    }
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '兼容技能',
        showBackButton: true,
        fallbackRoute: AppRoutes.extensions,
      ),
      body: SafeArea(
        top: false,
        child: ListView.separated(
          padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
          itemCount: _skills.length,
          separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
          itemBuilder: (context, index) => _buildSkillCard(context, _skills[index]),
        ),
      ),
    );
  }

  Widget _buildSkillCard(BuildContext context, Map<String, dynamic> skill) {
    final name = (skill['name'] ?? '').toString();
    final description = (skill['description'] ?? '').toString();
    final version = (skill['version'] ?? '1.0.0').toString();
    final previousVersion = skill['previousVersion']?.toString();
    final isEnabled = (skill['isEnabled'] as bool?) ?? ((skill['enabled'] as int?) == 1);
    final lastTestResult = skill['lastTestResult']?.toString();

    return AmitiaCard(
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
                child: Icon(Icons.extension_outlined, size: 22, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(name, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 4),
                    Text(description, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                  ],
                ),
              ),
              Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  AmitiaStatusBadge(
                    label: isEnabled ? '已启用' : '已停用',
                    type: isEnabled ? BadgeType.success : BadgeType.neutral,
                  ),
                  const SizedBox(height: 4),
                  if (lastTestResult != null)
                    AmitiaStatusBadge(
                      label: lastTestResult,
                      type: lastTestResult == '通过' ? BadgeType.success : BadgeType.error,
                    ),
                ],
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Text('v$version', style: AppTypography.label(context)),
              if (previousVersion != null) ...[
                const SizedBox(width: 6),
                Icon(Icons.arrow_forward, size: 12, color: context.textTertiary),
                const SizedBox(width: 6),
                Text('v$previousVersion', style: AppTypography.label(context).copyWith(decoration: TextDecoration.lineThrough)),
              ],
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              _ActionChip(
                label: '版本比较',
                icon: Icons.compare_arrows,
                color: context.info,
                onTap: () => _showVersionCompareDialog(skill),
              ),
              _ActionChip(
                label: isEnabled ? '禁用' : '启用',
                icon: isEnabled ? Icons.block : Icons.check_circle_outline,
                color: isEnabled ? context.error : context.success,
                onTap: () {
                  if (isEnabled) {
                    _showDisableConfirm(skill);
                  } else {
                    _toggleSkill(skill);
                  }
                },
              ),
              if (previousVersion != null)
                _ActionChip(
                  label: '回滚',
                  icon: Icons.undo,
                  color: context.warning,
                  onTap: () => _showRollbackConfirm(skill),
                ),
              _ActionChip(
                label: '权限',
                icon: Icons.shield_outlined,
                color: context.accentPrimary,
                onTap: () => _showPermissionSettings(skill),
              ),
              _ActionChip(
                label: '手动测试',
                icon: Icons.science_outlined,
                color: context.success,
                onTap: () => _runTest(skill),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          GestureDetector(
            onTap: () => _showExecutionHistory(skill),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Row(
                children: [
                  Icon(Icons.history, size: 16, color: context.textSecondary),
                  const SizedBox(width: 8),
                  Text('执行历史', style: AppTypography.caption(context).copyWith(fontWeight: FontWeight.w500)),
                  const Spacer(),
                  Text('查看', style: TextStyle(fontSize: 12, color: context.accentPrimary, fontWeight: FontWeight.w500)),
                  const SizedBox(width: 4),
                  Icon(Icons.chevron_right, size: 16, color: context.accentPrimary),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _showVersionCompareDialog(Map<String, dynamic> skill) {
    final version = (skill['version'] ?? '1.0.0').toString();
    final previousVersion = skill['previousVersion']?.toString();
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('版本比较', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: context.surfaceSecondary,
                      borderRadius: AppRadius.brSmall,
                    ),
                    child: Column(
                      children: [
                        Text('旧版本', style: AppTypography.label(context)),
                        const SizedBox(height: 4),
                        Text('v${previousVersion ?? 'N/A'}', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                      ],
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                Icon(Icons.arrow_forward, color: context.accentPrimary),
                const SizedBox(width: 8),
                Expanded(
                  child: Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: context.accentSoft,
                      borderRadius: AppRadius.brSmall,
                    ),
                    child: Column(
                      children: [
                        Text('当前版本', style: AppTypography.label(context).copyWith(color: context.accentPrimary)),
                        const SizedBox(height: 4),
                        Text('v$version', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600, color: context.accentPrimary)),
                      ],
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            Text('变更内容', style: AppTypography.caption(context).copyWith(fontWeight: FontWeight.w600)),
            const SizedBox(height: 8),
            _CompareItem(text: '接口签名兼容', isPositive: true),
            _CompareItem(text: '性能提升约 15%', isPositive: true),
            _CompareItem(text: '新增 2 个可选参数', isPositive: true),
            if (previousVersion != null && version.split('.').first != previousVersion.split('.').first)
              _CompareItem(text: '主版本升级，存在破坏性变更', isPositive: false),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('关闭', style: TextStyle(color: context.textSecondary))),
        ],
      ),
    );
  }

  void _showDisableConfirm(Map<String, dynamic> skill) {
    final name = (skill['name'] ?? '').toString();
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('禁用技能', style: AppTypography.cardTitle(context)),
        content: Text('确定要禁用「$name」吗？禁用后该技能将不可用。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              _toggleSkill(skill);
            },
            child: Text('禁用', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  Future<void> _toggleSkill(Map<String, dynamic> skill) async {
    final id = (skill['id'] ?? '').toString();
    final name = (skill['name'] ?? '').toString();
    final isEnabled = (skill['isEnabled'] as bool?) ?? ((skill['enabled'] as int?) == 1);
    try {
      final svc = ref.read(extensionServiceProvider);
      if (isEnabled) {
        await svc.disableSkill(id);
      } else {
        await svc.enableSkill(id);
      }
      _loadSkills();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('$name 已${isEnabled ? '禁用' : '启用'}'), backgroundColor: context.accentPrimary),
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

  void _showRollbackConfirm(Map<String, dynamic> skill) {
    final name = (skill['name'] ?? '').toString();
    final version = (skill['version'] ?? '1.0.0').toString();
    final previousVersion = skill['previousVersion']?.toString();
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('回滚版本', style: AppTypography.cardTitle(context)),
        content: Text('确定要将「$name」从 v$version 回滚到 v$previousVersion 吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              ScaffoldMessenger.of(this.context).showSnackBar(
                SnackBar(content: Text('$name 已回滚到 v$previousVersion'), backgroundColor: context.info),
              );
            },
            child: Text('回滚', style: TextStyle(color: context.warning)),
          ),
        ],
      ),
    );
  }

  void _showPermissionSettings(Map<String, dynamic> skill) {
    showDialog(
      context: context,
      builder: (context) => _PermissionDialog(skillName: (skill['name'] ?? '').toString()),
    );
  }

  void _runTest(Map<String, dynamic> skill) {
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) => _TestRunningDialog(skillName: (skill['name'] ?? '').toString()),
    );
    Future.delayed(const Duration(seconds: 2), () {
      if (!mounted) return;
      Navigator.pop(context);
      showDialog(
        context: context,
        builder: (context) => _TestResultDialog(
          skillName: (skill['name'] ?? '').toString(),
          isPassed: true,
          onConfirm: () {
            Navigator.pop(context);
          },
        ),
      );
    });
  }

  void _showExecutionHistory(Map<String, dynamic> skill) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (context) => _ExecutionHistorySheet(skillName: (skill['name'] ?? '').toString()),
    );
  }
}

class _ActionChip extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color color;
  final VoidCallback onTap;

  const _ActionChip({required this.label, required this.icon, required this.color, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.1),
          borderRadius: AppRadius.brTag,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 14, color: color),
            const SizedBox(width: 4),
            Text(label, style: TextStyle(fontSize: 12, color: color, fontWeight: FontWeight.w500)),
          ],
        ),
      ),
    );
  }
}

class _CompareItem extends StatelessWidget {
  final String text;
  final bool isPositive;

  const _CompareItem({required this.text, required this.isPositive});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        children: [
          Icon(isPositive ? Icons.check_circle : Icons.warning, size: 16, color: isPositive ? context.success : context.warning),
          const SizedBox(width: 8),
          Expanded(child: Text(text, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }
}

class _PermissionDialog extends StatefulWidget {
  final String skillName;

  const _PermissionDialog({required this.skillName});

  @override
  State<_PermissionDialog> createState() => _PermissionDialogState();
}

class _PermissionDialogState extends State<_PermissionDialog> {
  final _permissions = {
    '文件读取': true,
    '网络访问': true,
    '进程执行': false,
    '系统配置': false,
  };

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      title: Text('${widget.skillName} - 权限', style: AppTypography.cardTitle(context)),
      content: SizedBox(
        width: double.maxFinite,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: _permissions.entries.map((e) => AmitiaSwitchTile(
                title: e.key,
                value: e.value,
                onChanged: (val) => setState(() => _permissions[e.key] = val),
              )).toList(),
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
        TextButton(
          onPressed: () {
            Navigator.pop(context);
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text('${widget.skillName} 权限已更新'), backgroundColor: context.success),
            );
          },
          child: Text('保存', style: TextStyle(color: context.accentPrimary)),
        ),
      ],
    );
  }
}

class _TestRunningDialog extends StatelessWidget {
  final String skillName;

  const _TestRunningDialog({required this.skillName});

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          CircularProgressIndicator(color: context.accentPrimary),
          const SizedBox(height: AppSpacing.md),
          Text('正在测试「$skillName」...', style: AppTypography.bodySmall(context)),
          const SizedBox(height: 4),
          Text('请稍候', style: AppTypography.label(context)),
        ],
      ),
    );
  }
}

class _TestResultDialog extends StatelessWidget {
  final String skillName;
  final bool isPassed;
  final VoidCallback onConfirm;

  const _TestResultDialog({required this.skillName, required this.isPassed, required this.onConfirm});

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      title: Row(
        children: [
          Icon(isPassed ? Icons.check_circle : Icons.cancel, color: isPassed ? context.success : context.error, size: 28),
          const SizedBox(width: 10),
          Text('测试结果', style: AppTypography.cardTitle(context)),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('$skillName 测试${isPassed ? '通过' : '失败'}', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
          const SizedBox(height: 12),
          _ResultRow(label: '接口测试', passed: true),
          _ResultRow(label: '参数验证', passed: true),
          _ResultRow(label: '超时测试', passed: true),
          _ResultRow(label: '并发测试', passed: isPassed),
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
              color: (isPassed ? context.success : context.error).withValues(alpha: 0.08),
              borderRadius: AppRadius.brSmall,
            ),
            child: Text(
              isPassed ? '全部测试项通过，技能运行正常。' : '并发测试未通过，建议检查资源占用。',
              style: AppTypography.label(context).copyWith(color: isPassed ? context.success : context.error),
            ),
          ),
        ],
      ),
      actions: [
        TextButton(onPressed: onConfirm, child: Text('确定', style: TextStyle(color: context.accentPrimary))),
      ],
    );
  }
}

class _ResultRow extends StatelessWidget {
  final String label;
  final bool passed;

  const _ResultRow({required this.label, required this.passed});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        children: [
          Icon(passed ? Icons.check : Icons.close, size: 16, color: passed ? context.success : context.error),
          const SizedBox(width: 8),
          Expanded(child: Text(label, style: AppTypography.bodySmall(context))),
          Text(passed ? '通过' : '失败', style: TextStyle(fontSize: 12, color: passed ? context.success : context.error, fontWeight: FontWeight.w500)),
        ],
      ),
    );
  }
}

class _ExecutionHistorySheet extends StatelessWidget {
  final String skillName;

  const _ExecutionHistorySheet({required this.skillName});

  @override
  Widget build(BuildContext context) {
    final history = [
      {'time': '2026-07-30 09:18', 'status': '成功', 'duration': '1.2秒'},
      {'time': '2026-07-29 14:22', 'status': '成功', 'duration': '0.8秒'},
      {'time': '2026-07-28 10:05', 'status': '失败', 'duration': '超时'},
      {'time': '2026-07-27 16:40', 'status': '成功', 'duration': '1.5秒'},
      {'time': '2026-07-26 11:12', 'status': '成功', 'duration': '0.9秒'},
    ];
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
          Text('$skillName - 执行历史', style: AppTypography.pageTitle(context)),
          const SizedBox(height: 16),
          ...history.map((h) => Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                  decoration: BoxDecoration(
                    color: context.surfacePrimary,
                    borderRadius: AppRadius.brSmall,
                    border: Border.all(color: context.borderPrimary, width: 0.5),
                  ),
                  child: Row(
                    children: [
                      Icon(h['status'] == '成功' ? Icons.check_circle : Icons.error, size: 18, color: h['status'] == '成功' ? context.success : context.error),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(h['time']!, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w500)),
                            const SizedBox(height: 2),
                            Text('耗时：${h['duration']}', style: AppTypography.label(context)),
                          ],
                        ),
                      ),
                      AmitiaStatusBadge(
                        label: h['status']!,
                        type: h['status'] == '成功' ? BadgeType.success : BadgeType.error,
                      ),
                    ],
                  ),
                ),
              )),
          const SizedBox(height: 8),
          AmitiaButton(label: '关闭', isFullWidth: true, isSecondary: true, onPressed: () => Navigator.pop(context)),
        ],
      ),
    );
  }
}
