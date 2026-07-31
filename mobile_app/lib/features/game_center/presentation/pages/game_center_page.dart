import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/mock_data/mock_data.dart';

enum GameConnectionState { disconnected, connecting, connected, failed }

class GameCenterPage extends ConsumerStatefulWidget {
  const GameCenterPage({super.key});

  @override
  ConsumerState<GameCenterPage> createState() => _GameCenterPageState();
}

class _GameCenterPageState extends ConsumerState<GameCenterPage> {
  GameConnectionState _connectionState = GameConnectionState.disconnected;
  String? _connectedGame;
  String _connectionMethod = 'WebSocket';
  String _connectionAddress = '';
  Timer? _installTimer;
  String? _installingPlugin;
  double _installProgress = 0;

  final _availableGames = ['Minecraft', '星露谷物语', '原神', '幻兽帕鲁'];
  final _connectionMethods = ['WebSocket', 'HTTP', 'TCP'];

  @override
  void dispose() {
    _installTimer?.cancel();
    super.dispose();
  }

  List<String> get _installedPlugins => MockData.gamePlugins;

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '游戏中心',
        navigation: AmitiaAppBarNavigation.drawer,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.only(bottom: AppSpacing.xxl),
          children: [
            const SizedBox(height: AppSpacing.sm),
            _buildConnectionCard(context),
            const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '已安装游戏插件'),
            const SizedBox(height: AppSpacing.sm),
            _buildInstalledPluginsCard(context),
            const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '可用游戏插件'),
            const SizedBox(height: AppSpacing.sm),
            ..._buildAvailablePluginCards(context),
            const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '最近游戏任务'),
            const SizedBox(height: AppSpacing.sm),
            _buildTasksCard(context),
            const SizedBox(height: AppSpacing.sectionGap),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
              child: AmitiaButton(
                label: '插件管理',
                icon: Icons.extension_outlined,
                isFullWidth: true,
                isSecondary: true,
                onPressed: () => context.push(AppRoutes.extensions),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildConnectionCard(BuildContext context) {
    final IconData stateIcon;
    final Color stateColor;
    final String stateLabel;
    switch (_connectionState) {
      case GameConnectionState.disconnected:
        stateIcon = Icons.link_off;
        stateColor = context.warning;
        stateLabel = '未连接';
      case GameConnectionState.connecting:
        stateIcon = Icons.sync;
        stateColor = context.accentPrimary;
        stateLabel = '正在连接...';
      case GameConnectionState.connected:
        stateIcon = Icons.link;
        stateColor = context.success;
        stateLabel = _connectedGame != null ? '已连接 · $_connectedGame' : '已连接';
      case GameConnectionState.failed:
        stateIcon = Icons.error_outline;
        stateColor = context.error;
        stateLabel = '连接失败';
    }
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                color: stateColor.withValues(alpha: 0.12),
                borderRadius: AppRadius.brSmall,
              ),
              child: _connectionState == GameConnectionState.connecting
                  ? Padding(
                      padding: const EdgeInsets.all(12),
                      child: CircularProgressIndicator(strokeWidth: 2.5, color: stateColor),
                    )
                  : Icon(stateIcon, size: 24, color: stateColor),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('游戏连接', style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(stateLabel, style: AppTypography.caption(context)),
                ],
              ),
            ),
            AmitiaButton(
              label: _connectionState == GameConnectionState.connected ? '管理' : '连接',
              isSecondary: true,
              height: 36,
              onPressed: () => _showConnectionSheet(context),
            ),
          ],
        ),
      ),
    );
  }

  void _showConnectionSheet(BuildContext context) {
    String selectedGame = _connectedGame ?? _availableGames.first;
    String selectedMethod = _connectionMethod;
    final addressController = TextEditingController(text: _connectionAddress);
    GameConnectionState testState = _connectionState;
    bool isTesting = false;

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetCtx) {
        return StatefulBuilder(
          builder: (ctx, setSheetState) {
            return SafeArea(
              child: Padding(
                padding: EdgeInsets.fromLTRB(20, 0, 20, 34 + MediaQuery.of(ctx).viewInsets.bottom),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const SizedBox(height: 8),
                    Center(
                      child: Container(
                        width: 40,
                        height: 4,
                        decoration: BoxDecoration(
                          color: ctx.borderPrimary,
                          borderRadius: BorderRadius.circular(2),
                        ),
                      ),
                    ),
                    const SizedBox(height: 20),
                    Text('连接游戏', style: AppTypography.pageTitle(ctx)),
                    const SizedBox(height: 20),
                    Text('选择游戏', style: AppTypography.caption(ctx)),
                    const SizedBox(height: 6),
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12),
                      decoration: BoxDecoration(
                        color: ctx.surfaceSecondary,
                        borderRadius: AppRadius.brSmall,
                      ),
                      child: DropdownButton<String>(
                        value: selectedGame,
                        isExpanded: true,
                        underline: const SizedBox(),
                        items: _availableGames.map((g) => DropdownMenuItem(value: g, child: Text(g))).toList(),
                        onChanged: (val) {
                          if (val != null) {
                            setSheetState(() => selectedGame = val);
                          }
                        },
                      ),
                    ),
                    const SizedBox(height: AppSpacing.md),
                    Text('连接方式', style: AppTypography.caption(ctx)),
                    const SizedBox(height: 6),
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12),
                      decoration: BoxDecoration(
                        color: ctx.surfaceSecondary,
                        borderRadius: AppRadius.brSmall,
                      ),
                      child: DropdownButton<String>(
                        value: selectedMethod,
                        isExpanded: true,
                        underline: const SizedBox(),
                        items: _connectionMethods.map((m) => DropdownMenuItem(value: m, child: Text(m))).toList(),
                        onChanged: (val) {
                          if (val != null) {
                            setSheetState(() => selectedMethod = val);
                          }
                        },
                      ),
                    ),
                    const SizedBox(height: AppSpacing.md),
                    Text('连接地址', style: AppTypography.caption(ctx)),
                    const SizedBox(height: 6),
                    AmitiaTextField(
                      hintText: '输入服务器地址',
                      controller: addressController,
                      prefixIcon: Icon(Icons.dns_outlined, size: 20),
                    ),
                    const SizedBox(height: AppSpacing.md),
                    if (testState == GameConnectionState.connecting)
                      Padding(
                        padding: const EdgeInsets.only(bottom: AppSpacing.md),
                        child: Row(
                          children: [
                            SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2, color: ctx.accentPrimary),
                            ),
                            const SizedBox(width: 8),
                            Text('正在测试连接...', style: AppTypography.caption(ctx)),
                          ],
                        ),
                      ),
                    if (testState == GameConnectionState.connected)
                      Padding(
                        padding: const EdgeInsets.only(bottom: AppSpacing.md),
                        child: Row(
                          children: [
                            Icon(Icons.check_circle, size: 18, color: ctx.success),
                            const SizedBox(width: 8),
                            Text('连接测试成功', style: AppTypography.caption(ctx).copyWith(color: ctx.success)),
                          ],
                        ),
                      ),
                    if (testState == GameConnectionState.failed)
                      Padding(
                        padding: const EdgeInsets.only(bottom: AppSpacing.md),
                        child: Row(
                          children: [
                            Icon(Icons.error_outline, size: 18, color: ctx.error),
                            const SizedBox(width: 8),
                            Text('连接测试失败，请检查地址', style: AppTypography.caption(ctx).copyWith(color: ctx.error)),
                          ],
                        ),
                      ),
                    Row(
                      children: [
                        Expanded(
                          child: AmitiaButton(
                            label: isTesting ? '测试中...' : '测试连接',
                            isSecondary: true,
                            icon: Icons.wifi_protected_setup,
                            onPressed: isTesting
                                ? null
                                : () {
                                    setSheetState(() {
                                      isTesting = true;
                                      testState = GameConnectionState.connecting;
                                    });
                                    Future.delayed(const Duration(seconds: 2), () {
                                      if (!ctx.mounted) return;
                                      setSheetState(() {
                                        isTesting = false;
                                        testState = addressController.text.isNotEmpty
                                            ? GameConnectionState.connected
                                            : GameConnectionState.failed;
                                      });
                                    });
                                  },
                          ),
                        ),
                        const SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: AmitiaButton(
                            label: '保存',
                            icon: Icons.save_outlined,
                            onPressed: testState != GameConnectionState.connected
                                ? null
                                : () {
                                    Navigator.pop(ctx);
                                    setState(() {
                                      _connectionState = GameConnectionState.connected;
                                      _connectedGame = selectedGame;
                                      _connectionMethod = selectedMethod;
                                      _connectionAddress = addressController.text;
                                    });
                                    amitiaSnackBar(context, '游戏连接已保存');
                                  },
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            );
          },
        );
      },
    );
  }

  Widget _buildInstalledPluginsCard(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Column(
          children: [
            for (int i = 0; i < _installedPlugins.length; i++) ...[
              _buildPluginItem(context, _installedPlugins[i]),
              if (i < _installedPlugins.length - 1)
                Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildPluginItem(BuildContext context, String name) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => _showPluginManagementSheet(context, name),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 10),
        child: Row(
          children: [
            Container(
              width: 36,
              height: 36,
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brExtraSmall,
              ),
              child: Icon(Icons.sports_esports_outlined, size: 18, color: context.accentPrimary),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Text(name, style: AppTypography.body(context)),
            ),
            Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showPluginManagementSheet(BuildContext context, String pluginName) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetCtx) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
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
                Text(pluginName, style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                AmitiaListTile(
                  leading: Icon(Icons.info_outline, size: 20, color: context.accentPrimary),
                  title: '查看详情',
                  onTap: () {
                    Navigator.pop(sheetCtx);
                    amitiaSnackBar(context, '$pluginName v1.0.0 · 游戏辅助插件');
                  },
                ),
                AmitiaListTile(
                  leading: Icon(Icons.tune_outlined, size: 20, color: context.accentPrimary),
                  title: '管理设置',
                  onTap: () {
                    Navigator.pop(sheetCtx);
                    amitiaSnackBar(context, '已打开 $pluginName 设置');
                  },
                ),
                AmitiaListTile(
                  leading: Icon(Icons.delete_outline, size: 20, color: context.error),
                  title: '卸载',
                  onTap: () {
                    Navigator.pop(sheetCtx);
                    _confirmUninstall(context, pluginName);
                  },
                ),
                const SizedBox(height: 8),
              ],
            ),
          ),
        );
      },
    );
  }

  void _confirmUninstall(BuildContext context, String pluginName) {
    showAmitiaConfirmDialog(
      context,
      title: '卸载插件',
      message: '确定要卸载 $pluginName 吗？卸载后相关数据将被清除。',
      confirmLabel: '卸载',
      isDestructive: true,
    ).then((confirmed) {
      if (confirmed == true) {
        setState(() {
          MockData.gamePlugins.remove(pluginName);
        });
        amitiaSnackBar(context, '$pluginName 已卸载');
      }
    });
  }

  List<Widget> _buildAvailablePluginCards(BuildContext context) {
    final plugins = [
      ('游戏数据统计', '记录和分析游戏数据', Icons.analytics_outlined),
      ('自动签到', '自动完成每日签到任务', Icons.check_circle_outline),
      ('游戏攻略', '提供游戏攻略和指南', Icons.menu_book_outlined),
    ];

    return plugins.map((p) {
      final isInstalled = _installedPlugins.contains(p.$1);
      return Padding(
        padding: const EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          0,
          AppSpacing.pagePadding,
          AppSpacing.sm,
        ),
        child: AmitiaCard(
          child: Row(
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(p.$3, size: 20, color: context.accentPrimary),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(p.$1, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text(p.$2, style: AppTypography.caption(context)),
                  ],
                ),
              ),
              GestureDetector(
                onTap: isInstalled ? null : () => _startInstall(context, p.$1),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                  decoration: BoxDecoration(
                    color: isInstalled ? context.borderPrimary : context.accentPrimary,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Text(
                    isInstalled ? '已安装' : '安装',
                    style: TextStyle(fontSize: 13, color: isInstalled ? context.textTertiary : Colors.white, fontWeight: FontWeight.w500),
                  ),
                ),
              ),
            ],
          ),
        ),
      );
    }).toList();
  }

  void _startInstall(BuildContext context, String pluginName) {
    showAmitiaConfirmDialog(
      context,
      title: '安装插件',
      message: '确定要安装 $pluginName 吗？',
      confirmLabel: '安装',
    ).then((confirmed) {
      if (confirmed != true) return;
      _showInstallDialog(context, pluginName);
    });
  }

  void _showInstallDialog(BuildContext context, String pluginName) {
    setState(() {
      _installingPlugin = pluginName;
      _installProgress = 0;
    });
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (ctx, setDialogState) {
            if (_installProgress == 0) {
              _installTimer?.cancel();
              _installTimer = Timer.periodic(const Duration(milliseconds: 300), (timer) {
                final progress = _installProgress + 0.1;
                if (!mounted) {
                  timer.cancel();
                  return;
                }
                setState(() => _installProgress = progress);
                setDialogState(() {});
                if (progress >= 1.0) {
                  timer.cancel();
                  setState(() {
                    MockData.gamePlugins.add(pluginName);
                    _installingPlugin = null;
                    _installProgress = 0;
                  });
                  Navigator.pop(ctx);
                  amitiaSnackBar(context, '$pluginName 安装成功');
                }
              });
            }
            return AlertDialog(
              backgroundColor: context.surfacePrimary,
              shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
              title: Text('正在安装', style: AppTypography.cardTitle(ctx)),
              content: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(pluginName, style: AppTypography.bodySmall(ctx)),
                  const SizedBox(height: AppSpacing.md),
                  LinearProgressIndicator(
                    value: _installProgress.clamp(0.0, 1.0),
                    color: context.accentPrimary,
                    backgroundColor: context.accentSoft,
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Text('${(_installProgress * 100).clamp(0, 100).round()}%', style: AppTypography.label(ctx)),
                ],
              ),
            );
          },
        );
      },
    );
  }

  Widget _buildTasksCard(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Column(
          children: [
            for (int i = 0; i < MockData.gameTasks.length; i++) ...[
              _buildTaskItem(context, MockData.gameTasks[i]),
              if (i < MockData.gameTasks.length - 1)
                Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildTaskItem(BuildContext context, String name) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 10),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: context.info.withValues(alpha: 0.12),
              borderRadius: AppRadius.brExtraSmall,
            ),
            child: Icon(Icons.schedule, size: 18, color: context.info),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Text(name, style: AppTypography.body(context)),
          ),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
            decoration: BoxDecoration(
              color: context.borderSecondary,
              borderRadius: AppRadius.brTag,
            ),
            child: Text(
              '待执行',
              style: TextStyle(fontSize: 11, color: context.textTertiary),
            ),
          ),
        ],
      ),
    );
  }
}
