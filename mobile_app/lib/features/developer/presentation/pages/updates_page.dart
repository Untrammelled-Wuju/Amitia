import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class UpdatesPage extends ConsumerStatefulWidget {
  const UpdatesPage({super.key});

  @override
  ConsumerState<UpdatesPage> createState() => _UpdatesPageState();
}

class _UpdatesPageState extends ConsumerState<UpdatesPage> {
  bool _loading = true;
  bool _checking = false;
  String? _error;
  Map<String, dynamic> _version = {};
  Map<String, dynamic> _check = {};
  Map<String, dynamic> _config = {};
  Map<String, dynamic> _latest = {};
  List<Map<String, dynamic>> _history = [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<Map<String, dynamic>?> _get(String path) {
    return ref.read(backendServiceProvider).get<Map<String, dynamic>>(
          path,
          fromJson: (value) => Map<String, dynamic>.from(value as Map),
        );
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final values = await Future.wait([
        _get('/api/version'),
        _get('/api/update/check'),
        _get('/api/update/config'),
        _get('/api/release-check/latest'),
        _get('/api/release-check/history'),
      ]);
      if (!mounted) return;
      final historyRaw = values[4]?['history'] as List<dynamic>? ?? const [];
      setState(() {
        _version = values[0] ?? {};
        _check = values[1] ?? {};
        _config = values[2] ?? {};
        _latest = values[3]?['latest'] is Map ? Map<String, dynamic>.from(values[3]!['latest'] as Map) : (values[3] ?? {});
        _history = historyRaw.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList();
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _runCheck() async {
    setState(() => _checking = true);
    try {
      final api = ref.read(backendServiceProvider);
      await api.post<Map<String, dynamic>>(
        '/api/release-check/run',
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('版本检查已完成')));
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
    } finally {
      if (mounted) setState(() => _checking = false);
    }
  }

  Future<void> _setAutoCheck(bool value) async {
    try {
      final result = await ref.read(backendServiceProvider).put<Map<String, dynamic>>(
        '/api/update/config',
        data: {'autoCheck': value},
        fromJson: (raw) => Map<String, dynamic>.from(raw as Map),
      );
      if (!mounted) return;
      setState(() => _config = result ?? {..._config, 'autoCheck': value});
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '更新中心',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [AmitiaIconButton(icon: Icons.refresh, onPressed: _load, tooltip: '刷新')],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    final current = (_version['version'] ?? _check['currentVersion'] ?? '').toString();
    final latest = (_check['latestVersion'] ?? _latest['version'] ?? current).toString();
    final hasUpdate = _check['hasUpdate'] == true;
    final buildTime = (_version['buildTime'] ?? '').toString();
    final autoCheck = _config['autoCheck'] != false;
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
        children: [
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Container(
                        width: 48,
                        height: 48,
                        decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                        child: Icon(Icons.system_update_outlined, color: context.accentPrimary),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(current.isEmpty ? 'Amitia' : 'v$current', style: AppTypography.cardTitle(context).copyWith(fontSize: 18)),
                            if (buildTime.isNotEmpty) Text('构建时间 $buildTime', style: AppTypography.caption(context)),
                          ],
                        ),
                      ),
                      AmitiaStatusBadge(
                        label: hasUpdate ? '检测到更新' : '已是当前版本',
                        type: hasUpdate ? BadgeType.accent : BadgeType.success,
                      ),
                    ],
                  ),
                  SizedBox(height: AppSpacing.md),
                  _row(context, '当前版本', current),
                  _row(context, '检查结果版本', latest),
                  _row(context, '更新通道', (_config['channel'] ?? 'stable').toString()),
                  _row(context, '最后检查', (_check['lastCheckedAt'] ?? _history.firstOrNull?['checkedAt'] ?? '未记录').toString()),
                  SizedBox(height: AppSpacing.md),
                  AmitiaButton(
                    label: _checking ? '检查中...' : '立即检查',
                    isFullWidth: true,
                    icon: Icons.sync,
                    onPressed: _checking ? null : _runCheck,
                  ),
                  SizedBox(height: AppSpacing.sm),
                  Text(
                    hasUpdate
                        ? '后端已检测到新版本。当前后端没有提供跨平台安装 API，因此本页不会伪造下载或安装状态。'
                        : '版本检查结果来自后端 release-check/update 接口。',
                    style: AppTypography.caption(context).copyWith(color: context.textSecondary),
                  ),
                ],
              ),
            ),
          ),
          SizedBox(height: AppSpacing.xl),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaCard(
              child: SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: Text('自动检查更新', style: AppTypography.cardTitle(context)),
                subtitle: Text('由后端更新配置持久化', style: AppTypography.caption(context)),
                value: autoCheck,
                onChanged: _setAutoCheck,
              ),
            ),
          ),
          SizedBox(height: AppSpacing.xl),
          const AmitiaSectionHeader(title: '检查历史'),
          SizedBox(height: AppSpacing.sm),
          if (_history.isEmpty)
            const AmitiaEmptyState(icon: Icons.history, title: '暂无检查历史')
          else
            ..._history.map((item) => Padding(
                  padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
                  child: AmitiaCard(
                    onTap: () => _showJson(item),
                    child: Row(
                      children: [
                        Icon(Icons.history, color: context.textSecondary),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text('v${item['version'] ?? ''}', style: AppTypography.cardTitle(context)),
                              Text((item['checkedAt'] ?? '').toString(), style: AppTypography.caption(context)),
                            ],
                          ),
                        ),
                        AmitiaStatusBadge(
                          label: item['hasUpdate'] == true ? '有更新' : '无更新',
                          type: item['hasUpdate'] == true ? BadgeType.accent : BadgeType.neutral,
                        ),
                      ],
                    ),
                  ),
                )),
        ],
      ),
    );
  }

  Widget _row(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          SizedBox(width: 110, child: Text(label, style: AppTypography.label(context))),
          Expanded(child: Text(value.isEmpty ? '-' : value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }

  void _showJson(Map<String, dynamic> item) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('检查详情'),
        content: SelectableText(const JsonEncoder.withIndent('  ').convert(item)),
        actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('关闭'))],
      ),
    );
  }
}
