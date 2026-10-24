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
import '../../data/pet_release_service.dart';

class PetInstallationsPage extends ConsumerStatefulWidget {
  const PetInstallationsPage({super.key, this.initialReleaseId = ''});

  final String initialReleaseId;

  @override
  ConsumerState<PetInstallationsPage> createState() =>
      _PetInstallationsPageState();
}

class _PetInstallationsPageState extends ConsumerState<PetInstallationsPage> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _installations = const [];
  List<PetRelease> _releases = const [];
  String _deviceId = '';
  String _operationNotice = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Map<String, String> get _deviceHeaders => {'X-Amitia-Device-ID': _deviceId};

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      _deviceId = await ref.read(uiRuntimeProvider.notifier).deviceId;
      final service = PetReleaseService(ref.read(backendServiceProvider));
      final results = await Future.wait<Object>([
        service.listInstallations(_deviceId),
        service.listReleases(),
      ]);
      if (!mounted) return;
      setState(() {
        _installations = results[0] as List<Map<String, dynamic>>;
        _releases = results[1] as List<PetRelease>;
        _loading = false;
      });
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '桌宠安装管理',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            onPressed: _load,
            color: context.textSecondary,
          ),
        ],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) {
      return AmitiaErrorState(message: _error!, onRetry: _load);
    }
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Text(
            '设备：$_deviceId',
            style: AppTypography.caption(
              context,
            ).copyWith(color: context.textTertiary),
          ),
          if (_operationNotice.isNotEmpty) ...[
            SizedBox(height: AppSpacing.sm),
            AmitiaCard(
              child: Row(
                children: [
                  Icon(Icons.sync, color: context.accentPrimary),
                  SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Text(
                      _operationNotice,
                      style: AppTypography.caption(context),
                    ),
                  ),
                ],
              ),
            ),
          ],
          SizedBox(height: AppSpacing.lg),
          const AmitiaSectionHeader(title: '已安装'),
          SizedBox(height: AppSpacing.sm),
          if (_installations.isEmpty)
            const SizedBox(
              height: 150,
              child: AmitiaEmptyState(
                icon: Icons.pets_outlined,
                title: '当前设备暂无已安装桌宠',
              ),
            )
          else
            ..._installations.map((item) => _installationCard(context, item)),
          SizedBox(height: AppSpacing.xl),
          const AmitiaSectionHeader(title: '可安装桌宠发布'),
          SizedBox(height: AppSpacing.sm),
          if (_releases.isEmpty)
            const SizedBox(
              height: 150,
              child: AmitiaEmptyState(
                icon: Icons.inventory_2_outlined,
                title: '暂无可安装桌宠发布',
                subtitle: '当前没有可安装的桌宠发布资源。',
              ),
            )
          else
            ..._releases.map((item) => _releaseCard(context, item)),
          SizedBox(height: AppSpacing.xxl),
        ],
      ),
    );
  }

  Widget _installationCard(BuildContext context, Map<String, dynamic> item) {
    final id = (item['id'] ?? '').toString();
    final name = (item['name'] ?? id).toString();
    final status = (item['status'] ?? '').toString();
    final runtime = item['runtime'] is Map
        ? Map<String, dynamic>.from(item['runtime'] as Map)
        : const <String, dynamic>{};
    final pending = runtime['pending'] == true;
    final enabled =
        runtime['desiredEnabled'] == true ||
        item['isActive'] == 1 ||
        status == 'enabled' ||
        status == 'running';
    final defaultAction = (item['defaultActionKey'] ?? '').toString();
    final desiredAction = (runtime['desiredActionKey'] ?? defaultAction)
        .toString();
    final actualAction = (runtime['actualActionKey'] ?? '').toString();
    final runtimeSync = (runtime['runtimeSyncState'] ?? '').toString();
    final actualHealth = (runtime['actualHealth'] ?? '').toString();
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.pets, color: context.accentPrimary),
                SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(name, style: AppTypography.cardTitle(context)),
                      Text(
                        '${item['packageVersion'] ?? ''} · ${pending ? '等待运行端确认' : status}',
                        style: AppTypography.caption(context),
                      ),
                    ],
                  ),
                ),
                Switch.adaptive(
                  value: enabled,
                  onChanged: (_) => _toggle(id, enabled),
                ),
              ],
            ),
            if (defaultAction.isNotEmpty) ...[
              SizedBox(height: AppSpacing.xs),
              Text('期望动作：$desiredAction', style: AppTypography.label(context)),
            ],
            if (actualAction.isNotEmpty) ...[
              SizedBox(height: AppSpacing.xs),
              Text('实际动作：$actualAction', style: AppTypography.label(context)),
            ],
            if (runtimeSync.isNotEmpty || actualHealth.isNotEmpty) ...[
              SizedBox(height: AppSpacing.xs),
              Text(
                '运行状态：${runtimeSync.isNotEmpty ? runtimeSync : actualHealth}',
                style: AppTypography.caption(context),
              ),
            ],
            SizedBox(height: AppSpacing.sm),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                OutlinedButton.icon(
                  onPressed: () => _recenter(id),
                  icon: const Icon(Icons.center_focus_strong, size: 16),
                  label: const Text('重置位置'),
                ),
                if (actualAction.isNotEmpty || desiredAction.isNotEmpty)
                  OutlinedButton.icon(
                    onPressed: () => _play(
                      id,
                      actualAction.isNotEmpty ? actualAction : desiredAction,
                    ),
                    icon: const Icon(Icons.play_arrow, size: 16),
                    label: const Text('播放默认动作'),
                  ),
                OutlinedButton.icon(
                  onPressed: () => _changeDefaultAction(
                    id,
                    actualAction.isNotEmpty ? actualAction : desiredAction,
                  ),
                  icon: const Icon(Icons.animation, size: 16),
                  label: const Text('默认动作'),
                ),
                OutlinedButton.icon(
                  onPressed: () => _changeVersion(item),
                  icon: const Icon(Icons.swap_horiz, size: 16),
                  label: const Text('版本'),
                ),
                OutlinedButton.icon(
                  onPressed: () => _repair(id),
                  icon: const Icon(Icons.build_outlined, size: 16),
                  label: const Text('修复'),
                ),
                OutlinedButton.icon(
                  onPressed: () => _uninstall(id, name),
                  icon: const Icon(Icons.delete_outline, size: 16),
                  label: const Text('卸载'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _releaseCard(BuildContext context, PetRelease item) {
    final selected = item.id == widget.initialReleaseId;
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Row(
          children: [
            Icon(
              selected ? Icons.new_releases : Icons.inventory_2_outlined,
              color: context.accentPrimary,
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '桌宠发布 v${item.version}',
                    style: AppTypography.body(context),
                  ),
                  Text(
                    '${item.actionCount} 个动作 · ${item.defaultActionKey}',
                    style: AppTypography.caption(context),
                  ),
                  if (selected)
                    Text(
                      '刚生成的发布',
                      style: AppTypography.caption(
                        context,
                      ).copyWith(color: context.accentPrimary),
                    ),
                ],
              ),
            ),
            FilledButton(
              onPressed: () => _install(item),
              child: const Text('安装'),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _toggle(String id, bool enabled) async {
    try {
      final service = PetReleaseService(ref.read(backendServiceProvider));
      final submitted = await service.setEnabled(_deviceId, id, !enabled);
      _setOperationNotice(enabled ? '正在停用桌宠' : '正在启用桌宠');
      final operation = await service.waitForOperation(_deviceId, submitted);
      _finishOperation(operation, enabled ? '停用' : '启用');
      await _load();
    } catch (e) {
      _setOperationNotice('');
      _toast('操作失败：$e');
    }
  }

  Future<void> _recenter(String id) async {
    try {
      final service = PetReleaseService(ref.read(backendServiceProvider));
      final submitted = await service.recenter(_deviceId, id);
      _setOperationNotice('正在重置桌宠位置');
      final operation = await service.waitForOperation(_deviceId, submitted);
      _finishOperation(operation, '位置重置');
    } catch (e) {
      _setOperationNotice('');
      _toast('重置失败：$e');
    }
  }

  Future<void> _play(String id, String actionKey) async {
    try {
      await ref
          .read(backendServiceProvider)
          .post<dynamic>(
            '/api/desktop-pets/installations/$id/actions/$actionKey/play',
            headers: _deviceHeaders,
          );
      _toast('动作已触发');
    } catch (e) {
      _toast('动作触发失败：$e');
    }
  }

  Future<void> _changeDefaultAction(String id, String current) async {
    final controller = TextEditingController(text: current);
    final action = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('设置默认动作'),
        content: TextField(
          controller: controller,
          decoration: const InputDecoration(hintText: '例如 idle_normal'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () =>
                Navigator.pop(dialogContext, controller.text.trim()),
            child: const Text('保存'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (action == null || action.isEmpty) return;
    try {
      final service = PetReleaseService(ref.read(backendServiceProvider));
      final submitted = await service.changeDefaultAction(
        _deviceId,
        id,
        action,
      );
      _setOperationNotice('正在更新默认动作');
      final operation = await service.waitForOperation(_deviceId, submitted);
      _finishOperation(operation, '默认动作更新');
      await _load();
    } catch (e) {
      _setOperationNotice('');
      _toast('设置默认动作失败：$e');
    }
  }

  Future<void> _uninstall(String id, String name) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('卸载桌宠'),
        content: Text('确认从当前设备卸载「$name」？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('卸载'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      final service = PetReleaseService(ref.read(backendServiceProvider));
      final submitted = await service.uninstall(_deviceId, id);
      _setOperationNotice('正在卸载桌宠');
      final operation = await service.waitForOperation(_deviceId, submitted);
      _finishOperation(operation, '卸载');
      await _load();
    } catch (e) {
      _setOperationNotice('');
      _toast('卸载失败：$e');
    }
  }

  Future<void> _install(PetRelease release) async {
    try {
      final service = PetReleaseService(ref.read(backendServiceProvider));
      final submitted = await service.install(_deviceId, release);
      _setOperationNotice('正在安装桌宠发布 v${release.version}');
      final operation = await service.waitForOperation(
        _deviceId,
        submitted,
        attempts: 40,
      );
      _finishOperation(operation, '安装');
      await _load();
    } catch (e) {
      _setOperationNotice('');
      _toast('安装失败：$e');
    }
  }

  Future<void> _changeVersion(Map<String, dynamic> installation) async {
    final installationId = (installation['id'] ?? '').toString();
    final currentReleaseId = (installation['currentReleaseId'] ?? '')
        .toString();
    final candidates = _releases
        .where((release) => release.id != currentReleaseId)
        .toList(growable: false);
    if (installationId.isEmpty || candidates.isEmpty) {
      _toast('暂无可切换的桌宠发布');
      return;
    }
    final selected = await showDialog<PetRelease>(
      context: context,
      builder: (dialogContext) => SimpleDialog(
        title: const Text('选择目标版本'),
        children: candidates
            .map(
              (release) => SimpleDialogOption(
                onPressed: () => Navigator.pop(dialogContext, release),
                child: Text('v${release.version} · ${release.actionCount} 个动作'),
              ),
            )
            .toList(growable: false),
      ),
    );
    if (selected == null || !mounted) return;
    PetRelease? current;
    for (final release in _releases) {
      if (release.id == currentReleaseId) {
        current = release;
        break;
      }
    }
    final direction = _compareReleaseVersions(
      selected.version,
      current?.version ?? '',
    );
    var safetyConfirm = false;
    if (direction < 0) {
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('确认降级'),
          content: Text(
            '将桌宠从 v${current?.version ?? '未知'} 降级到 v${selected.version}。是否继续？',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('取消'),
            ),
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, true),
              child: const Text('确认降级'),
            ),
          ],
        ),
      );
      if (confirmed != true) return;
      safetyConfirm = true;
    }
    try {
      final service = PetReleaseService(ref.read(backendServiceProvider));
      final submitted = direction > 0
          ? await service.upgrade(_deviceId, installationId, selected.id)
          : direction < 0
          ? await service.downgrade(
              _deviceId,
              installationId,
              selected.id,
              safetyConfirm: safetyConfirm,
            )
          : await service.switchRelease(_deviceId, installationId, selected.id);
      _setOperationNotice('正在切换到桌宠发布 v${selected.version}');
      final operation = await service.waitForOperation(_deviceId, submitted);
      _finishOperation(operation, '版本切换');
      await _load();
    } catch (e) {
      _setOperationNotice('');
      _toast('版本切换失败：$e');
    }
  }

  Future<void> _repair(String installationId) async {
    try {
      final service = PetReleaseService(ref.read(backendServiceProvider));
      final submitted = await service.repair(_deviceId, installationId);
      _setOperationNotice('正在修复桌宠安装');
      final operation = await service.waitForOperation(_deviceId, submitted);
      _finishOperation(operation, '修复');
      await _load();
    } catch (e) {
      _setOperationNotice('');
      _toast('修复失败：$e');
    }
  }

  void _setOperationNotice(String value) {
    if (mounted) setState(() => _operationNotice = value);
  }

  void _finishOperation(PetInstallationOperation operation, String action) {
    _setOperationNotice('');
    if (operation.failed) {
      _toast(
        '$action失败：${operation.errorMessage.isNotEmpty ? operation.errorMessage : operation.errorCode}',
      );
      return;
    }
    if (operation.status == 'completed') {
      _toast('$action完成');
      return;
    }
    _toast('$action已提交，等待桌宠运行时同步');
  }

  int _compareReleaseVersions(String left, String right) {
    final leftParts = left.split('.').map(int.tryParse).toList(growable: false);
    final rightParts = right
        .split('.')
        .map(int.tryParse)
        .toList(growable: false);
    final length = leftParts.length > rightParts.length
        ? leftParts.length
        : rightParts.length;
    for (var index = 0; index < length; index++) {
      final leftValue = index < leftParts.length ? leftParts[index] ?? 0 : 0;
      final rightValue = index < rightParts.length ? rightParts[index] ?? 0 : 0;
      if (leftValue != rightValue) return leftValue.compareTo(rightValue);
    }
    return 0;
  }

  void _toast(String message) {
    if (mounted) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(message)));
    }
  }
}
