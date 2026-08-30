import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class AppUpdatePage extends ConsumerStatefulWidget {
  const AppUpdatePage({super.key});
  @override
  ConsumerState<AppUpdatePage> createState() => _AppUpdatePageState();
}

class _AppUpdatePageState extends ConsumerState<AppUpdatePage> {
  bool _loading = true;
  bool _checking = false;
  String? _error;
  Map<String, dynamic> _version = const {};
  Map<String, dynamic> _check = const {};
  Map<String, dynamic> _config = const {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<Map<String, dynamic>?> _get(String path) => ref.read(backendServiceProvider).get<Map<String, dynamic>>(
        path,
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );

  Future<Map<String, dynamic>?> _updateCheck() => ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        '/api/update/check',
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final values = await Future.wait([_get('/api/version'), _updateCheck(), _get('/api/update/config')]);
      if (!mounted) return;
      setState(() {
        _version = values[0] ?? const {};
        _check = values[1] ?? const {};
        _config = values[2] ?? const {};
        _loading = false;
      });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  Future<void> _checkNow() async {
    setState(() => _checking = true);
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        '/api/release-check/run',
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      await _load();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('版本检查已完成')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('检查失败：$e')));
    } finally {
      if (mounted) setState(() => _checking = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final current = (_version['version'] ?? _check['currentVersion'] ?? '—').toString();
    final latest = (_check['latestVersion'] ?? current).toString();
    final hasUpdate = _check['hasUpdate'] == true;
    final channel = (_config['channel'] ?? 'stable').toString();
    return AmitiaScaffold(
      appBar: const AmitiaAppBar(title: '检查更新', navigation: AmitiaAppBarNavigation.back),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Padding(padding: const EdgeInsets.all(28), child: AmitiaButton(label: '重新加载', onPressed: _load)))
              : ListView(
                  padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.md, AppSpacing.pagePadding, AppSpacing.xl),
                  children: [
                    Container(
                      padding: const EdgeInsets.all(15),
                      decoration: BoxDecoration(color: context.surfacePrimary, borderRadius: AppRadius.brMedium, border: Border.all(color: context.borderPrimary, width: 0.6)),
                      child: Column(children: [
                        Row(children: [
                          Container(width: 46, height: 46, decoration: BoxDecoration(color: context.accentSoft, borderRadius: BorderRadius.circular(14)), child: Icon(Icons.system_update_outlined, color: context.accentPrimary)),
                          const SizedBox(width: 12),
                          Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                            Text('Amitia', style: AppTypography.cardTitle(context)),
                            const SizedBox(height: 3),
                            Text(hasUpdate ? '检测到可用更新' : '当前已是最新检查结果', style: AppTypography.caption(context)),
                          ])),
                        ]),
                        const SizedBox(height: 13),
                        _row(context, '当前版本', current),
                        _row(context, '最新版本', latest),
                        _row(context, '更新通道', channel),
                        _row(context, '最后检查', (_check['lastCheckedAt'] ?? '未记录').toString()),
                      ]),
                    ),
                    SizedBox(height: AppSpacing.lg),
                    AmitiaButton(label: _checking ? '正在检查…' : '检查更新', icon: Icons.refresh, isFullWidth: true, onPressed: _checking ? null : _checkNow),
                    SizedBox(height: AppSpacing.sm),
                    Text('版本信息来自后端 update/release-check 接口。当前没有跨平台安装 API，因此不会伪造下载或安装进度。', style: AppTypography.caption(context)),
                  ],
                ),
    );
  }

  Widget _row(BuildContext context, String label, String value) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 7),
        child: Row(children: [
          Expanded(child: Text(label, style: AppTypography.caption(context))),
          Flexible(child: Text(value, style: AppTypography.body(context), textAlign: TextAlign.right, overflow: TextOverflow.ellipsis)),
        ]),
      );
}
