import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/ui_runtime/ui_provider.dart';
import '../../../../core/ui_runtime/ui_runtime_controller.dart';

class UIProviderSettingsPage extends ConsumerStatefulWidget {
  const UIProviderSettingsPage({super.key});

  @override
  ConsumerState<UIProviderSettingsPage> createState() => _UIProviderSettingsPageState();
}

class _UIProviderSettingsPageState extends ConsumerState<UIProviderSettingsPage> {
  UIProfileScopeKind _scope = UIProfileScopeKind.user;
  UIProfileEnvelope? _envelope;
  bool _scopeLoading = false;
  String? _savingCapability;

  static const _groups = <String, List<String>>{
    '应用外壳': [UICapability.appShell, UICapability.appNavigation, UICapability.appWorkspace, UICapability.routeRegistry, UICapability.pageProvider],
    '对话': [UICapability.conversationShell, UICapability.conversationHeader, UICapability.conversationMessages, UICapability.conversationMessageRenderer, UICapability.conversationSidebar, UICapability.conversationComposer, UICapability.conversationOverlay],
    '业务页面': [UICapability.characterShell, UICapability.characterDetail, UICapability.memoryShell, UICapability.memoryDetail, UICapability.settingsShell, UICapability.settingsSection, UICapability.extensionCenter, UICapability.extensionPage],
    '设计系统': [UICapability.theme, UICapability.tokens, UICapability.icons, UICapability.components],
  };

  @override
  void initState() {
    super.initState();
    Future.microtask(() async {
      await ref.read(uiRuntimeProvider.notifier).ensureLoaded();
      await _loadScope();
    });
  }

  Future<void> _loadScope() async {
    if (_scopeLoading) return;
    setState(() => _scopeLoading = true);
    try {
      final envelope = await ref.read(uiRuntimeProvider.notifier).loadProfileScope(_scope);
      if (mounted) setState(() => _envelope = envelope);
    } catch (error) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('加载配置层失败：$error')));
    } finally {
      if (mounted) setState(() => _scopeLoading = false);
    }
  }

  Future<void> _changeScope(UIProfileScopeKind? value) async {
    if (value == null || value == _scope) return;
    setState(() { _scope = value; _envelope = null; });
    await _loadScope();
  }

  Future<void> _choose(String capability, String? providerId) async {
    setState(() => _savingCapability = capability);
    try {
      await ref.read(uiRuntimeProvider.notifier).updateSelection(capability, providerId, scope: _scope);
      await _loadScope();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('界面提供者已更新')));
    } catch (error) {
      await _loadScope();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('更新失败（可能存在 revision 冲突）：$error')));
    } finally {
      if (mounted) setState(() => _savingCapability = null);
    }
  }

  Future<void> _resetScope() async {
    if (_scope == UIProfileScopeKind.global) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('清除此层覆盖'),
        content: const Text('清除后将重新继承云端默认/用户/平台等上层 UI Profile。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('清除')),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(uiRuntimeProvider.notifier).resetProfileScope(_scope);
      await _loadScope();
    } catch (error) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('清除失败：$error')));
    }
  }

  String _scopeLabel(UIProfileScopeKind value) => switch (value) {
    UIProfileScopeKind.global => '云端默认（管理员）',
    UIProfileScopeKind.user => '当前用户',
    UIProfileScopeKind.platform => '当前平台',
    UIProfileScopeKind.device => '当前设备',
    UIProfileScopeKind.devicePlatform => '当前设备 + 平台',
    UIProfileScopeKind.runtime => '当前运行时',
  };

  @override
  Widget build(BuildContext context) {
    final runtime = ref.watch(uiRuntimeProvider);
    final snapshot = runtime.valueOrNull;
    final usingLkg = ref.watch(uiRuntimeUsingLastKnownGoodProvider);
    final envelope = _envelope;
    final ctx = snapshot?.context;

    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      appBar: AppBar(
        title: const Text('界面提供者'),
        leading: BackButton(onPressed: () => Navigator.of(context).maybePop()),
      ),
      body: RefreshIndicator(
        onRefresh: () async {
          await ref.read(uiRuntimeProvider.notifier).ensureLoaded(force: true);
          await _loadScope();
        },
        child: ListView(
          padding: EdgeInsets.all(24),
          children: [
            Text('UI Profile 按云端默认 → 用户 → 平台/设备 → 运行时分层合并；本页只修改当前 override 层。', style: TextStyle(color: scheme.onSurfaceVariant)),
            SizedBox(height: 12),
            if (usingLkg) ...[
              Card(child: Padding(padding: EdgeInsets.all(12), child: const Text('云端当前不可用，正在使用 Last-Known-Good UI 配置；连接恢复后会自动同步。'))),
              SizedBox(height: 12),
            ],
            Card(
              child: Padding(
                padding: EdgeInsets.all(12),
                child: Wrap(
                  spacing: 12,
                  runSpacing: 8,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    DropdownButton<UIProfileScopeKind>(
                      value: _scope,
                      items: UIProfileScopeKind.values.map((value) => DropdownMenuItem(value: value, child: Text(_scopeLabel(value)))).toList(),
                      onChanged: _scopeLoading ? null : _changeScope,
                    ),
                    Text('revision ${envelope?.scopeProfile.revision ?? 0}', style: TextStyle(color: scheme.onSurfaceVariant, fontSize: 12)),
                    if (ctx != null) Text('${ctx.runtimeProfile ?? ''} · ${ctx.platform ?? currentUIPlatform()}${(ctx.deviceId ?? '').isNotEmpty ? ' · ${ctx.deviceId}' : ''}', style: TextStyle(color: scheme.onSurfaceVariant, fontSize: 12)),
                    if (_scope != UIProfileScopeKind.global && envelope?.scopeExists == true)
                      TextButton(onPressed: _scopeLoading ? null : _resetScope, child: const Text('清除此层覆盖')),
                  ],
                ),
              ),
            ),
            SizedBox(height: 20),
            if (runtime.isLoading && snapshot == null)
              const Center(child: Padding(padding: EdgeInsets.all(80), child: CircularProgressIndicator(strokeWidth: 2)))
            else if (runtime.hasError && snapshot == null)
              Text('加载 UI Provider 失败：${runtime.error}')
            else if (snapshot != null)
              for (final group in _groups.entries) ...[
                Text(group.key, style: Theme.of(context).textTheme.titleMedium),
                SizedBox(height: 8),
                Card(
                  margin: EdgeInsets.zero,
                  child: Column(children: [
                    for (var i = 0; i < group.value.length; i++) ...[
                      _ProviderRow(
                        capability: group.value[i], snapshot: snapshot, scopeProfile: envelope?.scopeProfile,
                        saving: _savingCapability == group.value[i], onChanged: (value) => _choose(group.value[i], value),
                      ),
                      if (i != group.value.length - 1) const Divider(height: 1),
                    ],
                  ]),
                ),
                SizedBox(height: 20),
              ],
          ],
        ),
      ),
    );
  }
}

class _ProviderRow extends StatelessWidget {
  const _ProviderRow({required this.capability, required this.snapshot, required this.scopeProfile, required this.saving, required this.onChanged});
  final String capability;
  final UIProviderSnapshot snapshot;
  final UIProfile? scopeProfile;
  final bool saving;
  final ValueChanged<String?> onChanged;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final candidates = snapshot.providers.where((p) => p.capability == capability).toList()..sort((a, b) => b.priority.compareTo(a.priority));
    final registryCapability = capability == UICapability.routeRegistry;
    final requested = scopeProfile?.selections[capability] ?? '';
    final explicit = candidates.any((provider) => provider.providerId == requested) ? requested : '';
    final resolved = snapshot.resolved[capability];
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      child: Row(children: [
        Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(capability, style: const TextStyle(fontWeight: FontWeight.w600)),
          const SizedBox(height: 2),
          Text(
            registryCapability
                ? '自动组合所有启用且兼容的 Route Registry；冲突按 priority 处理'
                : '有效：${resolved?.providerId ?? '无可用 Provider'}${explicit.isEmpty ? ' · 当前层继承' : ''}',
            style: TextStyle(fontSize: 12, color: scheme.onSurfaceVariant),
          ),
        ])),
        const SizedBox(width: 12),
        if (registryCapability)
          Chip(label: Text('自动组合 ${candidates.where((p) => p.enabled && !p.builtin).length}'))
        else if (saving)
          const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2))
        else
          DropdownButton<String>(
            value: explicit,
            items: [
              const DropdownMenuItem(value: '', child: Text('此层继承')),
              ...candidates.map((p) => DropdownMenuItem(
                value: p.providerId,
                enabled: p.enabled,
                child: Text('${p.providerId}${p.builtin ? ' · Built-in' : ''}${p.placement != UIProviderPlacement.any ? ' · ${p.placement.name}' : ''}', overflow: TextOverflow.ellipsis),
              )),
            ],
            onChanged: (value) => onChanged(value?.isEmpty == true ? null : value),
          ),
      ]),
    );
  }
}
