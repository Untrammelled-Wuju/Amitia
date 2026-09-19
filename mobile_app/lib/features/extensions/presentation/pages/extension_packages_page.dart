import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:amitia_app/core/widgets/amitia_popup_menu.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/artifact/artifact_providers.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/services/error_utils.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/ui_runtime/ui_runtime_invalidation.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ExtensionPackagesPage extends ConsumerStatefulWidget {
  const ExtensionPackagesPage({super.key});

  @override
  ConsumerState<ExtensionPackagesPage> createState() =>
      _ExtensionPackagesPageState();
}

class _ExtensionPackagesPageState extends ConsumerState<ExtensionPackagesPage> {
  List<Map<String, dynamic>> _packages = const [];
  bool _loading = true;
  bool _busy = false;
  bool _searchVisible = false;
  String? _error;
  String _query = '';
  final _searchController = TextEditingController();

  @override
  void initState() {
    super.initState();
    _loadPackages();
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  List<Map<String, dynamic>> get _searchResults {
    final query = _query.trim().toLowerCase();
    if (query.isEmpty) return const [];
    return _packages.where((pkg) {
      final searchable = [
        pkg['name'],
        pkg['extensionId'],
        pkg['version'],
        pkg['state'],
        pkg['enablement'],
      ].map((value) => (value ?? '').toString().toLowerCase());
      return searchable.any((value) => value.contains(query));
    }).toList(growable: false);
  }

  Future<void> _loadPackages() async {
    if (mounted)
      setState(() {
        _loading = true;
        _error = null;
      });
    try {
      final data = await ref.read(extensionServiceProvider).kernelExtensions();
      if (mounted)
        setState(() {
          _packages = data;
          _loading = false;
        });
    } catch (e) {
      if (mounted)
        setState(() {
          _error = safeErrorMessage(e);
          _loading = false;
        });
    }
  }

  Future<Dio> _dio() async {
    final availability = await ref.read(backendConnectionProvider.future);
    if (availability is! BackendConnectionAvailable)
      throw StateError('后端当前不可用');
    return createAuthenticatedDio(availability.config);
  }

  void _toast(String message, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor: error ? context.error : null,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: _searchVisible ? '搜索扩展包' : '扩展包',
        showBackButton: true,
        fallbackRoute: AppRoutes.extensions,
        actions: _searchVisible
            ? [
                AmitiaIconButton(
                  icon: Icons.close,
                  tooltip: '退出搜索',
                  onPressed: () => _toggleSearch(context),
                ),
              ]
            : [
                AmitiaIconButton(
                  icon: Icons.refresh,
                  onPressed: _busy ? null : _loadPackages,
                  tooltip: '刷新',
                ),
                AmitiaIconButton(
                  icon: Icons.search,
                  onPressed: () => _toggleSearch(context),
                  tooltip: '搜索',
                ),
                AmitiaIconButton(
                  icon: Icons.download_outlined,
                  onPressed: _busy ? null : _showInstallLocalSheet,
                  tooltip: '安装本地包',
                ),
                _buildKernelMoreMenu(context),
              ],
      ),
      body: SafeArea(top: false, child: _body()),
    );
  }

  void _toggleSearch(BuildContext context) {
    FocusScope.of(context).unfocus();
    _searchController.clear();
    setState(() {
      _searchVisible = !_searchVisible;
      _query = '';
    });
  }

  Widget _buildKernelMoreMenu(BuildContext context) {
    return AmitiaPopupMenuButton<String>(
      tooltip: '更多',
      enabled: !_busy,
      icon: Icon(
        Icons.more_horiz_rounded,
        size: 22,
        color: context.textSecondary,
      ),
      onSelected: (route) => context.push(AppRoutes.kernelPage(route)),
      itemBuilder: (context) => const [
        PopupMenuItem(
          value: 'trusted-services',
          child: _PackageMenuLabel(
            icon: Icons.verified_user_outlined,
            label: '可信服务运行时',
          ),
        ),
        PopupMenuItem(
          value: 'wasm',
          child: _PackageMenuLabel(
            icon: Icons.memory_outlined,
            label: 'WASM 运行时',
          ),
        ),
        PopupMenuItem(
          value: 'hooks',
          child: _PackageMenuLabel(
            icon: Icons.account_tree_outlined,
            label: 'Hook 中心',
          ),
        ),
        PopupMenuItem(
          value: 'tasks',
          child: _PackageMenuLabel(
            icon: Icons.play_circle_outline,
            label: '任务运行时',
          ),
        ),
        PopupMenuItem(
          value: 'events',
          child: _PackageMenuLabel(
            icon: Icons.notifications_active_outlined,
            label: '事件中心',
          ),
        ),
        PopupMenuItem(
          value: 'schedules',
          child: _PackageMenuLabel(
            icon: Icons.schedule_outlined,
            label: '调度中心',
          ),
        ),
        PopupMenuItem(
          value: 'desktop',
          child: _PackageMenuLabel(
            icon: Icons.desktop_windows_outlined,
            label: '桌面贡献中心',
          ),
        ),
        PopupMenuItem(
          value: 'dev-console',
          child: _PackageMenuLabel(
            icon: Icons.terminal,
            label: '开发者诊断控制台',
          ),
        ),
        PopupMenuItem(
          value: 'migrations',
          child: _PackageMenuLabel(
            icon: Icons.merge_type_outlined,
            label: '迁移与灰度中心',
          ),
        ),
        PopupMenuItem(
          value: 'dev-mode',
          child: _PackageMenuLabel(
            icon: Icons.developer_mode_outlined,
            label: '开发模式中心',
          ),
        ),
      ],
    );
  }

  Widget _body() {
    if (_loading) return const AmitiaLoadingState(message: '加载已安装扩展...');
    if (_error != null)
      return AmitiaErrorState(message: '加载失败: $_error', onRetry: _loadPackages);
    if (_searchVisible) return _buildSearchView();
    if (_packages.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.inventory_2_outlined,
        title: '暂无扩展包',
        subtitle: '安装 .amitiax 扩展包后会显示在这里',
        actionText: '安装本地包',
        onAction: _showInstallLocalSheet,
      );
    }
    return _buildPackageList(_packages);
  }

  Widget _buildSearchView() {
    final query = _query.trim();
    final results = _searchResults;
    return Column(
      children: [
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.md,
            AppSpacing.pagePadding,
            AppSpacing.sm,
          ),
          child: AmitiaSearchField(
            hintText: '搜索扩展名称、扩展 ID、版本或状态',
            controller: _searchController,
            autofocus: true,
            onChanged: (value) => setState(() => _query = value),
          ),
        ),
        Expanded(
          child: query.isEmpty
              ? const AmitiaEmptyState(
                  icon: Icons.search,
                  title: '输入关键词',
                  subtitle: '在当前页面搜索扩展包',
                )
              : results.isEmpty
              ? const AmitiaEmptyState(
                  icon: Icons.search_off,
                  title: '未找到相关扩展包',
                  subtitle: '尝试更换关键词',
                )
              : _buildPackageList(results),
        ),
      ],
    );
  }

  Widget _buildPackageList(List<Map<String, dynamic>> packages) {
    return RefreshIndicator(
      onRefresh: _loadPackages,
      child: ListView.separated(
        padding: EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          AppSpacing.sm,
          AppSpacing.pagePadding,
          AppSpacing.xxxl,
        ),
        keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
        itemCount: packages.length,
        separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
        itemBuilder: (context, index) => _buildPackageCard(packages[index]),
      ),
    );
  }

  Widget _buildPackageCard(Map<String, dynamic> pkg) {
    final id = (pkg['extensionId'] ?? '').toString();
    final name = (pkg['name'] ?? '').toString();
    final title = name.isNotEmpty ? name : (id.isEmpty ? '未命名扩展' : id);
    final version = (pkg['version'] ?? '').toString();
    final state = (pkg['state'] ?? '').toString();
    final enablement = (pkg['enablement'] ?? '').toString();
    final enabled = enablement == 'enabled';
    final systemManaged = pkg['systemManaged'] == true;
    final statusText = enabled ? '已启用' : (state == 'paused' ? '已暂停' : '已停用');

    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(
                  Icons.extension_outlined,
                  size: 22,
                  color: context.accentPrimary,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      title,
                      style: AppTypography.cardTitle(context),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 4),
                    Text(
                      version.isEmpty ? '版本未知' : 'v$version',
                      style: AppTypography.caption(context),
                    ),
                  ],
                ),
              ),
              AmitiaPopupMenuButton<String>(
                tooltip: '更多操作',
                enabled: !_busy,
                padding: EdgeInsets.zero,
                icon: Icon(
                  Icons.more_horiz_rounded,
                  size: 22,
                  color: context.textSecondary,
                ),
                onSelected: (action) => _handlePackageAction(pkg, action),
                itemBuilder: (context) => [
                  PopupMenuItem(
                    value: 'permissions',
                    child: _PackageMenuLabel(
                      icon: Icons.admin_panel_settings_outlined,
                      label: '权限管理',
                    ),
                  ),
                  PopupMenuItem(
                    value: 'pause',
                    enabled: enabled,
                    child: _PackageMenuLabel(
                      icon: Icons.pause_circle_outline,
                      label: '暂停',
                    ),
                  ),
                  const PopupMenuItem(
                    value: 'update',
                    child: _PackageMenuLabel(
                      icon: Icons.system_update_alt,
                      label: '更新',
                    ),
                  ),
                  const PopupMenuItem(
                    value: 'rollback',
                    child: _PackageMenuLabel(icon: Icons.history, label: '回滚'),
                  ),
                  const PopupMenuItem(
                    value: 'diagnose',
                    child: _PackageMenuLabel(
                      icon: Icons.monitor_heart_outlined,
                      label: '诊断',
                    ),
                  ),
                  PopupMenuDivider(),
                  PopupMenuItem(
                    value: 'uninstall',
                    enabled: !systemManaged,
                    child: _PackageMenuLabel(
                      icon: Icons.delete_outline,
                      label: '卸载',
                      color: systemManaged ? null : context.error,
                    ),
                  ),
                ],
              ),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Expanded(
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: AmitiaStatusBadge(
                    label: statusText,
                    type: enabled
                        ? BadgeType.success
                        : (state == 'paused'
                              ? BadgeType.warning
                              : BadgeType.neutral),
                  ),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              _MiniButton(
                label: '详情',
                icon: Icons.info_outline,
                color: context.accentPrimary,
                onTap: () => _showExtensionDetail(pkg),
              ),
              SizedBox(width: 8),
              Semantics(
                label: enabled ? '停用扩展' : '启用扩展',
                child: Switch.adaptive(
                  value: enabled,
                  onChanged: _busy
                      ? null
                      : (value) => _toggleExtension(pkg, value),
                  materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _handlePackageAction(
    Map<String, dynamic> pkg,
    String action,
  ) async {
    switch (action) {
      case 'permissions':
        await _showPermissionManager(pkg);
        break;
      case 'pause':
        await _pauseExtension(pkg);
        break;
      case 'update':
        await _checkUpdate(pkg);
        break;
      case 'rollback':
        await _rollbackExtension(pkg);
        break;
      case 'diagnose':
        context.push(AppRoutes.kernelPage('dev-console'));
        break;
      case 'uninstall':
        await _showUninstallConfirm(pkg);
        break;
    }
  }

  Future<void> _showPermissionManager(Map<String, dynamic> pkg) async {
    if (_busy) return;
    final id = (pkg['extensionId'] ?? '').toString();
    if (id.isEmpty) return;
    setState(() => _busy = true);
    try {
      final detail = await ref
          .read(extensionServiceProvider)
          .kernelExtension(id);
      final permissions = ((detail['permissions'] as List?) ?? const [])
          .whereType<Map>()
          .map((item) => Map<String, dynamic>.from(item))
          .toList(growable: false);
      if (!mounted) return;
      if (permissions.isEmpty) {
        _toast('该扩展没有声明权限');
        return;
      }
      await showDialog<void>(
        context: context,
        builder: (dialogContext) {
          var updating = false;
          return StatefulBuilder(
            builder: (dialogContext, setDialogState) => AlertDialog(
              backgroundColor: dialogContext.surfacePrimary,
              shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
              title: Text(
                '权限管理',
                style: AppTypography.cardTitle(dialogContext),
              ),
              content: SizedBox(
                width: double.maxFinite,
                child: ListView.separated(
                  shrinkWrap: true,
                  itemCount: permissions.length,
                  separatorBuilder: (_, _) =>
                      Divider(height: 1, color: dialogContext.borderSecondary),
                  itemBuilder: (context, index) {
                    final permission = permissions[index];
                    final permissionName = (permission['name'] ?? '')
                        .toString();
                    final granted = permission['granted'] == true;
                    final reason = (permission['reason'] ?? '').toString();
                    return Padding(
                      padding: const EdgeInsets.symmetric(vertical: 8),
                      child: Row(
                        children: [
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  permissionName,
                                  style: AppTypography.bodySmall(dialogContext),
                                ),
                                if (reason.isNotEmpty) ...[
                                  const SizedBox(height: 3),
                                  Text(
                                    reason,
                                    style: AppTypography.caption(dialogContext),
                                  ),
                                ],
                              ],
                            ),
                          ),
                          const SizedBox(width: 12),
                          Switch.adaptive(
                            value: granted,
                            onChanged: updating
                                ? null
                                : (value) async {
                                    setDialogState(() => updating = true);
                                    try {
                                      await ref
                                          .read(extensionServiceProvider)
                                          .setKernelExtensionPermission(
                                            id,
                                            permissionName,
                                            value,
                                          );
                                      if (dialogContext.mounted) {
                                        setDialogState(() {
                                          permission['granted'] = value;
                                          updating = false;
                                        });
                                      }
                                    } catch (e) {
                                      if (dialogContext.mounted) {
                                        setDialogState(() => updating = false);
                                      }
                                      _toast(
                                        '权限更新失败: ${safeErrorMessage(e)}',
                                        error: true,
                                      );
                                    }
                                  },
                          ),
                        ],
                      ),
                    );
                  },
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(dialogContext),
                  child: const Text('关闭'),
                ),
              ],
            ),
          );
        },
      );
    } catch (e) {
      _toast('读取权限失败: ${safeErrorMessage(e)}', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _pauseExtension(Map<String, dynamic> pkg) async {
    if (_busy) return;
    final id = (pkg['extensionId'] ?? '').toString();
    if (id.isEmpty) return;
    setState(() => _busy = true);
    try {
      await ref.read(extensionServiceProvider).pauseKernelExtension(id);
      await _loadPackages();
      _toast('$id 已暂停');
    } catch (e) {
      _toast('暂停失败: ${safeErrorMessage(e)}', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _rollbackExtension(Map<String, dynamic> pkg) async {
    if (_busy) return;
    final id = (pkg['extensionId'] ?? '').toString();
    if (id.isEmpty) return;
    final currentVersion = (pkg['version'] ?? '').toString();
    final controller = TextEditingController();
    final targetVersion = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: dialogContext.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('回滚扩展', style: AppTypography.cardTitle(dialogContext)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              currentVersion.isEmpty ? id : '$id · v$currentVersion',
              style: AppTypography.bodySmall(dialogContext),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: controller,
              autofocus: true,
              decoration: const InputDecoration(
                labelText: '目标版本',
                hintText: '例如 1.0.0',
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () =>
                Navigator.pop(dialogContext, controller.text.trim()),
            child: const Text('开始回滚'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (targetVersion == null || targetVersion.isEmpty) return;
    setState(() => _busy = true);
    try {
      final result = await ref
          .read(extensionServiceProvider)
          .rollbackKernelExtension(id, targetVersion);
      await _loadPackages();
      final operationId = (result['operationId'] ?? '').toString();
      _toast(
        operationId.isEmpty
            ? '$id 已回滚到 v$targetVersion'
            : '$id 回滚操作已提交 · $operationId',
      );
    } catch (e) {
      _toast('回滚失败: ${safeErrorMessage(e)}', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _checkUpdate(Map<String, dynamic> pkg) async {
    if (_busy) return;
    final extensionId = (pkg['extensionId'] ?? '').toString();
    if (extensionId.isEmpty) return;
    setState(() => _busy = true);
    try {
      final result = await ref
          .read(extensionServiceProvider)
          .checkKernelExtensionUpdate(extensionId);
      final rawItems = result['items'];
      final items = rawItems is List
          ? rawItems
                .whereType<Map>()
                .map((item) => item.cast<String, dynamic>())
                .toList(growable: false)
          : const <Map<String, dynamic>>[];
      if (!mounted) return;
      if (items.isEmpty) {
        _toast('当前已是最新版本');
        return;
      }
      await showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        backgroundColor: context.surfacePrimary,
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(top: Radius.circular(22)),
        ),
        builder: (sheetContext) => SafeArea(
          child: Padding(
            padding: EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  '可用更新 · $extensionId',
                  style: AppTypography.sectionTitle(context),
                ),
                const SizedBox(height: 6),
                Text(
                  '当前版本：${pkg['version'] ?? 'unknown'}',
                  style: AppTypography.caption(context),
                ),
                SizedBox(height: AppSpacing.md),
                ...items.map((item) {
                  final version = (item['version'] ?? '').toString();
                  final size = (item['packageSize'] as num?)?.toInt() ?? 0;
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: AmitiaCard(
                      child: Row(
                        children: [
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  version.isEmpty ? '未知版本' : 'v$version',
                                  style: AppTypography.bodySmall(
                                    context,
                                  ).copyWith(fontWeight: FontWeight.w600),
                                ),
                                const SizedBox(height: 3),
                                Text(
                                  '${item['releaseChannel'] ?? 'stable'}${size > 0 ? ' · ${(size / 1024 / 1024).toStringAsFixed(1)} MB' : ''}',
                                  style: AppTypography.caption(context),
                                ),
                              ],
                            ),
                          ),
                          AmitiaButton(
                            label: '下载',
                            icon: Icons.download_outlined,
                            height: 36,
                            onPressed: version.isEmpty
                                ? null
                                : () async {
                                    Navigator.of(sheetContext).pop();
                                    await _downloadUpdate(extensionId, version);
                                  },
                          ),
                        ],
                      ),
                    ),
                  );
                }),
              ],
            ),
          ),
        ),
      );
    } catch (e) {
      _toast('检查更新失败: ${safeErrorMessage(e)}', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _downloadUpdate(String extensionId, String version) async {
    if (mounted) setState(() => _busy = true);
    try {
      final result = await ref
          .read(extensionServiceProvider)
          .downloadKernelExtensionUpdate(extensionId, version);
      final operationId = (result['operationId'] ?? '').toString();
      if (operationId.isEmpty) throw StateError('后端未返回更新操作 ID');
      _toast('更新包下载任务已创建');
      if (!mounted) return;
      await _showUpdateOperation(extensionId, operationId);
    } catch (e) {
      _toast('下载更新失败: ${safeErrorMessage(e)}', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _showUpdateOperation(
    String extensionId,
    String operationId,
  ) async {
    Map<String, dynamic> operation = const <String, dynamic>{};
    List<Map<String, dynamic>> steps = const <Map<String, dynamic>>[];
    String? localError;
    bool localBusy = false;

    Future<void> refresh(StateSetter setDialogState) async {
      setDialogState(() {
        localBusy = true;
        localError = null;
      });
      try {
        final service = ref.read(extensionServiceProvider);
        final values = await Future.wait<dynamic>([
          service.kernelUpdateOperation(operationId),
          service.kernelUpdateOperationSteps(operationId),
        ]);
        setDialogState(() {
          operation = Map<String, dynamic>.from(values[0] as Map);
          steps = (values[1] as List)
              .whereType<Map>()
              .map((item) => item.cast<String, dynamic>())
              .toList(growable: false);
          localBusy = false;
        });
      } catch (e) {
        setDialogState(() {
          localError = safeErrorMessage(e);
          localBusy = false;
        });
      }
    }

    Future<void> runAction(StateSetter setDialogState, String action) async {
      setDialogState(() => localBusy = true);
      try {
        final service = ref.read(extensionServiceProvider);
        switch (action) {
          case 'install':
            await service.installKernelExtensionUpdate(
              extensionId,
              operationId,
            );
            break;
          case 'cancel':
            await service.cancelKernelExtensionUpdate(extensionId, operationId);
            break;
          case 'retry':
            await service.retryKernelExtensionUpdate(extensionId, operationId);
            break;
          case 'rollback':
            await service.rollbackKernelExtensionUpdate(
              extensionId,
              operationId,
            );
            break;
        }
        await refresh(setDialogState);
        await _loadPackages();
      } catch (e) {
        setDialogState(() {
          localError = safeErrorMessage(e);
          localBusy = false;
        });
      }
    }

    if (!mounted) return;
    await showDialog<void>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) {
          if (operation.isEmpty && !localBusy && localError == null) {
            WidgetsBinding.instance.addPostFrameCallback(
              (_) => refresh(setDialogState),
            );
          }
          final status = (operation['status'] ?? 'loading').toString();
          return AlertDialog(
            backgroundColor: dialogContext.surfacePrimary,
            shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
            title: Text('更新操作', style: AppTypography.cardTitle(dialogContext)),
            content: SizedBox(
              width: 520,
              child: SingleChildScrollView(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    _DetailRow(label: '扩展', value: extensionId),
                    _DetailRow(label: '操作 ID', value: operationId),
                    _DetailRow(label: '状态', value: status),
                    if ((operation['version'] ?? '').toString().isNotEmpty)
                      _DetailRow(
                        label: '版本',
                        value: operation['version'].toString(),
                      ),
                    if ((operation['error'] ?? '').toString().isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.only(top: 8),
                        child: Text(
                          operation['error'].toString(),
                          style: AppTypography.caption(
                            dialogContext,
                          ).copyWith(color: dialogContext.error),
                        ),
                      ),
                    if (localError != null)
                      Padding(
                        padding: const EdgeInsets.only(top: 8),
                        child: Text(
                          localError!,
                          style: AppTypography.caption(
                            dialogContext,
                          ).copyWith(color: dialogContext.error),
                        ),
                      ),
                    if (steps.isNotEmpty) ...[
                      SizedBox(height: AppSpacing.md),
                      Text(
                        '执行步骤',
                        style: AppTypography.bodySmall(
                          dialogContext,
                        ).copyWith(fontWeight: FontWeight.w600),
                      ),
                      const SizedBox(height: 6),
                      ...steps.map(
                        (step) => Padding(
                          padding: const EdgeInsets.only(bottom: 4),
                          child: Text(
                            '• ${step['name'] ?? step['stepId'] ?? '-'} · ${step['status'] ?? '-'}',
                            style: AppTypography.caption(dialogContext),
                          ),
                        ),
                      ),
                    ],
                    if (localBusy) ...[
                      SizedBox(height: AppSpacing.md),
                      const LinearProgressIndicator(),
                    ],
                  ],
                ),
              ),
            ),
            actions: [
              TextButton(
                onPressed: localBusy ? null : () => refresh(setDialogState),
                child: const Text('刷新'),
              ),
              TextButton(
                onPressed: localBusy
                    ? null
                    : () => runAction(setDialogState, 'cancel'),
                child: const Text('取消任务'),
              ),
              TextButton(
                onPressed: localBusy
                    ? null
                    : () => runAction(setDialogState, 'retry'),
                child: const Text('重试'),
              ),
              TextButton(
                onPressed: localBusy
                    ? null
                    : () => runAction(setDialogState, 'rollback'),
                child: const Text('回滚'),
              ),
              FilledButton(
                onPressed: localBusy
                    ? null
                    : () => runAction(setDialogState, 'install'),
                child: const Text('安装更新'),
              ),
              TextButton(
                onPressed: () => Navigator.pop(dialogContext),
                child: const Text('关闭'),
              ),
            ],
          );
        },
      ),
    );
  }

  Future<void> _toggleExtension(Map<String, dynamic> pkg, bool enabled) async {
    if (_busy) return;
    final id = (pkg['extensionId'] ?? '').toString();
    if (id.isEmpty) return;
    setState(() => _busy = true);
    try {
      await ref
          .read(extensionServiceProvider)
          .setKernelExtensionEnabled(id, enabled);
      await _loadPackages();
      _toast('$id 已${enabled ? '启用' : '停用'}');
    } catch (e) {
      _toast('操作失败: ${safeErrorMessage(e)}', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _showExtensionDetail(Map<String, dynamic> pkg) async {
    final id = (pkg['extensionId'] ?? '').toString();
    if (id.isEmpty) return;
    try {
      final detail = await ref
          .read(extensionServiceProvider)
          .kernelExtension(id);
      if (!mounted) return;
      showDialog(
        context: context,
        builder: (dialogContext) => _ExtensionDetailDialog(detail: detail),
      );
    } catch (e) {
      _toast('读取扩展详情失败: ${safeErrorMessage(e)}', error: true);
    }
  }

  Future<void> _showInstallLocalSheet() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['amitiax', 'zip'],
      withData: false,
    );
    if (picked == null || picked.files.isEmpty) return;
    final file = picked.files.first;
    if (file.path == null || file.path!.isEmpty) {
      _toast('无法读取所选文件', error: true);
      return;
    }

    Dio? dio;
    if (mounted) setState(() => _busy = true);
    try {
      dio = await _dio();
      final previewResponse = await dio.post(
        '/api/extensions/packages/artifacts',
        data: FormData.fromMap({
          'scopeType': 'global',
          'scopeId': '',
          'file': await MultipartFile.fromFile(file.path!, filename: file.name),
        }),
      );
      dynamic raw = previewResponse.data;
      if (raw is! Map || raw['preview'] is! Map) throw StateError('后端未返回扩展包预览');
      final preview = Map<String, dynamic>.from(raw['preview'] as Map);
      if (!mounted) return;

      final accepted = await showModalBottomSheet<bool>(
        context: context,
        isScrollControlled: true,
        backgroundColor: context.surfacePrimary,
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(top: Radius.circular(22)),
        ),
        builder: (sheetContext) =>
            _PackagePreviewSheet(fileName: file.name, preview: preview),
      );
      if (accepted != true) return;

      final sessionId = (preview['sessionId'] ?? '').toString();
      if (sessionId.isEmpty) throw StateError('预览会话无效');
      final confirmations = _buildInstallConfirmations(preview);
      final confirmResponse = await dio.post(
        '/api/extensions/packages/previews/${Uri.encodeComponent(sessionId)}/confirm',
        data: {
          'scopeType': (preview['scopeType'] ?? 'global').toString(),
          'scopeId': (preview['scopeId'] ?? '').toString(),
          'confirmations': confirmations,
        },
      );
      dynamic confirmed = confirmResponse.data;
      if (confirmed is Map && confirmed['data'] is Map)
        confirmed = confirmed['data'];
      if (confirmed is! Map) throw StateError('安装确认失败');
      final token = (confirmed['confirmationToken'] ?? '').toString();
      if (token.isEmpty) throw StateError('安装确认令牌缺失');

      final isUpdate = (preview['currentVersion'] ?? '').toString().isNotEmpty;
      final extensionId = (preview['id'] ?? '').toString();
      final operationResponse = await dio.post(
        isUpdate
            ? '/api/extensions/packages/operations/update'
            : '/api/extensions/packages/operations/install',
        data: {
          'sessionId': sessionId,
          'scopeType': (preview['scopeType'] ?? 'global').toString(),
          'scopeId': (preview['scopeId'] ?? '').toString(),
          'confirmationToken': token,
          if (isUpdate && extensionId.isNotEmpty)
            'expectedExtensionId': extensionId,
          'idempotencyKey':
              'mobile-package-${DateTime.now().microsecondsSinceEpoch}',
        },
      );
      dynamic operation = operationResponse.data;
      if (operation is Map && operation['data'] is Map)
        operation = operation['data'];
      final operationId = operation is Map
          ? (operation['operationId'] ?? '').toString()
          : '';
      UIRuntimeInvalidationBus.notifyChanged();
      await _loadPackages();
      _toast(operationId.isEmpty ? '扩展包操作已提交' : '扩展包操作已提交 · $operationId');
    } catch (e) {
      _toast('安装失败: ${safeErrorMessage(e)}', error: true);
    } finally {
      dio?.close(force: true);
      if (mounted) setState(() => _busy = false);
    }
  }

  Map<String, bool> _buildInstallConfirmations(Map<String, dynamic> preview) {
    final result = <String, bool>{};
    for (final value
        in (preview['capabilityConfirmations'] as List?) ?? const []) {
      final key = value.toString();
      if (key.isNotEmpty) result[key] = true;
    }
    final signature = preview['signature'];
    final signatureStatus = signature is Map
        ? (signature['status'] ?? '').toString()
        : '';
    if (signatureStatus == 'unsigned') result['confirm.unsigned_dev'] = true;
    final scriptCount = (preview['scripts'] as num?)?.toInt() ?? 0;
    if (scriptCount > 0) result['confirm.scripts'] = true;
    if ((preview['currentVersion'] ?? '').toString().isNotEmpty)
      result['confirm.version_change'] = true;
    if (((preview['highRiskCapabilities'] as List?) ?? const []).isNotEmpty)
      result['confirm.permission_escalation'] = true;
    if (preview['upgradeDiff'] is Map) {
      final diff = preview['upgradeDiff'] as Map;
      if (diff['signerChanged'] == true) result['confirm.signer_change'] = true;
      if (diff['configMigrationRequired'] == true)
        result['confirm.config_migration'] = true;
    }
    return result;
  }

  Future<void> _showUninstallConfirm(Map<String, dynamic> pkg) async {
    if (_busy) return;
    final id = (pkg['extensionId'] ?? '').toString();
    if (id.isEmpty) return;
    setState(() => _busy = true);
    try {
      final svc = ref.read(extensionServiceProvider);
      final preview = await svc.previewKernelUninstall(id);
      if (!mounted) return;
      final dependents = ((preview['dependents'] as List?) ?? const [])
          .map((e) => e.toString())
          .toList(growable: false);
      final required = ((preview['requiredConfirmations'] as List?) ?? const [])
          .map((e) => e.toString())
          .toList(growable: false);
      final allowed = preview['uninstallable'] != false;
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          backgroundColor: dialogContext.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
          title: Text('卸载扩展', style: AppTypography.cardTitle(dialogContext)),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('扩展：$id', style: AppTypography.bodySmall(dialogContext)),
              const SizedBox(height: 6),
              Text(
                '当前版本：${preview['currentVersion'] ?? pkg['version'] ?? ''}',
                style: AppTypography.label(dialogContext),
              ),
              Text(
                '制品策略：${preview['artifactPolicy'] ?? 'unknown'}',
                style: AppTypography.label(dialogContext),
              ),
              if (dependents.isNotEmpty) ...[
                const SizedBox(height: 8),
                Text(
                  '依赖此扩展：${dependents.join('、')}',
                  style: AppTypography.label(
                    dialogContext,
                  ).copyWith(color: dialogContext.warning),
                ),
              ],
              if (!allowed) ...[
                const SizedBox(height: 8),
                Text(
                  '后端判定当前不可卸载。',
                  style: AppTypography.label(
                    dialogContext,
                  ).copyWith(color: dialogContext.error),
                ),
              ],
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: allowed
                  ? () => Navigator.pop(dialogContext, true)
                  : null,
              child: const Text('确认卸载'),
            ),
          ],
        ),
      );
      if (confirmed != true) return;
      final confirmation = await svc.confirmKernelUninstall(id, {
        for (final key in required) key: true,
      });
      final token = (confirmation['confirmationToken'] ?? '').toString();
      if (token.isEmpty) throw StateError('卸载确认令牌缺失');
      final result = await svc.uninstallKernelExtension(id, token);
      await _loadPackages();
      final operationId = (result['operationId'] ?? '').toString();
      _toast(
        operationId.isEmpty ? '$id 卸载操作已提交' : '$id 卸载操作已提交 · $operationId',
      );
    } catch (e) {
      _toast('卸载失败: ${safeErrorMessage(e)}', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}

class _PackageMenuLabel extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color? color;

  const _PackageMenuLabel({
    required this.icon,
    required this.label,
    this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(icon, size: 19, color: color ?? context.textSecondary),
        const SizedBox(width: 10),
        Text(
          label,
          style: AppTypography.bodySmall(
            context,
          ).copyWith(color: color ?? context.textPrimary),
        ),
      ],
    );
  }
}

class _MiniButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color color;
  final VoidCallback onTap;

  const _MiniButton({
    required this.label,
    required this.icon,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
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
            Text(
              label,
              style: TextStyle(
                fontSize: 13,
                color: color,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _PackagePreviewSheet extends StatelessWidget {
  final String fileName;
  final Map<String, dynamic> preview;

  const _PackagePreviewSheet({required this.fileName, required this.preview});

  @override
  Widget build(BuildContext context) {
    final errors = ((preview['errors'] as List?) ?? const [])
        .map((e) => e.toString())
        .toList(growable: false);
    final warnings = ((preview['warnings'] as List?) ?? const [])
        .map((e) => e.toString())
        .toList(growable: false);
    final risks = ((preview['risks'] as List?) ?? const [])
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .toList(growable: false);
    final compatible = preview['compatible'] != false && errors.isEmpty;
    final currentVersion = (preview['currentVersion'] ?? '').toString();
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Center(
              child: Container(
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderPrimary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const SizedBox(height: 20),
            Text(
              currentVersion.isEmpty ? '安装扩展包' : '更新扩展包',
              style: AppTypography.pageTitle(context),
            ),
            const SizedBox(height: 14),
            _DetailRow(label: '文件', value: fileName),
            _DetailRow(label: '扩展 ID', value: (preview['id'] ?? '').toString()),
            _DetailRow(label: '名称', value: (preview['name'] ?? '').toString()),
            _DetailRow(
              label: '版本',
              value: (preview['version'] ?? '').toString(),
            ),
            if (currentVersion.isNotEmpty)
              _DetailRow(label: '当前版本', value: currentVersion),
            _DetailRow(
              label: '签名',
              value: preview['signature'] is Map
                  ? ((preview['signature'] as Map)['status'] ?? 'unknown')
                        .toString()
                  : 'unknown',
            ),
            _DetailRow(
              label: '兼容性',
              value:
                  (preview['compatibility'] ??
                          (compatible ? 'compatible' : 'blocked'))
                      .toString(),
            ),
            if (warnings.isNotEmpty) ...[
              const SizedBox(height: 10),
              Text('警告', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: 4),
              ...warnings
                  .take(5)
                  .map(
                    (item) =>
                        Text('• $item', style: AppTypography.label(context)),
                  ),
            ],
            if (risks.isNotEmpty) ...[
              const SizedBox(height: 10),
              Text('风险', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: 4),
              ...risks
                  .take(5)
                  .map(
                    (item) => Text(
                      '• ${item['message'] ?? item['code'] ?? item}',
                      style: AppTypography.label(
                        context,
                      ).copyWith(color: context.warning),
                    ),
                  ),
            ],
            if (errors.isNotEmpty) ...[
              const SizedBox(height: 10),
              ...errors
                  .take(5)
                  .map(
                    (item) => Text(
                      '• $item',
                      style: AppTypography.label(
                        context,
                      ).copyWith(color: context.error),
                    ),
                  ),
            ],
            const SizedBox(height: 18),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: () => Navigator.pop(context, false),
                    child: const Text('取消'),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: FilledButton(
                    onPressed: compatible
                        ? () => Navigator.pop(context, true)
                        : null,
                    child: Text(currentVersion.isEmpty ? '确认安装' : '确认更新'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _ExtensionDetailDialog extends StatelessWidget {
  final Map<String, dynamic> detail;

  const _ExtensionDetailDialog({required this.detail});

  @override
  Widget build(BuildContext context) {
    final modules = ((detail['modules'] as List?) ?? const [])
        .whereType<Map>()
        .toList(growable: false);
    final contributions = ((detail['contributions'] as List?) ?? const [])
        .whereType<Map>()
        .toList(growable: false);
    final name = (detail['name'] ?? '').toString();
    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      title: Text(
        name.isNotEmpty ? name : (detail['extensionId'] ?? '扩展详情').toString(),
        style: AppTypography.cardTitle(context),
      ),
      content: SizedBox(
        width: double.maxFinite,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _DetailRow(
                label: '版本',
                value: (detail['version'] ?? '').toString(),
              ),
              _DetailRow(
                label: '状态',
                value: (detail['state'] ?? '').toString(),
              ),
              _DetailRow(
                label: '启用状态',
                value: (detail['enablement'] ?? '').toString(),
              ),
              _DetailRow(
                label: '安装 ID',
                value: (detail['installationId'] ?? '').toString(),
              ),
              _DetailRow(
                label: 'Generation',
                value: (detail['generation'] ?? '').toString(),
              ),
              _DetailRow(label: '模块数量', value: modules.length.toString()),
              _DetailRow(label: '贡献数量', value: contributions.length.toString()),
              if (modules.isNotEmpty) ...[
                const SizedBox(height: 8),
                Text('模块', style: AppTypography.sectionTitle(context)),
                ...modules
                    .take(12)
                    .map(
                      (module) => Padding(
                        padding: const EdgeInsets.only(top: 5),
                        child: Text(
                          '${module['id'] ?? ''} · ${module['type'] ?? ''} · ${module['runtime'] ?? ''}',
                          style: AppTypography.label(context),
                        ),
                      ),
                    ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('关闭'),
        ),
      ],
    );
  }
}

class _DetailRow extends StatelessWidget {
  final String label;
  final String value;

  const _DetailRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 84,
            child: Text(label, style: AppTypography.label(context)),
          ),
          Expanded(
            child: Text(
              value.isEmpty ? '-' : value,
              style: AppTypography.bodySmall(context),
            ),
          ),
        ],
      ),
    );
  }
}
