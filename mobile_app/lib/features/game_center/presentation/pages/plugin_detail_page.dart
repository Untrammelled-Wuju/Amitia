import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/amitia_page_route.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/ui_runtime/mobile_extension_slot.dart';
import '../../../../core/ui_runtime/ui_runtime_controller.dart';
import '../../domain/game_center_dto.dart';
import '../../domain/game_runtime_operations.dart';
import '../../data/game_package_picker.dart';
import '../controllers/game_center_providers.dart';
import '../controllers/game_center_controller.dart';
import 'runtime_detail_page.dart';
import '../widgets/game_package_confirmation.dart';

class PluginDetailPage extends ConsumerStatefulWidget {
  final String pluginId;
  final String extensionId;

  const PluginDetailPage({
    super.key,
    required this.pluginId,
    required this.extensionId,
  });

  @override
  ConsumerState<PluginDetailPage> createState() => _PluginDetailPageState();
}

class _PluginDetailPageState extends ConsumerState<PluginDetailPage> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() => _loadPlugin());
  }

  @override
  void didUpdateWidget(covariant PluginDetailPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.pluginId != widget.pluginId ||
        oldWidget.extensionId != widget.extensionId) {
      Future.microtask(() => _loadPlugin());
    }
  }

  Future<void> _loadPlugin({bool refreshUi = false}) async {
    if (widget.pluginId.trim().isEmpty || widget.extensionId.trim().isEmpty) {
      return;
    }
    await Future.wait([
      ref
          .read(gameCenterControllerProvider.notifier)
          .selectPlugin(widget.pluginId, extensionId: widget.extensionId),
      ref.read(uiRuntimeProvider.notifier).ensureLoaded(force: refreshUi),
    ]);
    if (!mounted) return;
    final controller = ref.read(gameCenterControllerProvider.notifier);
    final detail = ref.read(gameCenterControllerProvider).pluginDetail;
    if (detail == null ||
        detail.pluginId != widget.pluginId ||
        detail.extensionId != widget.extensionId) {
      return;
    }
    final selectedRuntimeId = detail.runtimes.isEmpty
        ? ''
        : detail.runtimes.first.runtimeId;
    for (final runtime in detail.runtimes) {
      final operations = availableGameRuntimeOperations(runtime.state);
      if (!operations.contains(GameRuntimeOperation.start)) continue;
      final started = await controller.startRuntime(runtime.runtimeId);
      if (!started) {
        final error = ref.read(gameCenterControllerProvider).runtimeError;
        throw StateError(error ?? '启动运行实例失败');
      }
    }
    await controller.selectPlugin(
      widget.pluginId,
      extensionId: widget.extensionId,
    );
    if (selectedRuntimeId.isEmpty || !mounted) return;
    final updated = await controller.updateGameMode(
      nextState: 'active',
      gameTitle: detail.name,
      runtimeId: selectedRuntimeId,
    );
    if (!updated && mounted) {
      final error =
          ref.read(gameCenterControllerProvider).gameModeError ??
          '更新游戏模式运行实例失败';
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(error)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final controller = ref.watch(gameCenterControllerProvider.notifier);
    final state = ref.watch(gameCenterControllerProvider);

    final detail = state.pluginDetail;
    final isCurrentDetail =
        detail?.pluginId == widget.pluginId &&
        detail?.extensionId == widget.extensionId;
    final visibleDetail = isCurrentDetail ? detail : null;
    final isLoading = state.pluginDetailLoading;
    final error = state.pluginDetailError;
    final hasOp = controller.hasPackageOp(widget.extensionId);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: visibleDetail?.name ?? '游戏控制台',
        navigation: AmitiaAppBarNavigation.back,
        fallbackRoute: AppRoutes.gameCenter,
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: hasOp ? null : () => _loadPlugin(refreshUi: true),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _buildBody(
          context,
          ref,
          visibleDetail,
          isLoading,
          error,
          hasOp,
          controller,
          state,
        ),
      ),
    );
  }

  Widget _buildBody(
    BuildContext context,
    WidgetRef ref,
    GamePluginDetail? detail,
    bool isLoading,
    String? error,
    bool hasOp,
    GameCenterController controller,
    GameCenterState state,
  ) {
    if (isLoading) {
      return const AmitiaLoadingState(message: '加载中...');
    }
    if (error != null) {
      return AmitiaErrorState(
        message: '加载失败: $error',
        onRetry: () => _loadPlugin(refreshUi: true),
      );
    }
    if (detail == null) {
      return const AmitiaEmptyState(
        icon: Icons.extension_outlined,
        title: '暂无插件信息',
      );
    }

    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        _buildGameSurface(context, detail),
        SizedBox(height: AppSpacing.lg),
        _buildInfoSection(context, detail),
        SizedBox(height: AppSpacing.lg),
        _buildActionsSection(context, ref, detail, hasOp, controller),
        SizedBox(height: AppSpacing.lg),
        _buildRuntimesSection(context, detail, state),
      ],
    );
  }

  Widget _buildGameSurface(BuildContext context, GamePluginDetail detail) {
    final slotContext = <String, dynamic>{
      'extensionId': detail.extensionId,
      'pluginId': detail.pluginId,
      'extension': <String, dynamic>{
        'id': detail.extensionId,
        'name': detail.name,
        'version': detail.version,
        'enabled': detail.enabled,
      },
      'gamePlugin': <String, dynamic>{
        'extensionId': detail.extensionId,
        'pluginId': detail.pluginId,
        'name': detail.name,
        'version': detail.version,
        'enabled': detail.enabled,
      },
      'managementTarget': 'game-center',
      'surface': 'game-detail',
      'surfaceRole': 'main',
      'slotFallback': 'default',
      'slotLayout': 'stack',
      'capabilities': detail.capabilities,
    };

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        MobileExtensionSlot(
          slotId: 'extension.detail.tab',
          extensionId: detail.extensionId,
          context: slotContext,
          fallback: _PluginBehaviorPanel(
            pluginId: detail.pluginId,
            runtimeId: detail.runtimes.isEmpty
                ? ''
                : detail.runtimes.first.runtimeId,
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        MobileExtensionSlot(
          slotId: 'extension.detail.action',
          extensionId: detail.extensionId,
          context: slotContext,
          fallback: const SizedBox.shrink(),
        ),
      ],
    );
  }

  Widget _buildInfoSection(BuildContext context, GamePluginDetail detail) {
    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('插件信息', style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.sm),
            _buildInfoRow(context, '名称', detail.name),
            _buildInfoRow(context, '版本', detail.version),
            _buildInfoRow(context, '状态', detail.enabled ? '已启用' : '已禁用'),
            _buildInfoRow(context, '安装状态', detail.installState),
            if (detail.description.isNotEmpty)
              _buildInfoRow(context, '描述', detail.description),
            if (detail.healthSummary != null)
              _buildInfoRow(context, '健康', detail.healthSummary!.status),
            _buildInfoRow(context, 'ExtensionID', detail.extensionId),
            _buildInfoRow(context, 'PluginID', detail.pluginId),
            if (detail.provider != null)
              _buildInfoRow(context, 'Provider', detail.provider!),
            if (detail.packageRevision != null)
              _buildInfoRow(
                context,
                'PackageRevision',
                detail.packageRevision!,
              ),
            if (detail.permissions.isNotEmpty) ...[
              SizedBox(height: AppSpacing.sm),
              Text('权限', style: AppTypography.caption(context)),
              const SizedBox(height: 4),
              Wrap(
                spacing: 4,
                runSpacing: 4,
                children: detail.permissions
                    .map(
                      (p) => AmitiaStatusBadge(label: p, type: BadgeType.info),
                    )
                    .toList(),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildActionsSection(
    BuildContext context,
    WidgetRef ref,
    GamePluginDetail detail,
    bool hasOp,
    GameCenterController controller,
  ) {
    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('插件操作', style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.sm),
            if (hasOp)
              const SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            else
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: [
                  if (!detail.enabled)
                    AmitiaButton(
                      label: '启用',
                      isSecondary: true,
                      height: 36,
                      onPressed: () => controller.enable(detail.extensionId),
                    ),
                  if (detail.enabled)
                    AmitiaButton(
                      label: '禁用',
                      isSecondary: true,
                      height: 36,
                      onPressed: () => controller.disable(detail.extensionId),
                    ),
                  AmitiaButton(
                    label: '更新',
                    isSecondary: true,
                    height: 36,
                    onPressed: () =>
                        _pickUpdate(context, ref, controller, detail),
                  ),
                  AmitiaButton(
                    label: '卸载',
                    isDestructive: true,
                    height: 36,
                    onPressed: () =>
                        _confirmUninstall(context, ref, controller, detail),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildRuntimesSection(
    BuildContext context,
    GamePluginDetail detail,
    GameCenterState state,
  ) {
    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '运行实例 (${detail.runtimes.length})',
              style: AppTypography.sectionTitle(context),
            ),
            SizedBox(height: AppSpacing.sm),
            if (detail.runtimes.isEmpty)
              Text('尚无运行实例', style: AppTypography.caption(context))
            else
              ...detail.runtimes.map(
                (rt) => InkWell(
                  onTap: () {
                    Navigator.push(
                      context,
                      amitiaSlideRoute(
                        builder: (_) => RuntimeDetailPage(
                          runtimeId: rt.runtimeId,
                          pluginId: rt.pluginId,
                        ),
                      ),
                    );
                  },
                  child: Padding(
                    padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
                    child: Row(
                      children: [
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                rt.runtimeId,
                                style: AppTypography.bodySmall(context),
                              ),
                              Text(
                                '状态: ${_stateLabel(rt.state)} | 控制: ${_modeLabel(rt.controlMode)}',
                                style: AppTypography.caption(context),
                              ),
                            ],
                          ),
                        ),
                        AmitiaStatusBadge(
                          label: rt.connected
                              ? (rt.ready ? 'Ready' : 'Connected')
                              : 'Disconnected',
                          type: rt.connected
                              ? (rt.ready ? BadgeType.success : BadgeType.info)
                              : BadgeType.neutral,
                        ),
                      ],
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildInfoRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 100,
            child: Text(label, style: AppTypography.caption(context)),
          ),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }

  String _stateLabel(String state) {
    switch (state.toLowerCase()) {
      case 'running':
        return '运行中';
      case 'stopped':
        return '已停止';
      case 'starting':
        return '启动中';
      case 'stopping':
        return '停止中';
      case 'failed':
        return '故障';
      default:
        return state;
    }
  }

  String _modeLabel(String mode) {
    switch (mode.toLowerCase()) {
      case 'user':
        return '用户';
      case 'plugin':
        return '插件';
      case 'observe':
        return '观察';
      case 'assist':
        return '辅助';
      case 'shared':
        return '共享';
      case 'suspended':
        return '暂停';
      default:
        return mode;
    }
  }

  Future<void> _pickUpdate(
    BuildContext context,
    WidgetRef ref,
    GameCenterController controller,
    GamePluginDetail detail,
  ) async {
    final lifecycle = ref.read(gameCenterPackageLifecycleProvider);
    try {
      final path = await pickGamePackagePath();
      if (path == null) return;
      final preview = await lifecycle.previewPackage(
        path,
        expectedExtensionId: detail.extensionId,
      );
      if (!context.mounted) return;
      final confirmed = await showGamePackagePreviewConfirmation(
        context,
        preview,
        actionLabel: '更新',
      );
      if (!confirmed) return;
      final ok = await controller.runPackageOperation(
        detail.extensionId,
        () => lifecycle.commitPackage(
          preview,
          expectedExtensionId: detail.extensionId,
        ),
      );
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(ok ? '插件更新完成' : '插件更新失败')));
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('插件更新失败: $e')));
    }
  }

  Future<void> _confirmUninstall(
    BuildContext context,
    WidgetRef ref,
    GameCenterController controller,
    GamePluginDetail detail,
  ) async {
    final lifecycle = ref.read(gameCenterPackageLifecycleProvider);
    try {
      final preview = await lifecycle.previewUninstall(detail.extensionId);
      if (!context.mounted) return;
      final confirmed = await showGamePackageUninstallConfirmation(
        context,
        preview,
        displayName: detail.name,
      );
      if (!confirmed) return;
      final ok = await controller.runPackageOperation(
        detail.extensionId,
        () => lifecycle.commitUninstall(detail.extensionId, preview),
        clearSelectionAfterSuccess: true,
      );
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(ok ? '插件卸载完成' : '插件卸载失败')));
      if (ok && context.mounted) Navigator.of(context).maybePop();
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('插件卸载失败: $e')));
    }
  }
}

class _PluginBehaviorPanel extends ConsumerStatefulWidget {
  final String pluginId;
  final String runtimeId;

  const _PluginBehaviorPanel({required this.pluginId, required this.runtimeId});

  @override
  ConsumerState<_PluginBehaviorPanel> createState() =>
      _PluginBehaviorPanelState();
}

class _PluginBehaviorPanelState extends ConsumerState<_PluginBehaviorPanel> {
  bool _loading = true;
  String? _error;
  String _catalogRevision = '';
  List<Map<String, dynamic>> _behaviors = const [];

  @override
  void initState() {
    super.initState();
    Future.microtask(_load);
  }

  @override
  void didUpdateWidget(covariant _PluginBehaviorPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.pluginId != widget.pluginId) {
      Future.microtask(_load);
    }
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await ref
          .read(gameCenterApiProvider)
          .listBehaviors(widget.pluginId);
      final rows = (data['behaviors'] as List? ?? const [])
          .whereType<Map>()
          .map((row) => row.cast<String, dynamic>())
          .toList(growable: false);
      if (!mounted) return;
      setState(() {
        _catalogRevision = (data['catalogRevision'] ?? '').toString();
        _behaviors = rows;
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error.toString();
        _loading = false;
      });
    }
  }

  Future<Map<String, dynamic>> _invoke(
    String behaviorId,
    Map<String, dynamic> arguments,
  ) async {
    if (widget.runtimeId.isEmpty) {
      throw StateError('游戏插件没有可用运行实例');
    }
    return ref
        .read(gameCenterApiProvider)
        .invokeBehavior(
          widget.pluginId,
          behaviorId,
          runtimeId: widget.runtimeId,
          expectedCatalogRevision: _catalogRevision,
          arguments: arguments,
        );
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return Card(
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.lg),
          child: const Center(child: CircularProgressIndicator()),
        ),
      );
    }
    if (_error != null) {
      return Card(
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.md),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('游戏行为加载失败', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.sm),
              SelectableText(_error!, style: AppTypography.bodySmall(context)),
              Align(
                alignment: Alignment.centerRight,
                child: TextButton(onPressed: _load, child: const Text('重试')),
              ),
            ],
          ),
        ),
      );
    }
    if (_behaviors.isEmpty) {
      return Card(
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.lg),
          child: Column(
            children: [
              Icon(Icons.sports_esports_outlined, size: 36),
              SizedBox(height: AppSpacing.sm),
              Text('该游戏扩展暂未提供专属控制界面', style: AppTypography.body(context)),
            ],
          ),
        ),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text('游戏行为', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        for (final behavior in _behaviors) ...[
          _GameBehaviorCard(
            key: ValueKey(behavior['id']),
            behavior: behavior,
            onInvoke: _invoke,
          ),
          SizedBox(height: AppSpacing.sm),
        ],
      ],
    );
  }
}

class _GameBehaviorCard extends StatefulWidget {
  final Map<String, dynamic> behavior;
  final Future<Map<String, dynamic>> Function(
    String behaviorId,
    Map<String, dynamic> arguments,
  )
  onInvoke;

  const _GameBehaviorCard({
    super.key,
    required this.behavior,
    required this.onInvoke,
  });

  @override
  State<_GameBehaviorCard> createState() => _GameBehaviorCardState();
}

class _GameBehaviorCardState extends State<_GameBehaviorCard> {
  final Map<String, dynamic> _values = {};
  bool _loading = false;
  String? _result;
  String? _error;

  Map<String, dynamic> get _schema =>
      ((widget.behavior['inputSchema'] as Map?) ?? const {})
          .cast<String, dynamic>();

  Map<String, dynamic> get _properties =>
      ((_schema['properties'] as Map?) ?? const {}).cast<String, dynamic>();

  @override
  void initState() {
    super.initState();
    for (final entry in _properties.entries) {
      final property = (entry.value as Map?)?.cast<String, dynamic>();
      final options = property?['enum'];
      if (options is List && options.isNotEmpty) {
        _values[entry.key] = options.first;
      }
    }
  }

  Future<void> _run() async {
    setState(() {
      _loading = true;
      _error = null;
      _result = null;
    });
    try {
      final result = await widget.onInvoke(
        (widget.behavior['id'] ?? '').toString(),
        _values,
      );
      if (!mounted) return;
      setState(() {
        _result = const JsonEncoder.withIndent('  ').convert(result);
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error.toString();
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final title = (widget.behavior['name'] ?? widget.behavior['id'] ?? '游戏行为')
        .toString();
    final description = (widget.behavior['description'] ?? '').toString();
    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(title, style: AppTypography.cardTitle(context)),
            if (description.isNotEmpty) ...[
              SizedBox(height: AppSpacing.xs),
              Text(description, style: AppTypography.bodySmall(context)),
            ],
            for (final entry in _properties.entries) ...[
              SizedBox(height: AppSpacing.sm),
              _buildInput(context, entry.key, entry.value),
            ],
            SizedBox(height: AppSpacing.md),
            AmitiaButton(
              label: _loading ? '执行中...' : '执行',
              isFullWidth: true,
              onPressed: _loading ? null : _run,
            ),
            if (_error != null) ...[
              SizedBox(height: AppSpacing.sm),
              SelectableText(
                _error!,
                style: AppTypography.bodySmall(
                  context,
                ).copyWith(color: Theme.of(context).colorScheme.error),
              ),
            ],
            if (_result != null) ...[
              SizedBox(height: AppSpacing.sm),
              SelectableText(_result!, style: AppTypography.bodySmall(context)),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildInput(BuildContext context, String key, dynamic rawProperty) {
    final property = (rawProperty as Map?)?.cast<String, dynamic>() ?? const {};
    final label = (property['title'] ?? key).toString();
    final options = property['enum'];
    if (options is List && options.isNotEmpty) {
      return DropdownButtonFormField<dynamic>(
        initialValue: _values[key],
        decoration: InputDecoration(labelText: label),
        items: options
            .map(
              (value) => DropdownMenuItem<dynamic>(
                value: value,
                child: Text(value.toString()),
              ),
            )
            .toList(growable: false),
        onChanged: (value) => setState(() => _values[key] = value),
      );
    }
    if (property['type'] == 'boolean') {
      return SwitchListTile(
        contentPadding: EdgeInsets.zero,
        title: Text(label),
        value: _values[key] == true,
        onChanged: (value) => setState(() => _values[key] = value),
      );
    }
    return TextFormField(
      decoration: InputDecoration(labelText: label),
      keyboardType:
          property['type'] == 'integer' || property['type'] == 'number'
          ? const TextInputType.numberWithOptions(decimal: true)
          : TextInputType.text,
      onChanged: (value) {
        if (value.trim().isEmpty) {
          _values.remove(key);
          return;
        }
        if (property['type'] == 'integer') {
          _values[key] = int.tryParse(value);
        } else if (property['type'] == 'number') {
          _values[key] = num.tryParse(value);
        } else {
          _values[key] = value;
        }
      },
    );
  }
}
