import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_extensions.dart';

class SystemPluginsPage extends ConsumerStatefulWidget {
  const SystemPluginsPage({super.key});

  @override
  ConsumerState<SystemPluginsPage> createState() => _SystemPluginsPageState();
}

class _SystemPluginsPageState extends ConsumerState<SystemPluginsPage> {
  late List<SystemPlugin> _plugins;

  @override
  void initState() {
    super.initState();
    _plugins = List.from(MockExtensions.systemPlugins);
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '系统插件',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: ListView.separated(
          padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
          itemCount: _plugins.length,
          separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
          itemBuilder: (context, index) => _buildPluginCard(context, _plugins[index]),
        ),
      ),
    );
  }

  Widget _buildPluginCard(BuildContext context, SystemPlugin plugin) {
    return AmitiaCard(
      onTap: () => _showDetailSheet(plugin),
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
                child: Icon(Icons.extension, size: 22, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Text(plugin.name, style: AppTypography.cardTitle(context)),
                        const SizedBox(width: 8),
                        Text('v${plugin.version}', style: AppTypography.label(context)),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text(plugin.description, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Container(
                width: 8,
                height: 8,
                decoration: BoxDecoration(
                  color: plugin.runtimeStatus == '运行中' ? context.success : context.warning,
                  shape: BoxShape.circle,
                ),
              ),
              const SizedBox(width: 6),
              Text(plugin.runtimeStatus, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w500)),
              const Spacer(),
              AmitiaStatusBadge(
                label: plugin.isEnabled ? '已启用' : '已禁用',
                type: plugin.isEnabled ? BadgeType.success : BadgeType.neutral,
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          _buildInfoChips(context, plugin),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              GestureDetector(
                onTap: () => _showDetailSheet(plugin),
                child: _ActionButton(
                  label: '详情',
                  icon: Icons.info_outline,
                  color: context.accentPrimary,
                ),
              ),
              const SizedBox(width: 8),
              GestureDetector(
                onTap: () => _showPermissionSettings(plugin),
                child: _ActionButton(
                  label: '权限',
                  icon: Icons.shield_outlined,
                  color: context.info,
                ),
              ),
              const SizedBox(width: 8),
              GestureDetector(
                onTap: () {
                  if (plugin.isEnabled) {
                    _showDisableImpactConfirm(plugin);
                  } else {
                    _enablePlugin(plugin);
                  }
                },
                child: _ActionButton(
                  label: plugin.isEnabled ? '禁用' : '启用',
                  icon: plugin.isEnabled ? Icons.block : Icons.check_circle_outline,
                  color: plugin.isEnabled ? context.error : context.success,
                ),
              ),
              const Spacer(),
              Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildInfoChips(BuildContext context, SystemPlugin plugin) {
    return Wrap(
      spacing: 6,
      runSpacing: 6,
      children: [
        if (plugin.hooks.isNotEmpty)
          _InfoChip(label: 'Hook ${plugin.hooks.length}', icon: Icons.link, color: context.accentPrimary),
        if (plugin.events.isNotEmpty)
          _InfoChip(label: '事件 ${plugin.events.length}', icon: Icons.event_outlined, color: context.info),
        if (plugin.schedules.isNotEmpty)
          _InfoChip(label: '调度 ${plugin.schedules.length}', icon: Icons.schedule, color: context.warning),
        if (plugin.registeredSkills.isNotEmpty)
          _InfoChip(label: '技能 ${plugin.registeredSkills.length}', icon: Icons.auto_awesome, color: context.success),
      ],
    );
  }

  void _showDetailSheet(SystemPlugin plugin) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (context) => _PluginDetailSheet(plugin: plugin, onPermissionTap: () {
        Navigator.pop(context);
        _showPermissionSettings(plugin);
      }),
    );
  }

  void _showDisableImpactConfirm(SystemPlugin plugin) {
    showDialog(
      context: context,
      builder: (context) => _DisableImpactDialog(
        plugin: plugin,
        onConfirm: () {
          Navigator.pop(context);
          setState(() {
            final index = _plugins.indexWhere((p) => p.id == plugin.id);
            _plugins[index] = SystemPlugin(
              id: plugin.id,
              name: plugin.name,
              description: plugin.description,
              runtimeStatus: '已暂停',
              hooks: plugin.hooks,
              events: plugin.events,
              schedules: plugin.schedules,
              registeredSkills: plugin.registeredSkills,
              isEnabled: false,
              version: plugin.version,
            );
          });
          ScaffoldMessenger.of(this.context).showSnackBar(
            SnackBar(content: Text('${plugin.name} 已禁用'), backgroundColor: context.error),
          );
        },
      ),
    );
  }

  void _enablePlugin(SystemPlugin plugin) {
    setState(() {
      final index = _plugins.indexWhere((p) => p.id == plugin.id);
      _plugins[index] = SystemPlugin(
        id: plugin.id,
        name: plugin.name,
        description: plugin.description,
        runtimeStatus: '运行中',
        hooks: plugin.hooks,
        events: plugin.events,
        schedules: plugin.schedules,
        registeredSkills: plugin.registeredSkills,
        isEnabled: true,
        version: plugin.version,
      );
    });
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('${plugin.name} 已启用'), backgroundColor: context.success),
    );
  }

  void _showPermissionSettings(SystemPlugin plugin) {
    showDialog(
      context: context,
      builder: (context) => _PermissionSettingsDialog(plugin: plugin),
    );
  }
}

class _ActionButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color color;

  const _ActionButton({required this.label, required this.icon, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: AppRadius.brTag,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 15, color: color),
          const SizedBox(width: 5),
          Text(label, style: TextStyle(fontSize: 13, color: color, fontWeight: FontWeight.w500)),
        ],
      ),
    );
  }
}

class _InfoChip extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color color;

  const _InfoChip({required this.label, required this.icon, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: AppRadius.brTag,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 13, color: color),
          const SizedBox(width: 4),
          Text(label, style: TextStyle(fontSize: 11, color: color, fontWeight: FontWeight.w500)),
        ],
      ),
    );
  }
}

class _PluginDetailSheet extends StatelessWidget {
  final SystemPlugin plugin;
  final VoidCallback onPermissionTap;

  const _PluginDetailSheet({required this.plugin, required this.onPermissionTap});

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
                child: Icon(Icons.extension, size: 22, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(plugin.name, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text('v${plugin.version}', style: AppTypography.label(context)),
                  ],
                ),
              ),
              AmitiaStatusBadge(
                label: plugin.runtimeStatus,
                type: plugin.runtimeStatus == '运行中' ? BadgeType.success : BadgeType.warning,
              ),
            ],
          ),
          const SizedBox(height: 12),
          Text(plugin.description, style: AppTypography.bodySmall(context)),
          const SizedBox(height: 16),
          if (plugin.hooks.isNotEmpty) ...[
            _SectionLabel(label: 'Hook', icon: Icons.link),
            const SizedBox(height: 6),
            ...plugin.hooks.map((h) => _ListItem(text: h)),
          ],
          if (plugin.events.isNotEmpty) ...[
            const SizedBox(height: 12),
            _SectionLabel(label: '事件', icon: Icons.event_outlined),
            const SizedBox(height: 6),
            ...plugin.events.map((e) => _ListItem(text: e)),
          ],
          if (plugin.schedules.isNotEmpty) ...[
            const SizedBox(height: 12),
            _SectionLabel(label: '调度', icon: Icons.schedule),
            const SizedBox(height: 6),
            ...plugin.schedules.map((s) => _ListItem(text: s)),
          ],
          if (plugin.registeredSkills.isNotEmpty) ...[
            const SizedBox(height: 12),
            _SectionLabel(label: '注册技能', icon: Icons.auto_awesome),
            const SizedBox(height: 6),
            ...plugin.registeredSkills.map((s) => _ListItem(text: s)),
          ],
          const SizedBox(height: 20),
          Row(
            children: [
              Expanded(
                child: AmitiaButton(
                  label: '权限设置',
                  isSecondary: true,
                  icon: Icons.shield_outlined,
                  onPressed: onPermissionTap,
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: AmitiaButton(
                  label: '关闭',
                  onPressed: () => Navigator.pop(context),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _SectionLabel extends StatelessWidget {
  final String label;
  final IconData icon;

  const _SectionLabel({required this.label, required this.icon});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(icon, size: 14, color: context.textTertiary),
        const SizedBox(width: 6),
        Text(label, style: AppTypography.caption(context).copyWith(fontWeight: FontWeight.w600)),
      ],
    );
  }
}

class _ListItem extends StatelessWidget {
  final String text;

  const _ListItem({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brTag,
        ),
        child: Text(text, style: AppTypography.label(context).copyWith(fontFamily: 'monospace')),
      ),
    );
  }
}

class _DisableImpactDialog extends StatelessWidget {
  final SystemPlugin plugin;
  final VoidCallback onConfirm;

  const _DisableImpactDialog({required this.plugin, required this.onConfirm});

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      title: Row(
        children: [
          Icon(Icons.warning_amber_rounded, color: context.warning, size: 24),
          const SizedBox(width: 8),
          Text('禁用影响确认', style: AppTypography.cardTitle(context)),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('禁用「${plugin.name}」将影响以下功能：', style: AppTypography.bodySmall(context)),
          const SizedBox(height: 12),
          if (plugin.hooks.isNotEmpty)
            _ImpactItem(text: '${plugin.hooks.length} 个 Hook 将停止响应', color: context.accentPrimary),
          if (plugin.events.isNotEmpty)
            _ImpactItem(text: '${plugin.events.length} 个事件将不再触发', color: context.info),
          if (plugin.schedules.isNotEmpty)
            _ImpactItem(text: '${plugin.schedules.length} 个调度任务将暂停', color: context.warning),
          if (plugin.registeredSkills.isNotEmpty)
            _ImpactItem(text: '${plugin.registeredSkills.length} 个注册技能将不可用', color: context.error),
          if (plugin.hooks.isEmpty && plugin.events.isEmpty && plugin.schedules.isEmpty && plugin.registeredSkills.isEmpty)
            _ImpactItem(text: '无直接影响，可安全禁用', color: context.success),
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
              color: context.warning.withValues(alpha: 0.08),
              borderRadius: AppRadius.brSmall,
            ),
            child: Row(
              children: [
                Icon(Icons.info_outline, size: 16, color: context.warning),
                const SizedBox(width: 8),
                Expanded(child: Text('禁用后相关功能将立即失效，请谨慎操作。', style: AppTypography.label(context).copyWith(color: context.warning))),
              ],
            ),
          ),
        ],
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
        TextButton(onPressed: onConfirm, child: Text('确认禁用', style: TextStyle(color: context.error))),
      ],
    );
  }
}

class _ImpactItem extends StatelessWidget {
  final String text;
  final Color color;

  const _ImpactItem({required this.text, required this.color});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        children: [
          Icon(Icons.arrow_right, size: 18, color: color),
          const SizedBox(width: 4),
          Expanded(child: Text(text, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }
}

class _PermissionSettingsDialog extends StatefulWidget {
  final SystemPlugin plugin;

  const _PermissionSettingsDialog({required this.plugin});

  @override
  State<_PermissionSettingsDialog> createState() => _PermissionSettingsDialogState();
}

class _PermissionSettingsDialogState extends State<_PermissionSettingsDialog> {
  late Map<String, bool> _permissions;

  @override
  void initState() {
    super.initState();
    _permissions = {
      '文件系统访问': true,
      '网络访问': widget.plugin.hooks.any((h) => h.contains('message')),
      '消息读取': true,
      '记忆访问': widget.plugin.name.contains('记忆'),
      '情感分析': widget.plugin.name.contains('情感'),
    };
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      title: Text('${widget.plugin.name} - 权限', style: AppTypography.cardTitle(context)),
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
              SnackBar(content: Text('${widget.plugin.name} 权限已更新'), backgroundColor: context.success),
            );
          },
          child: Text('保存', style: TextStyle(color: context.accentPrimary)),
        ),
      ],
    );
  }
}
