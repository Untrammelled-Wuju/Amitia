import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/ui_runtime/ui_client_info.dart';
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
  Map<String, dynamic> _coreVersion = const {};
  Map<String, dynamic> _coreCheck = const {};
  Map<String, dynamic> _coreConfig = const {};

  String get _clientVersion => currentUIClientInfo().appVersion;
  String get _clientArchitecture => currentUIClientInfo().architecture;

  Future<Map<String, dynamic>?> _get(String path) => ref.read(backendServiceProvider).get<Map<String, dynamic>>(
        path,
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );

  Future<Map<String, dynamic>?> _checkCoreUpdate() => ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        '/api/update/check',
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final values = await Future.wait([
        _get('/api/version'),
        _checkCoreUpdate(),
        _get('/api/update/config'),
      ]);
      if (!mounted) return;
      setState(() {
        _coreVersion = values[0] ?? const {};
        _coreCheck = values[1] ?? const {};
        _coreConfig = values[2] ?? const {};
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

  Future<void> _refreshCoreReleaseInfo() async {
    setState(() => _checking = true);
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        '/api/release-check/run',
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      await _load();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('业务 Core 版本信息已刷新')));
      }
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('刷新失败：$e')));
    } finally {
      if (mounted) setState(() => _checking = false);
    }
  }

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  Widget build(BuildContext context) {
    final coreCurrent = (_coreVersion['version'] ?? _coreCheck['currentVersion'] ?? '—').toString();
    final coreLatest = (_coreCheck['latestVersion'] ?? coreCurrent).toString();
    final coreHasUpdate = _coreCheck['hasUpdate'] == true;
    final coreChannel = (_coreConfig['channel'] ?? 'stable').toString();

    return AmitiaScaffold(
      appBar: const AmitiaAppBar(title: '版本与更新', navigation: AmitiaAppBarNavigation.back),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(
                  child: Padding(
                    padding: const EdgeInsets.all(28),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text('版本信息加载失败', style: AppTypography.cardTitle(context)),
                        SizedBox(height: AppSpacing.sm),
                        Text(_error!, style: AppTypography.caption(context), textAlign: TextAlign.center),
                        SizedBox(height: AppSpacing.lg),
                        AmitiaButton(label: '重新加载', icon: Icons.refresh, onPressed: _load),
                      ],
                    ),
                  ),
                )
              : ListView(
                  padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.md, AppSpacing.pagePadding, AppSpacing.xl),
                  children: [
                    _versionCard(
                      context,
                      icon: Icons.phone_android_outlined,
                      title: '当前 Flutter 客户端',
                      subtitle: '版本来自当前安装包自身，不经过业务 Backend',
                      rows: [
                        MapEntry('客户端版本', _clientVersion),
                        MapEntry('运行架构', _clientArchitecture.isEmpty ? '—' : _clientArchitecture),
                        const MapEntry('更新来源', '当前未配置跨平台原生更新源'),
                      ],
                    ),
                    SizedBox(height: AppSpacing.lg),
                    _versionCard(
                      context,
                      icon: Icons.dns_outlined,
                      title: '当前连接的业务 Core',
                      subtitle: coreHasUpdate ? 'Core 检测到可用更新' : '这是服务端/Core 版本，不代表当前手机 App 版本',
                      rows: [
                        MapEntry('Core 当前版本', coreCurrent),
                        MapEntry('Core 最新版本', coreLatest),
                        MapEntry('Core 更新通道', coreChannel),
                        MapEntry('Core 最后检查', (_coreCheck['lastCheckedAt'] ?? '未记录').toString()),
                      ],
                    ),
                    SizedBox(height: AppSpacing.lg),
                    AmitiaButton(
                      label: _checking ? '正在刷新…' : '刷新 Core 版本信息',
                      icon: Icons.refresh,
                      isFullWidth: true,
                      onPressed: _checking ? null : _refreshCoreReleaseInfo,
                    ),
                    SizedBox(height: AppSpacing.sm),
                    Text(
                      '云端模式下，业务 API 会连接 Cloud Core，因此上面的 Core 版本可能是云端服务版本；客户端版本始终读取本机安装包。当前没有客户端原生下载/安装 API，所以不会把 Core 更新伪装成 Flutter App 更新。',
                      style: AppTypography.caption(context),
                    ),
                  ],
                ),
    );
  }

  Widget _versionCard(
    BuildContext context, {
    required IconData icon,
    required String title,
    required String subtitle,
    required List<MapEntry<String, String>> rows,
  }) {
    return Container(
      padding: const EdgeInsets.all(15),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.6),
      ),
      child: Column(
        children: [
          Row(
            children: [
              Container(
                width: 46,
                height: 46,
                decoration: BoxDecoration(color: context.accentSoft, borderRadius: BorderRadius.circular(14)),
                child: Icon(icon, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 3),
                    Text(subtitle, style: AppTypography.caption(context)),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 13),
          ...rows.map((entry) => _row(context, entry.key, entry.value)),
        ],
      ),
    );
  }

  Widget _row(BuildContext context, String label, String value) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 7),
        child: Row(
          children: [
            Expanded(child: Text(label, style: AppTypography.caption(context))),
            Flexible(
              child: Text(
                value,
                style: AppTypography.body(context),
                textAlign: TextAlign.right,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      );
}
