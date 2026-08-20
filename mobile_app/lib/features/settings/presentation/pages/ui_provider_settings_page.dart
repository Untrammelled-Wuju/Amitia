import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../core/ui_runtime/ui_provider.dart';
import '../../../../core/ui_runtime/ui_runtime_controller.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class UIProviderSettingsPage extends ConsumerWidget {
  const UIProviderSettingsPage({super.key});

  static const _groups = <String, List<String>>{
    '应用外壳': [UICapability.appShell, UICapability.appNavigation, UICapability.appWorkspace, UICapability.routeRegistry, UICapability.pageProvider],
    '对话': [UICapability.conversationShell, UICapability.conversationHeader, UICapability.conversationMessages, UICapability.conversationMessageRenderer, UICapability.conversationSidebar, UICapability.conversationComposer, UICapability.conversationOverlay],
    '业务页面': [UICapability.characterShell, UICapability.characterDetail, UICapability.memoryShell, UICapability.memoryDetail, UICapability.settingsShell, UICapability.settingsSection, UICapability.extensionCenter, UICapability.extensionPage],
    '设计系统': [UICapability.theme, UICapability.tokens, UICapability.icons, UICapability.components],
  };

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final runtime = ref.watch(uiRuntimeProvider);
    final snapshot = runtime.valueOrNull;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '界面提供者', navigation: AmitiaAppBarNavigation.back),
      body: RefreshIndicator(
        onRefresh: () => ref.read(uiRuntimeProvider.notifier).ensureLoaded(force: true),
        child: runtime.when(
          loading: () => const ListView(children: [SizedBox(height: 240), Center(child: CircularProgressIndicator(strokeWidth: 2))]),
          error: (error, _) => ListView(padding: EdgeInsets.all(AppSpacing.pagePadding), children: [Text('加载 UI Provider 失败：$error')]),
          data: (_) => ListView(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            children: [
              Text('Amitia 默认界面同样作为 Built-in Provider 参与解析。登录、首次引导与恢复页属于安全壳，不允许扩展替换。', style: TextStyle(color: context.textSecondary)),
              SizedBox(height: AppSpacing.lg),
              if (snapshot != null)
                for (final group in _groups.entries) ...[
                  Text(group.key, style: Theme.of(context).textTheme.titleMedium),
                  SizedBox(height: AppSpacing.sm),
                  Card(
                    margin: EdgeInsets.zero,
                    child: Column(children: [
                      for (var i = 0; i < group.value.length; i++) ...[
                        _ProviderRow(capability: group.value[i], snapshot: snapshot),
                        if (i != group.value.length - 1) const Divider(height: 1),
                      ],
                    ]),
                  ),
                  SizedBox(height: AppSpacing.lg),
                ],
            ],
          ),
        ),
      ),
    );
  }
}

class _ProviderRow extends ConsumerWidget {
  const _ProviderRow({required this.capability, required this.snapshot});
  final String capability;
  final UIProviderSnapshot snapshot;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final candidates = snapshot.providers.where((p) => p.capability == capability).toList()
      ..sort((a, b) => b.priority.compareTo(a.priority));
    final requested = snapshot.profile.selections[capability] ?? '';
    final explicit = candidates.any((provider) => provider.providerId == requested) ? requested : '';
    final resolved = snapshot.resolved[capability];
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      child: Row(
        children: [
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text(capability, style: const TextStyle(fontWeight: FontWeight.w600)),
              const SizedBox(height: 2),
              Text('当前：${resolved?.providerId ?? '无可用 Provider'}', style: TextStyle(fontSize: 12, color: context.textTertiary)),
            ]),
          ),
          const SizedBox(width: 12),
          DropdownButton<String>(
            value: explicit,
            items: [
              const DropdownMenuItem(value: '', child: Text('自动（Built-in 优先）')),
              ...candidates.map((p) => DropdownMenuItem(value: p.providerId, enabled: p.enabled, child: Text('${p.providerId}${p.builtin ? ' · Built-in' : ''}', overflow: TextOverflow.ellipsis))),
            ],
            onChanged: (value) async {
              if (value == null) return;
              final selections = Map<String, String>.from(snapshot.profile.selections);
              if (value.isEmpty) selections.remove(capability); else selections[capability] = value;
              await ref.read(uiRuntimeProvider.notifier).updateProfile(UIProfile(
                profileId: snapshot.profile.profileId,
                name: snapshot.profile.name,
                selections: selections,
                updatedAt: DateTime.now().millisecondsSinceEpoch,
              ));
            },
          ),
        ],
      ),
    );
  }
}
