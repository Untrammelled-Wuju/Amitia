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
import '../../../../core/widgets/amitia_drawer.dart';
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
  String _characterId = '';

  @override
  void initState() {
    super.initState();
    _loadSkills();
  }

  Future<String> _resolveCharacterId() async {
    final selected = ref.read(currentCharacterIdProvider);
    final characters = await ref.read(characterServiceProvider).list();
    if (characters.isEmpty) {
      throw StateError('请先创建角色后再管理兼容技能');
    }
    final match = characters.where((item) => item.id == selected).firstOrNull;
    final resolved = match?.id ??
        characters.where((item) => item.isActive == 1).firstOrNull?.id ??
        characters.first.id;
    ref.read(currentCharacterIdProvider.notifier).state = resolved;
    return resolved;
  }

  Future<void> _loadSkills() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(extensionServiceProvider);
      final characterId = await _resolveCharacterId();
      final data = await svc.skills(characterId: characterId);
      if (mounted) setState(() { _characterId = characterId; _skills = data; _loading = false; });
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
          padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
          itemCount: _skills.length,
          separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
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
          SizedBox(height: AppSpacing.md),
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
          SizedBox(height: AppSpacing.md),
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
          SizedBox(height: AppSpacing.md),
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

  Future<void> _showVersionCompareDialog(Map<String, dynamic> skill) async {
    final id = (skill['id'] ?? '').toString();
    try {
      final detail = await ref.read(extensionServiceProvider).getSkill(id, characterId: _characterId);
      if (!mounted || detail == null) return;
      final versions = ((detail['versions'] as List?) ?? const [])
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList(growable: false);
      final currentVersion = (detail['version'] ?? skill['version'] ?? '').toString();
      final current = versions.where((item) => item['version']?.toString() == currentVersion).firstOrNull;
      final previous = versions.where((item) => item['version']?.toString() != currentVersion).firstOrNull;
      if (!mounted) return;
      showDialog(
        context: context,
        builder: (dialogContext) => AlertDialog(
          backgroundColor: dialogContext.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
          title: Text('版本信息', style: AppTypography.cardTitle(dialogContext)),
          content: SizedBox(
            width: double.maxFinite,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _VersionInfoCard(label: '当前版本', version: currentVersion, record: current, highlighted: true),
                if (previous != null) ...[
                  const SizedBox(height: 10),
                  _VersionInfoCard(label: '上一可用版本', version: (previous['version'] ?? '').toString(), record: previous),
                ],
                const SizedBox(height: 12),
                Text('能力声明', style: AppTypography.caption(dialogContext).copyWith(fontWeight: FontWeight.w600)),
                const SizedBox(height: 6),
                Text(
                  ((detail['capabilities'] as List?) ?? const []).map((e) => e.toString()).join('、').trim().isEmpty
                      ? '未声明额外能力'
                      : ((detail['capabilities'] as List?) ?? const []).map((e) => e.toString()).join('、'),
                  style: AppTypography.bodySmall(dialogContext),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext), child: Text('关闭', style: TextStyle(color: dialogContext.textSecondary))),
          ],
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('读取版本信息失败: ${safeErrorMessage(e)}'), backgroundColor: context.error));
      }
    }
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
        await svc.disableSkill(id, characterId: _characterId);
      } else {
        await svc.enableSkill(id, characterId: _characterId);
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
    final id = (skill['id'] ?? '').toString();
    final name = (skill['name'] ?? '').toString();
    final version = (skill['version'] ?? '').toString();
    final previousVersion = skill['previousVersion']?.toString();
    if (previousVersion == null || previousVersion.isEmpty) return;
    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: dialogContext.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('回滚版本', style: AppTypography.cardTitle(dialogContext)),
        content: Text('确定要将「$name」从 v$version 回滚到 v$previousVersion 吗？', style: AppTypography.bodySmall(dialogContext)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: Text('取消', style: TextStyle(color: dialogContext.textSecondary))),
          TextButton(
            onPressed: () async {
              Navigator.pop(dialogContext);
              try {
                await ref.read(extensionServiceProvider).rollbackSkill(id, previousVersion, characterId: _characterId);
                await _loadSkills();
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('$name 已回滚到 v$previousVersion'), backgroundColor: context.info));
                }
              } catch (e) {
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('回滚失败: ${safeErrorMessage(e)}'), backgroundColor: context.error));
                }
              }
            },
            child: Text('回滚', style: TextStyle(color: dialogContext.warning)),
          ),
        ],
      ),
    );
  }

  Future<void> _showPermissionSettings(Map<String, dynamic> skill) async {
    final id = (skill['id'] ?? '').toString();
    final name = (skill['name'] ?? '').toString();
    try {
      final detail = await ref.read(extensionServiceProvider).getSkill(id, characterId: _characterId);
      if (!mounted || detail == null) return;
      final grants = ((detail['permissions'] as List?) ?? const [])
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList(growable: false);
      final capabilities = ((detail['capabilities'] as List?) ?? const []).map((e) => e.toString()).toList(growable: false);
      if (!mounted) return;
      showDialog(
        context: context,
        builder: (dialogContext) => _PermissionDialog(
          skillName: name,
          characterId: _characterId,
          capabilities: capabilities,
          grants: grants,
          onSave: (updated) async {
            await ref.read(extensionServiceProvider).updatePermissions(id, {
              'characterId': _characterId,
              'grants': updated,
            });
          },
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('读取权限失败: ${safeErrorMessage(e)}'), backgroundColor: context.error));
      }
    }
  }

  Future<void> _runTest(Map<String, dynamic> skill) async {
    final id = (skill['id'] ?? '').toString();
    final name = (skill['name'] ?? '').toString();
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => _TestRunningDialog(skillName: name),
    );
    Map<String, dynamic>? result;
    Object? failure;
    try {
      result = await ref.read(extensionServiceProvider).executeSkill(id, {
        'characterId': _characterId,
        'channel': 'mobile',
        'input': <String, dynamic>{},
        'idempotencyKey': 'mobile-test-$id-${DateTime.now().microsecondsSinceEpoch}',
      });
    } catch (e) {
      failure = e;
    }
    if (!mounted) return;
    Navigator.of(context, rootNavigator: true).pop();
    final status = (result?['status'] ?? '').toString();
    final passed = failure == null && (status == 'succeeded' || status == 'partially_succeeded');
    showDialog(
      context: context,
      builder: (dialogContext) => _TestResultDialog(
        skillName: name,
        isPassed: passed,
        result: result ?? const <String, dynamic>{},
        errorText: failure == null ? null : safeErrorMessage(failure),
        onConfirm: () => Navigator.pop(dialogContext),
      ),
    );
    await _loadSkills();
  }

  Future<void> _showExecutionHistory(Map<String, dynamic> skill) async {
    final id = (skill['id'] ?? '').toString();
    final name = (skill['name'] ?? '').toString();
    try {
      final page = await ref.read(extensionServiceProvider).skillRuns(id, _characterId);
      final runs = ((page['items'] as List?) ?? const [])
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList(growable: false);
      if (!mounted) return;
      showModalBottomSheet(
        context: context,
        isScrollControlled: true,
        backgroundColor: context.surfacePrimary,
        shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
        builder: (sheetContext) => _ExecutionHistorySheet(skillName: name, runs: runs),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('读取执行历史失败: ${safeErrorMessage(e)}'), backgroundColor: context.error));
      }
    }
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

class _VersionInfoCard extends StatelessWidget {
  final String label;
  final String version;
  final Map<String, dynamic>? record;
  final bool highlighted;

  const _VersionInfoCard({required this.label, required this.version, this.record, this.highlighted = false});

  @override
  Widget build(BuildContext context) {
    final checksum = (record?['checksum'] ?? '').toString();
    final createdAt = (record?['createdAt'] ?? '').toString();
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: highlighted ? context.accentSoft : context.surfaceSecondary,
        borderRadius: AppRadius.brSmall,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: AppTypography.label(context).copyWith(color: highlighted ? context.accentPrimary : context.textSecondary)),
          const SizedBox(height: 4),
          Text('v${version.isEmpty ? '未知' : version}', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
          if (createdAt.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text('创建时间：$createdAt', style: AppTypography.label(context)),
          ],
          if (checksum.isNotEmpty) ...[
            const SizedBox(height: 2),
            Text('校验：${checksum.length > 20 ? '${checksum.substring(0, 20)}…' : checksum}', style: AppTypography.label(context)),
          ],
        ],
      ),
    );
  }
}

class _PermissionDialog extends StatefulWidget {
  final String skillName;
  final String characterId;
  final List<String> capabilities;
  final List<Map<String, dynamic>> grants;
  final Future<void> Function(List<Map<String, dynamic>> grants) onSave;

  const _PermissionDialog({
    required this.skillName,
    required this.characterId,
    required this.capabilities,
    required this.grants,
    required this.onSave,
  });

  @override
  State<_PermissionDialog> createState() => _PermissionDialogState();
}

class _PermissionDialogState extends State<_PermissionDialog> {
  late final Map<String, String> _decisions;
  bool _saving = false;
  String? _error;

  static const _decisionLabels = <String, String>{
    'deny': '拒绝',
    'allow_once': '允许一次',
    'allow_character': '允许当前角色',
    'allow_always': '始终允许',
  };

  @override
  void initState() {
    super.initState();
    _decisions = <String, String>{};
    for (final capability in widget.capabilities) {
      final current = widget.grants.where((grant) => grant['capability']?.toString() == capability).firstOrNull;
      final decision = (current?['decision'] ?? 'deny').toString();
      _decisions[capability] = _decisionLabels.containsKey(decision) ? decision : 'deny';
    }
  }

  List<Map<String, dynamic>> _buildGrants() {
    return widget.capabilities.map((capability) {
      final decision = _decisions[capability] ?? 'deny';
      final global = decision == 'allow_always';
      return <String, dynamic>{
        'capability': capability,
        'decision': decision,
        'scopeType': global ? 'global' : 'character',
        'scopeId': global ? '' : widget.characterId,
      };
    }).toList(growable: false);
  }

  Future<void> _save() async {
    if (_saving) return;
    setState(() { _saving = true; _error = null; });
    try {
      await widget.onSave(_buildGrants());
      if (!mounted) return;
      Navigator.pop(context);
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('${widget.skillName} 权限已更新'), backgroundColor: context.success));
    } catch (e) {
      if (mounted) setState(() { _saving = false; _error = safeErrorMessage(e); });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      title: Text('${widget.skillName} - 权限', style: AppTypography.cardTitle(context)),
      content: SizedBox(
        width: double.maxFinite,
        child: widget.capabilities.isEmpty
            ? Text('该技能未声明权限能力。', style: AppTypography.bodySmall(context))
            : SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    for (final capability in widget.capabilities)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 10),
                        child: Row(
                          children: [
                            Expanded(child: Text(capability, style: AppTypography.bodySmall(context))),
                            const SizedBox(width: 12),
                            DropdownButton<String>(
                              value: _decisions[capability] ?? 'deny',
                              underline: const SizedBox.shrink(),
                              items: _decisionLabels.entries
                                  .map((entry) => DropdownMenuItem(value: entry.key, child: Text(entry.value)))
                                  .toList(growable: false),
                              onChanged: _saving ? null : (value) => setState(() => _decisions[capability] = value ?? 'deny'),
                            ),
                          ],
                        ),
                      ),
                    if (_error != null)
                      Align(
                        alignment: Alignment.centerLeft,
                        child: Text(_error!, style: AppTypography.label(context).copyWith(color: context.error)),
                      ),
                  ],
                ),
              ),
      ),
      actions: [
        TextButton(onPressed: _saving ? null : () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
        TextButton(onPressed: _saving || widget.capabilities.isEmpty ? null : _save, child: Text(_saving ? '保存中...' : '保存', style: TextStyle(color: context.accentPrimary))),
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
          SizedBox(height: AppSpacing.md),
          Text('正在执行「$skillName」...', style: AppTypography.bodySmall(context)),
          const SizedBox(height: 4),
          Text('执行结果由后端运行时返回', style: AppTypography.label(context)),
        ],
      ),
    );
  }
}

class _TestResultDialog extends StatelessWidget {
  final String skillName;
  final bool isPassed;
  final Map<String, dynamic> result;
  final String? errorText;
  final VoidCallback onConfirm;

  const _TestResultDialog({
    required this.skillName,
    required this.isPassed,
    required this.result,
    required this.errorText,
    required this.onConfirm,
  });

  @override
  Widget build(BuildContext context) {
    final status = (result['status'] ?? (isPassed ? 'succeeded' : 'failed')).toString();
    final duration = (result['durationMs'] ?? 0).toString();
    final runId = (result['runId'] ?? '').toString();
    final visibleText = (result['visibleText'] ?? '').toString();
    final backendError = result['error'];
    final detail = errorText ?? (backendError is Map ? (backendError['detail'] ?? backendError['message'])?.toString() : null);
    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      title: Row(
        children: [
          Icon(isPassed ? Icons.check_circle : Icons.cancel, color: isPassed ? context.success : context.error, size: 28),
          const SizedBox(width: 10),
          Text('执行结果', style: AppTypography.cardTitle(context)),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('$skillName · $status', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
          const SizedBox(height: 10),
          if (runId.isNotEmpty) Text('Run ID：$runId', style: AppTypography.label(context)),
          Text('耗时：${duration}ms', style: AppTypography.label(context)),
          if (visibleText.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text(visibleText, style: AppTypography.bodySmall(context)),
          ],
          if (detail != null && detail.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text(detail, style: AppTypography.bodySmall(context).copyWith(color: context.error)),
          ],
        ],
      ),
      actions: [
        TextButton(onPressed: onConfirm, child: Text('确定', style: TextStyle(color: context.accentPrimary))),
      ],
    );
  }
}

class _ExecutionHistorySheet extends StatelessWidget {
  final String skillName;
  final List<Map<String, dynamic>> runs;

  const _ExecutionHistorySheet({required this.skillName, required this.runs});

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
            const SizedBox(height: 20),
            Text('$skillName - 执行历史', style: AppTypography.pageTitle(context)),
            const SizedBox(height: 16),
            if (runs.isEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 28),
                child: Center(child: Text('暂无真实执行记录', style: AppTypography.bodySmall(context))),
              )
            else
              Flexible(
                child: ListView.separated(
                  shrinkWrap: true,
                  itemCount: runs.length,
                  separatorBuilder: (_, _) => const SizedBox(height: 8),
                  itemBuilder: (context, index) {
                    final run = runs[index];
                    final status = (run['status'] ?? '').toString();
                    final success = status == 'succeeded' || status == 'partially_succeeded';
                    final duration = (run['durationMs'] ?? 0).toString();
                    final startedAt = (run['startedAt'] ?? '').toString();
                    final detail = (run['errorDetail'] ?? run['outputSummary'] ?? '').toString();
                    return Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                      decoration: BoxDecoration(
                        color: context.surfacePrimary,
                        borderRadius: AppRadius.brSmall,
                        border: Border.all(color: context.borderPrimary, width: 0.5),
                      ),
                      child: Row(
                        children: [
                          Icon(success ? Icons.check_circle : Icons.error_outline, size: 18, color: success ? context.success : context.error),
                          const SizedBox(width: 10),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(startedAt.isEmpty ? (run['runId'] ?? '').toString() : startedAt, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w500)),
                                const SizedBox(height: 2),
                                Text('耗时：${duration}ms${detail.isEmpty ? '' : ' · $detail'}', style: AppTypography.label(context), maxLines: 2, overflow: TextOverflow.ellipsis),
                              ],
                            ),
                          ),
                          AmitiaStatusBadge(label: status.isEmpty ? '未知' : status, type: success ? BadgeType.success : BadgeType.error),
                        ],
                      ),
                    );
                  },
                ),
              ),
            const SizedBox(height: 12),
            AmitiaButton(label: '关闭', isFullWidth: true, isSecondary: true, onPressed: () => Navigator.pop(context)),
          ],
        ),
      ),
    );
  }
}
