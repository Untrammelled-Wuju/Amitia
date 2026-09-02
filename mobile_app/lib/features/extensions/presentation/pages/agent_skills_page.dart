import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/artifact/artifact_providers.dart';
import '../../../../core/services/error_utils.dart';
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
  String _characterId = '';

  @override
  void initState() {
    super.initState();
    _loadSkills();
  }

  Future<String> _resolveCharacterId() async {
    final selected = ref.read(currentCharacterIdProvider);
    final characters = await ref.read(characterServiceProvider).list();
    if (characters.isEmpty) return '';
    final match = characters.where((item) => item.id == selected).firstOrNull;
    final resolved = match?.id ?? characters.where((item) => item.isActive == 1).firstOrNull?.id ?? characters.first.id;
    ref.read(currentCharacterIdProvider.notifier).state = resolved;
    return resolved;
  }

  Future<void> _loadSkills() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(extensionServiceProvider);
      final characterId = await _resolveCharacterId();
      final data = await svc.agentSkills(characterId: characterId);
      if (mounted) setState(() { _characterId = characterId; _skills = data; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = safeErrorMessage(e); _loading = false; });
    }
  }

  Future<Dio> _dio() async {
    final availability = await ref.read(backendConnectionProvider.future);
    if (availability is! BackendConnectionAvailable) throw StateError('后端当前不可用');
    return createAuthenticatedDio(availability.config);
  }

  BadgeType _compatibilityBadgeType(String compatibility) {
    switch (compatibility.toLowerCase()) {
      case 'compatible':
      case 'fully_compatible':
      case '完全兼容':
        return BadgeType.success;
      case 'partial':
      case 'partially_compatible':
      case '部分兼容':
        return BadgeType.warning;
      case 'incompatible':
      case '不兼容':
        return BadgeType.error;
      case '兼容':
        return BadgeType.info;
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
                padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
                itemCount: _skills.length,
                separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
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
    final isEnabled = (skill['enabled'] as bool?) ?? ((skill['isEnabled'] as bool?) ?? false);
    final name = (skill['name'] ?? '').toString();
    final description = (skill['description'] ?? '').toString();
    final version = ((skill['metadata'] as Map?)?['version'] ?? '').toString();
    final skillMd = (skill['rawSkillMd'] ?? skill['body'] ?? '').toString();
    final compatibility = (skill['compatibilityStatus'] ?? skill['compatibility'] ?? 'unknown').toString();
    final requiredMcp = ((skill['mcpDependencies'] as List?) ?? const []).whereType<Map>().map((e) => (e['id'] ?? e['description'] ?? '').toString()).where((e) => e.isNotEmpty).toList();

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
                        if (version.isNotEmpty) Text('v$version', style: AppTypography.label(context)),
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
          SizedBox(height: AppSpacing.md),
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
          SizedBox(height: AppSpacing.md),
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
          SizedBox(height: AppSpacing.md),
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

  Future<void> _showImportSheet() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['zip'],
      withData: false,
    );
    if (picked == null || picked.files.isEmpty) return;
    final file = picked.files.first;
    if (file.path == null || file.path!.isEmpty) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('无法读取所选文件')));
      return;
    }
    Dio? dio;
    try {
      dio = await _dio();
      final previewResponse = await dio.post(
        '/api/extensions/agent-skills/import/preview',
        data: FormData.fromMap({
          'file': await MultipartFile.fromFile(file.path!, filename: file.name),
        }),
      );
      dynamic preview = previewResponse.data;
      if (preview is Map && preview['data'] is Map) preview = preview['data'];
      if (preview is! Map) throw StateError('后端未返回导入预览');
      final previewMap = Map<String, dynamic>.from(preview);
      final previewId = (previewMap['previewId'] ?? '').toString();
      if (previewId.isEmpty) throw StateError('导入预览缺少 previewId');
      final definition = previewMap['definition'] is Map ? Map<String, dynamic>.from(previewMap['definition'] as Map) : <String, dynamic>{};
      final report = previewMap['compatibilityReport'] is Map ? Map<String, dynamic>.from(previewMap['compatibilityReport'] as Map) : <String, dynamic>{};
      if (!mounted) return;
      String scope = 'global';
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => StatefulBuilder(
          builder: (dialogContext, setDialogState) => AlertDialog(
            backgroundColor: dialogContext.surfacePrimary,
            shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
            title: Text('Agent Skill 导入预览', style: AppTypography.cardTitle(dialogContext)),
            content: SizedBox(
              width: double.maxFinite,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text((definition['displayName'] ?? definition['name'] ?? file.name).toString(), style: AppTypography.bodySmall(dialogContext).copyWith(fontWeight: FontWeight.w600)),
                  const SizedBox(height: 6),
                  Text((definition['description'] ?? '').toString(), style: AppTypography.caption(dialogContext)),
                  const SizedBox(height: 10),
                  Text('兼容性：${report['status'] ?? definition['compatibilityStatus'] ?? 'unknown'}', style: AppTypography.label(dialogContext)),
                  Text('文件数：${(previewMap['files'] as List?)?.length ?? 0}', style: AppTypography.label(dialogContext)),
                  const SizedBox(height: 12),
                  DropdownButtonFormField<String>(
                    initialValue: scope,
                    decoration: const InputDecoration(labelText: '安装范围'),
                    items: [
                      const DropdownMenuItem(value: 'global', child: Text('全局')),
                      if (_characterId.isNotEmpty) const DropdownMenuItem(value: 'character', child: Text('当前角色')),
                    ],
                    onChanged: (value) => setDialogState(() => scope = value ?? 'global'),
                  ),
                  if (((report['errors'] as List?) ?? const []).isNotEmpty) ...[
                    const SizedBox(height: 10),
                    Text('存在兼容性错误，后端可能拒绝安装。', style: AppTypography.label(dialogContext).copyWith(color: dialogContext.error)),
                  ],
                ],
              ),
            ),
            actions: [
              TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
              FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('安装')),
            ],
          ),
        ),
      );
      if (confirmed != true) return;
      final dependencies = (definition['mcpDependencies'] as List?) ?? const <dynamic>[];
      Map<String, dynamic>? dependencyPlan;
      Map<String, bool>? dependencyOptions;
      if (dependencies.isNotEmpty) {
        dependencyPlan = await ref.read(mcpServiceProvider).previewAgentSkillDependencies(
              agentSkillExtensionId: (definition['extensionId'] ?? definition['id'] ?? '').toString(),
              characterId: scope == 'character' ? _characterId : '',
              dependencies: dependencies,
            );
        if (dependencyPlan == null) throw StateError('MCP 依赖计划生成失败');
        dependencyOptions = await _confirmDependencyPlan(dependencyPlan);
        if (dependencyOptions == null) return;
      }

      dynamic installed;
      String installedId = '';
      try {
        final installResponse = await dio.post(
          '/api/extensions/agent-skills/import/install',
          data: {
            'previewId': previewId,
            'scope': scope,
            'characterId': scope == 'character' ? _characterId : '',
            'enable': true,
          },
        );
        installed = installResponse.data;
        if (installed is Map && installed['data'] != null) installed = installed['data'];
        if (installed == null) throw StateError('安装未完成');
        if (installed is Map) installedId = (installed['extensionId'] ?? installed['id'] ?? '').toString();

        if (dependencyPlan != null && dependencyOptions != null) {
          if (installedId.isEmpty) throw StateError('安装结果缺少 extensionId，无法绑定 MCP 依赖');
          dependencyPlan['agentSkillExtensionId'] = installedId;
          final dependencyResult = await ref.read(mcpServiceProvider).installAgentSkillDependencies(
                dependencyPlan,
                installOptional: dependencyOptions['installOptional'] ?? false,
                confirmHttp: dependencyOptions['confirmHttp'] ?? false,
                confirmStdio: dependencyOptions['confirmStdio'] ?? false,
              );
          final authServers = (dependencyResult?['authorizationServerIds'] as List?) ?? const <dynamic>[];
          if (authServers.isNotEmpty && mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text('Skill 已安装；${authServers.length} 个 MCP 依赖仍需要 OAuth 授权'), backgroundColor: context.warning),
            );
          }
        }
      } catch (_) {
        if (installedId.isNotEmpty) {
          await ref.read(mcpServiceProvider).removeAgentSkillDependencies(installedId).catchError((_) => null);
          await ref.read(extensionServiceProvider).removeAgentSkill(installedId).catchError((_) => false);
        }
        rethrow;
      }
      await _loadSkills();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: const Text('Agent Skill 与 MCP 依赖已按确认计划安装'), backgroundColor: context.success));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('导入失败: ${safeErrorMessage(e)}'), backgroundColor: context.error));
    } finally {
      dio?.close(force: true);
    }
  }

  Future<Map<String, bool>?> _confirmDependencyPlan(Map<String, dynamic> plan) async {
    final items = ((plan['items'] as List?) ?? const <dynamic>[])
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .toList();
    bool installOptional = false;
    bool confirmHttp = false;
    bool confirmStdio = false;
    final hasHttp = items.any((item) => ((item['dependency'] as Map?)?['transport'] ?? '').toString() == 'streamable_http' && item['installed'] != true);
    final hasStdio = items.any((item) => ((item['dependency'] as Map?)?['transport'] ?? '').toString() == 'stdio' && item['installed'] != true);
    final hasOptional = items.any((item) => ((item['dependency'] as Map?)?['required'] ?? true) == false);
    if (!mounted) return null;
    return showDialog<Map<String, bool>>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) => AlertDialog(
          title: const Text('确认 MCP 依赖计划'),
          content: SizedBox(
            width: 620,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('风险等级：${plan['riskLevel'] ?? 'unknown'}${plan['requiredMissing'] == true ? ' · 存在必需依赖需要确认/配置' : ''}'),
                  const SizedBox(height: 12),
                  ...items.map((item) {
                    final dep = item['dependency'] is Map ? Map<String, dynamic>.from(item['dependency'] as Map) : <String, dynamic>{};
                    final name = (dep['id'] ?? dep['description'] ?? 'MCP dependency').toString();
                    final transport = (dep['transport'] ?? 'unknown').toString();
                    final required = dep['required'] != false;
                    final installed = item['installed'] == true;
                    final needsConfig = item['needsUserConfiguration'] == true;
                    return ListTile(
                      dense: true,
                      contentPadding: EdgeInsets.zero,
                      leading: Icon(installed ? Icons.check_circle_outline : Icons.link_outlined),
                      title: Text(name),
                      subtitle: Text('$transport · ${required ? '必需' : '可选'}${installed ? ' · 已安装' : ''}${needsConfig ? ' · 需要手动配置' : ''}'),
                    );
                  }),
                  if (hasOptional)
                    CheckboxListTile(
                      contentPadding: EdgeInsets.zero,
                      value: installOptional,
                      onChanged: (value) => setDialogState(() => installOptional = value ?? false),
                      title: const Text('同时安装可选依赖'),
                    ),
                  if (hasHttp)
                    CheckboxListTile(
                      contentPadding: EdgeInsets.zero,
                      value: confirmHttp,
                      onChanged: (value) => setDialogState(() => confirmHttp = value ?? false),
                      title: const Text('确认自动连接远程 HTTP MCP'),
                      subtitle: const Text('可能向远程服务发送工具参数、资源请求或授权信息。'),
                    ),
                  if (hasStdio)
                    CheckboxListTile(
                      contentPadding: EdgeInsets.zero,
                      value: confirmStdio,
                      onChanged: (value) => setDialogState(() => confirmStdio = value ?? false),
                      title: const Text('确认启动本地 STDIO MCP 进程'),
                      subtitle: const Text('会在设备上启动依赖声明中的本地命令。'),
                    ),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext, <String, bool>{
                'installOptional': installOptional,
                'confirmHttp': confirmHttp,
                'confirmStdio': confirmStdio,
              }),
              child: const Text('确认依赖计划'),
            ),
          ],
        ),
      ),
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
    final isEnabled = (skill['enabled'] as bool?) ?? ((skill['isEnabled'] as bool?) ?? false);
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
                final dependencyResult = await ref.read(mcpServiceProvider).removeAgentSkillDependencies(id);
                final svc = ref.read(extensionServiceProvider);
                await svc.removeAgentSkill(id);
                _loadSkills();
                if (mounted) {
                  final unreferenced = (dependencyResult?['unreferencedServerIds'] as List?) ?? const <dynamic>[];
                  ScaffoldMessenger.of(this.context).showSnackBar(
                    SnackBar(
                      content: Text(unreferenced.isEmpty ? '$name 已移除，MCP 依赖引用已解除' : '$name 已移除；${unreferenced.length} 个 MCP 服务已无依赖引用，可在 MCP 页面确认删除'),
                      backgroundColor: context.error,
                    ),
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

BadgeType _compatibilityBadgeTypeStatic(String compatibility) {
  switch (compatibility.toLowerCase()) {
    case 'compatible':
    case 'fully_compatible':
    case '完全兼容':
      return BadgeType.success;
    case 'partial':
    case 'partially_compatible':
    case '部分兼容':
      return BadgeType.warning;
    case 'incompatible':
    case '不兼容':
      return BadgeType.error;
    case '兼容':
      return BadgeType.info;
    default:
      return BadgeType.neutral;
  }
}

class _SkillDetailSheet extends StatelessWidget {
  final Map<String, dynamic> skill;

  const _SkillDetailSheet({required this.skill});

  @override
  Widget build(BuildContext context) {
    final isEnabled = (skill['enabled'] as bool?) ?? ((skill['isEnabled'] as bool?) ?? false);
    final name = (skill['name'] ?? '').toString();
    final description = (skill['description'] ?? '').toString();
    final version = ((skill['metadata'] as Map?)?['version'] ?? '').toString();
    final skillMd = (skill['rawSkillMd'] ?? skill['body'] ?? '').toString();
    final compatibility = (skill['compatibilityStatus'] ?? skill['compatibility'] ?? 'unknown').toString();
    final requiredMcp = ((skill['mcpDependencies'] as List?) ?? const []).whereType<Map>().map((e) => (e['id'] ?? e['description'] ?? '').toString()).where((e) => e.isNotEmpty).toList();

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
                    if (version.isNotEmpty) Text('v$version', style: AppTypography.label(context)),
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
              AmitiaStatusBadge(label: compatibility, type: _compatibilityBadgeTypeStatic(compatibility)),
            ],
          ),
          const SizedBox(height: 20),
          AmitiaButton(label: '关闭', isFullWidth: true, isSecondary: true, onPressed: () => Navigator.pop(context)),
        ],
      ),
    );
  }
}
