import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/ui_runtime/ui_runtime_controller.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class PetInstallationsPage extends ConsumerStatefulWidget {
  const PetInstallationsPage({super.key});

  @override
  ConsumerState<PetInstallationsPage> createState() => _PetInstallationsPageState();
}

class _PetInstallationsPageState extends ConsumerState<PetInstallationsPage> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _installations = const [];
  List<Map<String, dynamic>> _packages = const [];
  String _deviceId = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Map<String, String> get _deviceHeaders => {'X-Amitia-Device-ID': _deviceId};

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      _deviceId = await ref.read(uiRuntimeProvider.notifier).deviceId;
      final api = ref.read(backendServiceProvider);
      final results = await Future.wait<dynamic>([
        api.get<dynamic>('/api/desktop-pets/installations', headers: _deviceHeaders),
        api.get<dynamic>('/api/desktop-pets/packages', queryParameters: {'page': 1, 'pageSize': 100}),
      ]);
      if (!mounted) return;
      setState(() {
        _installations = _items(results[0]);
        _packages = _items(results[1]);
        _loading = false;
      });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  List<Map<String, dynamic>> _items(dynamic response) {
    dynamic source = response;
    if (response is Map) source = response['items'] ?? const [];
    if (source is! List) return const [];
    return source.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '桌宠安装管理',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
        actions: [AmitiaIconButton(icon: Icons.refresh, onPressed: _load, color: context.textSecondary)],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Text('设备：$_deviceId', style: AppTypography.caption(context).copyWith(color: context.textTertiary)),
          SizedBox(height: AppSpacing.lg),
          const AmitiaSectionHeader(title: '已安装'),
          SizedBox(height: AppSpacing.sm),
          if (_installations.isEmpty)
            const SizedBox(height: 150, child: AmitiaEmptyState(icon: Icons.pets_outlined, title: '当前设备暂无已安装桌宠'))
          else
            ..._installations.map((item) => _installationCard(context, item)),
          SizedBox(height: AppSpacing.xl),
          const AmitiaSectionHeader(title: '可安装资源包'),
          SizedBox(height: AppSpacing.sm),
          if (_packages.isEmpty)
            const SizedBox(height: 150, child: AmitiaEmptyState(icon: Icons.inventory_2_outlined, title: '暂无可安装资源包', subtitle: '先在处理任务中完成打包。'))
          else
            ..._packages.map((item) => _packageCard(context, item)),
          SizedBox(height: AppSpacing.xxl),
        ],
      ),
    );
  }

  Widget _installationCard(BuildContext context, Map<String, dynamic> item) {
    final id = (item['id'] ?? '').toString();
    final name = (item['name'] ?? id).toString();
    final status = (item['status'] ?? '').toString();
    final enabled = item['isActive'] == 1 || status == 'enabled' || status == 'running';
    final defaultAction = (item['defaultActionKey'] ?? '').toString();
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Icon(Icons.pets, color: context.accentPrimary),
            SizedBox(width: AppSpacing.sm),
            Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text(name, style: AppTypography.cardTitle(context)),
              Text('${item['packageVersion'] ?? ''} · $status', style: AppTypography.caption(context)),
            ])),
            Switch.adaptive(value: enabled, onChanged: (_) => _toggle(id, enabled)),
          ]),
          if (defaultAction.isNotEmpty) ...[
            SizedBox(height: AppSpacing.xs),
            Text('默认动作：$defaultAction', style: AppTypography.label(context)),
          ],
          SizedBox(height: AppSpacing.sm),
          Wrap(spacing: 8, runSpacing: 8, children: [
            OutlinedButton.icon(onPressed: () => _recenter(id), icon: const Icon(Icons.center_focus_strong, size: 16), label: const Text('重置位置')),
            if (defaultAction.isNotEmpty)
              OutlinedButton.icon(onPressed: () => _play(id, defaultAction), icon: const Icon(Icons.play_arrow, size: 16), label: const Text('播放默认动作')),
            OutlinedButton.icon(onPressed: () => _changeDefaultAction(id, defaultAction), icon: const Icon(Icons.animation, size: 16), label: const Text('默认动作')),
            OutlinedButton.icon(onPressed: () => _uninstall(id, name), icon: const Icon(Icons.delete_outline, size: 16), label: const Text('卸载')),
          ]),
        ]),
      ),
    );
  }

  Widget _packageCard(BuildContext context, Map<String, dynamic> item) {
    final id = (item['id'] ?? '').toString();
    final name = (item['name'] ?? id).toString();
    final characterId = (item['characterId'] ?? '').toString();
    final status = (item['status'] ?? '').toString();
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Row(children: [
          Icon(Icons.inventory_2_outlined, color: context.accentPrimary),
          SizedBox(width: AppSpacing.sm),
          Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(name, style: AppTypography.body(context)),
            Text('v${item['version'] ?? '-'} · ${item['actionCount'] ?? 0} 动作 · $status', style: AppTypography.caption(context)),
          ])),
          FilledButton(onPressed: id.isEmpty || characterId.isEmpty ? null : () => _install(id, characterId, name), child: const Text('安装')),
        ]),
      ),
    );
  }

  Future<void> _toggle(String id, bool enabled) async {
    try {
      await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/installations/$id/${enabled ? 'disable' : 'enable'}',
        headers: _deviceHeaders,
      );
      await _load();
    } catch (e) { _toast('操作失败：$e'); }
  }

  Future<void> _recenter(String id) async {
    try {
      await ref.read(backendServiceProvider).post<dynamic>('/api/desktop-pets/installations/$id/recenter', headers: _deviceHeaders);
      _toast('已提交位置重置');
    } catch (e) { _toast('重置失败：$e'); }
  }

  Future<void> _play(String id, String actionKey) async {
    try {
      await ref.read(backendServiceProvider).post<dynamic>('/api/desktop-pets/installations/$id/actions/$actionKey/play', headers: _deviceHeaders);
      _toast('动作已触发');
    } catch (e) { _toast('动作触发失败：$e'); }
  }

  Future<void> _changeDefaultAction(String id, String current) async {
    final controller = TextEditingController(text: current);
    final action = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('设置默认动作'),
        content: TextField(controller: controller, decoration: const InputDecoration(hintText: '例如 idle_normal')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, controller.text.trim()), child: const Text('保存')),
        ],
      ),
    );
    controller.dispose();
    if (action == null || action.isEmpty) return;
    try {
      await ref.read(backendServiceProvider).patch<dynamic>(
        '/api/desktop-pets/installations/$id/default-action',
        headers: _deviceHeaders,
        data: {'action_key': action},
      );
      await _load();
    } catch (e) { _toast('设置默认动作失败：$e'); }
  }

  Future<void> _uninstall(String id, String name) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('卸载桌宠'),
        content: Text('确认从当前设备卸载「$name」？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('卸载')),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(backendServiceProvider).delete('/api/desktop-pets/installations/$id', headers: _deviceHeaders);
      await _load();
    } catch (e) { _toast('卸载失败：$e'); }
  }

  Future<void> _install(String packageId, String characterId, String name) async {
    try {
      await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/packages/$packageId/install',
        headers: _deviceHeaders,
        data: {'character_id': characterId},
      );
      _toast('「$name」安装任务已提交');
      await _load();
    } catch (e) { _toast('安装失败：$e'); }
  }

  void _toast(String message) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }
}
