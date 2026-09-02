import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/safety.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

final _safetyConfigProvider = FutureProvider<SafetyConfigDto>((ref) async {
  final svc = ref.read(safetyServiceProvider);
  return await svc.config() ?? const SafetyConfigDto();
});

class SafetyPage extends ConsumerWidget {
  const SafetyPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final configAsync = ref.watch(_safetyConfigProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '安全设置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: configAsync.when(
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
                AmitiaButton(label: '重试', onPressed: () => ref.invalidate(_safetyConfigProvider)),
              ],
            ),
          ),
        ),
        data: (config) => _SafetyContent(config: config),
      ),
    );
  }
}

class _SafetyContent extends ConsumerStatefulWidget {
  final SafetyConfigDto config;
  const _SafetyContent({required this.config});

  @override
  ConsumerState<_SafetyContent> createState() => _SafetyContentState();
}

class _SafetyContentState extends ConsumerState<_SafetyContent> {
  late bool _preventEmotionalBlackmail;
  late bool _preventExclusiveDependency;
  late bool _preventRealityIsolation;
  late bool _preventPunitiveExpression;
  late bool _preventPretendingHuman;
  late bool _preventSensitiveProactiveMention;
  late bool _restrictAdultContent;
  late int _negativeEmotionCap;
  late int _intimacyExpressionCap;
  late String _violationAction;
  late int _auditLogRetentionDays;
  bool _saving = false;
  bool _logsLoading = false;

  @override
  void initState() {
    super.initState();
    final config = widget.config;
    _preventEmotionalBlackmail = config.preventEmotionalBlackmail;
    _preventExclusiveDependency = config.preventExclusiveDependency;
    _preventRealityIsolation = config.preventRealityIsolation;
    _preventPunitiveExpression = config.preventPunitiveExpression;
    _preventPretendingHuman = config.preventPretendingHuman;
    _preventSensitiveProactiveMention = config.preventSensitiveProactiveMention;
    _restrictAdultContent = config.restrictAdultContent;
    _negativeEmotionCap = config.negativeEmotionCap;
    _intimacyExpressionCap = config.intimacyExpressionCap;
    _violationAction = config.violationAction;
    _auditLogRetentionDays = config.auditLogRetentionDays;
  }

  SafetyConfigDto get _current => SafetyConfigDto(
        preventEmotionalBlackmail: _preventEmotionalBlackmail,
        preventExclusiveDependency: _preventExclusiveDependency,
        preventRealityIsolation: _preventRealityIsolation,
        preventPunitiveExpression: _preventPunitiveExpression,
        preventPretendingHuman: _preventPretendingHuman,
        preventSensitiveProactiveMention: _preventSensitiveProactiveMention,
        restrictAdultContent: _restrictAdultContent,
        negativeEmotionCap: _negativeEmotionCap,
        intimacyExpressionCap: _intimacyExpressionCap,
        violationAction: _violationAction,
        auditLogRetentionDays: _auditLogRetentionDays,
      );

  Future<void> _saveConfig() async {
    setState(() => _saving = true);
    try {
      await ref.read(safetyServiceProvider).updateConfig(_current.toJson());
      if (!mounted) return;
      ref.invalidate(_safetyConfigProvider);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: const Text('安全设置已保存'), backgroundColor: context.success),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('保存失败: $e'), backgroundColor: context.error),
      );
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Future<void> _showAuditLogs() async {
    if (_logsLoading) return;
    setState(() => _logsLoading = true);
    try {
      final logs = await ref.read(safetyServiceProvider).auditLogs();
      if (!mounted) return;
      await showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        backgroundColor: context.surfacePrimary,
        shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
        builder: (sheetContext) => SafeArea(
          child: SizedBox(
            height: MediaQuery.sizeOf(sheetContext).height * 0.72,
            child: Column(
              children: [
                Padding(
                  padding: EdgeInsets.all(AppSpacing.lg),
                  child: Row(
                    children: [
                      Expanded(child: Text('安全审计记录', style: AppTypography.sectionTitle(sheetContext))),
                      IconButton(onPressed: () => Navigator.pop(sheetContext), icon: const Icon(Icons.close)),
                    ],
                  ),
                ),
                Divider(height: 1, color: sheetContext.borderSecondary),
                Expanded(
                  child: logs.isEmpty
                      ? Center(child: Text('暂无安全审计记录', style: AppTypography.caption(sheetContext)))
                      : ListView.separated(
                          padding: EdgeInsets.all(AppSpacing.md),
                          itemCount: logs.length,
                          separatorBuilder: (_, __) => Divider(height: 1, color: sheetContext.borderSecondary),
                          itemBuilder: (_, index) {
                            final log = logs[index];
                            final ruleId = (log['ruleId'] ?? '未标记规则').toString();
                            final action = (log['action'] ?? '').toString();
                            final time = (log['time'] ?? '').toString();
                            return ListTile(
                              contentPadding: EdgeInsets.symmetric(horizontal: AppSpacing.sm),
                              title: Text(ruleId, style: AppTypography.body(sheetContext)),
                              subtitle: Text(
                                [if (action.isNotEmpty) action, if (time.isNotEmpty) time].join(' · '),
                                style: AppTypography.caption(sheetContext),
                              ),
                            );
                          },
                        ),
                ),
              ],
            ),
          ),
        ),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('读取审计记录失败: $e'), backgroundColor: context.error),
      );
    } finally {
      if (mounted) setState(() => _logsLoading = false);
    }
  }

  Future<void> _showBdiConfig() async {
    try {
      final current = await ref.read(systemServiceProvider).bdiConfig() ?? <String, dynamic>{};
      if (!mounted) return;
      final controller = TextEditingController(text: const JsonEncoder.withIndent('  ').convert(current));
      final saved = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('BDI 安全策略'),
          content: SizedBox(
            width: 680,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('可配置 hardConstraints、softPreferences、copingStrategy 和 emotionExpression。保存时由后端统一持久化。', style: AppTypography.caption(dialogContext)),
                const SizedBox(height: 12),
                SizedBox(
                  height: MediaQuery.sizeOf(dialogContext).height * 0.5,
                  child: TextField(
                    controller: controller,
                    expands: true,
                    minLines: null,
                    maxLines: null,
                    textAlignVertical: TextAlignVertical.top,
                    keyboardType: TextInputType.multiline,
                    decoration: const InputDecoration(border: OutlineInputBorder(), hintText: 'BDI JSON'),
                  ),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
            FilledButton(
              onPressed: () async {
                try {
                  final decoded = jsonDecode(controller.text);
                  if (decoded is! Map) throw const FormatException('根节点必须是 JSON 对象');
                  await ref.read(systemServiceProvider).updateBdiConfig(Map<String, dynamic>.from(decoded));
                  if (dialogContext.mounted) Navigator.pop(dialogContext, true);
                } catch (e) {
                  if (dialogContext.mounted) {
                    ScaffoldMessenger.of(dialogContext).showSnackBar(SnackBar(content: Text('保存失败：$e')));
                  }
                }
              },
              child: const Text('保存'),
            ),
          ],
        ),
      );
      controller.dispose();
      if (saved == true && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('BDI 安全策略已保存')));
      }
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('读取 BDI 配置失败：$e')));
    }
  }

  Future<void> _showSafetyEvents() async {
    try {
      final result = await ref.read(systemServiceProvider).safetyEvents(pageSize: 100);
      if (!mounted) return;
      final raw = result['items'];
      final items = raw is List ? raw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : <Map<String, dynamic>>[];
      await showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        backgroundColor: context.surfacePrimary,
        builder: (sheetContext) => SafeArea(
          child: SizedBox(
            height: MediaQuery.sizeOf(sheetContext).height * 0.75,
            child: Column(
              children: [
                Padding(
                  padding: EdgeInsets.all(AppSpacing.lg),
                  child: Row(children: [
                    Expanded(child: Text('安全事件 (${result['total'] ?? items.length})', style: AppTypography.sectionTitle(sheetContext))),
                    TextButton.icon(
                      onPressed: items.isEmpty ? null : () async {
                        final confirmed = await showDialog<bool>(
                          context: sheetContext,
                          builder: (ctx) => AlertDialog(
                            title: const Text('清空安全事件'),
                            content: const Text('确定清空全部安全事件日志吗？此操作不可撤销。'),
                            actions: [
                              TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
                              TextButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('清空')),
                            ],
                          ),
                        );
                        if (confirmed != true) return;
                        await ref.read(systemServiceProvider).clearSafetyEvents();
                        if (sheetContext.mounted) Navigator.pop(sheetContext);
                        if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('安全事件已清空')));
                      },
                      icon: const Icon(Icons.delete_sweep_outlined, size: 18),
                      label: const Text('清空'),
                    ),
                    IconButton(onPressed: () => Navigator.pop(sheetContext), icon: const Icon(Icons.close)),
                  ]),
                ),
                Divider(height: 1, color: sheetContext.borderSecondary),
                Expanded(
                  child: items.isEmpty
                      ? Center(child: Text('暂无安全事件', style: AppTypography.caption(sheetContext)))
                      : ListView.separated(
                          itemCount: items.length,
                          separatorBuilder: (_, __) => Divider(height: 1, color: sheetContext.borderSecondary),
                          itemBuilder: (_, index) {
                            final item = items[index];
                            final eventType = (item['eventType'] ?? 'unknown').toString();
                            final description = (item['description'] ?? '').toString();
                            final direction = (item['direction'] ?? '').toString();
                            final createdAt = (item['createdAt'] ?? '').toString();
                            final handled = item['handled'] == true || item['handled'] == 1;
                            return ListTile(
                              leading: Icon(handled ? Icons.verified_outlined : Icons.warning_amber_outlined, color: handled ? sheetContext.success : sheetContext.warning),
                              title: Text(eventType, style: AppTypography.body(sheetContext)),
                              subtitle: Text([description, direction, createdAt].where((e) => e.isNotEmpty).join('\n'), style: AppTypography.caption(sheetContext)),
                              isThreeLine: description.isNotEmpty,
                            );
                          },
                        ),
                ),
              ],
            ),
          ),
        ),
      );
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('读取安全事件失败：$e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
      children: [
        const _SectionLabel(text: '关系与表达边界'),
        SizedBox(height: AppSpacing.sm),
        _buildCard([
          _switch('防止情感勒索', '拦截以 guilt、威胁或压力强迫用户互动的表达', _preventEmotionalBlackmail,
              (v) => setState(() => _preventEmotionalBlackmail = v)),
          _divider(),
          _switch('防止排他依赖', '避免要求用户只与当前角色建立关系', _preventExclusiveDependency,
              (v) => setState(() => _preventExclusiveDependency = v)),
          _divider(),
          _switch('防止现实隔离', '避免鼓励用户脱离现实社交和日常生活', _preventRealityIsolation,
              (v) => setState(() => _preventRealityIsolation = v)),
          _divider(),
          _switch('防止惩罚性表达', '限制冷暴力、惩罚和报复式互动', _preventPunitiveExpression,
              (v) => setState(() => _preventPunitiveExpression = v)),
          _divider(),
          _switch('禁止假装真人', '明确 AI 身份，不虚构真人身份', _preventPretendingHuman,
              (v) => setState(() => _preventPretendingHuman = v)),
          _divider(),
          _switch('限制敏感主动提及', '主动消息中减少敏感主题触发', _preventSensitiveProactiveMention,
              (v) => setState(() => _preventSensitiveProactiveMention = v)),
          _divider(),
          _switch('限制成人内容', '启用后按后端安全策略限制成人内容', _restrictAdultContent,
              (v) => setState(() => _restrictAdultContent = v)),
        ]),
        SizedBox(height: AppSpacing.sectionGap),
        const _SectionLabel(text: '表达强度'),
        SizedBox(height: AppSpacing.sm),
        _buildCard([
          _sliderTile(
            title: '负面情绪上限',
            value: _negativeEmotionCap,
            onChanged: (v) => setState(() => _negativeEmotionCap = v),
          ),
          _divider(),
          _sliderTile(
            title: '亲密表达上限',
            value: _intimacyExpressionCap,
            onChanged: (v) => setState(() => _intimacyExpressionCap = v),
          ),
          _divider(),
          _optionTile(
            title: '违规处理方式',
            value: _violationAction,
            options: const {'block': '阻止', 'warn': '警告', 'audit': '仅记录'},
            onChanged: (v) => setState(() => _violationAction = v),
          ),
          _divider(),
          _retentionTile(),
        ]),
        SizedBox(height: AppSpacing.sectionGap),
        const _SectionLabel(text: '隐私与审计'),
        SizedBox(height: AppSpacing.sm),
        _buildCard([
          _navTile(Icons.security, '隐私扫描', () => context.push(AppRoutes.settingsPrivacyScan)),
          _divider(),
          _navTile(Icons.rule_folder_outlined, 'BDI 安全策略', _showBdiConfig),
          _divider(),
          _navTile(Icons.warning_amber_outlined, '安全事件', _showSafetyEvents),
          _divider(),
          _navTile(Icons.receipt_long_outlined, _logsLoading ? '读取审计记录...' : '安全审计记录', _showAuditLogs),
        ]),
        SizedBox(height: AppSpacing.sectionGap),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: AmitiaButton(
            label: _saving ? '保存中...' : '保存配置',
            icon: Icons.check,
            isFullWidth: true,
            onPressed: _saving ? null : _saveConfig,
          ),
        ),
        SizedBox(height: AppSpacing.xl),
      ],
    );
  }

  Widget _buildCard(List<Widget> children) => Container(
        margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Column(children: children),
      );

  Widget _divider() => Padding(
        padding: const EdgeInsets.only(left: 16),
        child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
      );

  Widget _switch(String title, String subtitle, bool value, ValueChanged<bool> onChanged) => AmitiaSwitchTile(
        title: title,
        subtitle: subtitle,
        value: value,
        onChanged: onChanged,
      );

  Widget _sliderTile({required String title, required int value, required ValueChanged<int> onChanged}) => Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: AppSpacing.sm),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text(title, style: AppTypography.body(context))),
                Text('$value / 10', style: AppTypography.caption(context)),
              ],
            ),
            Slider(
              value: value.toDouble(),
              min: 0,
              max: 10,
              divisions: 10,
              onChanged: (v) => onChanged(v.round()),
            ),
          ],
        ),
      );

  Widget _optionTile({
    required String title,
    required String value,
    required Map<String, String> options,
    required ValueChanged<String> onChanged,
  }) => ListTile(
        title: Text(title, style: AppTypography.body(context)),
        trailing: DropdownButtonHideUnderline(
          child: DropdownButton<String>(
            value: options.containsKey(value) ? value : options.keys.first,
            items: options.entries
                .map((entry) => DropdownMenuItem<String>(value: entry.key, child: Text(entry.value)))
                .toList(growable: false),
            onChanged: (next) {
              if (next != null) onChanged(next);
            },
          ),
        ),
      );

  Widget _retentionTile() => ListTile(
        title: Text('审计日志保留天数', style: AppTypography.body(context)),
        subtitle: Text('后端安全审计日志保留周期', style: AppTypography.caption(context)),
        trailing: SizedBox(
          width: 96,
          child: TextFormField(
            key: ValueKey(_auditLogRetentionDays),
            initialValue: _auditLogRetentionDays.toString(),
            keyboardType: TextInputType.number,
            textAlign: TextAlign.end,
            decoration: const InputDecoration(isDense: true, border: InputBorder.none, suffixText: '天'),
            onChanged: (value) {
              final parsed = int.tryParse(value);
              if (parsed != null) _auditLogRetentionDays = parsed.clamp(1, 3650).toInt();
            },
          ),
        ),
      );

  Widget _navTile(IconData icon, String title, VoidCallback onTap) => ListTile(
        leading: Container(
          width: 32,
          height: 32,
          decoration: BoxDecoration(color: context.accentSoft, shape: BoxShape.circle),
          child: Icon(icon, size: 17, color: context.accentPrimary),
        ),
        title: Text(title, style: AppTypography.body(context)),
        trailing: Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
        onTap: onTap,
      );
}

class _SectionLabel extends StatelessWidget {
  final String text;
  const _SectionLabel({required this.text});

  @override
  Widget build(BuildContext context) => Padding(
        padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
        child: Text(text, style: AppTypography.caption(context)),
      );
}
