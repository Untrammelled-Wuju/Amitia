import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/app_update/app_update_models.dart';
import '../../../../core/app_update/app_update_providers.dart';
import '../../../../core/app_update/app_update_service.dart';
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
  InstalledAppInfo? _installedApp;
  AppUpdateManifest? _availableAppUpdate;
  int? _downloadId;
  int _downloadedBytes = 0;
  int _totalBytes = 0;
  bool _appChecking = false;
  bool _appDownloading = false;
  bool _appPolling = false;
  bool _downloadReady = false;
  bool _installStarted = false;
  String? _appError;
  String? _appMessage;
  Timer? _downloadTimer;

  String get _clientArchitecture => currentUIClientInfo().architecture;

  Future<Map<String, dynamic>?> _get(String path) => ref
      .read(backendServiceProvider)
      .get<Map<String, dynamic>>(
        path,
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );

  Future<Map<String, dynamic>?> _checkCoreUpdate() => ref
      .read(backendServiceProvider)
      .post<Map<String, dynamic>>(
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
      await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>(
            '/api/release-check/run',
            fromJson: (value) => Map<String, dynamic>.from(value as Map),
          );
      await _load();
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('业务 Core 版本信息已刷新')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('刷新失败：$e')));
      }
    } finally {
      if (mounted) setState(() => _checking = false);
    }
  }

  @override
  void initState() {
    super.initState();
    _load();
    unawaited(_checkAppUpdate(silent: true));
    unawaited(_consumeInstallResult());
  }

  @override
  void dispose() {
    _downloadTimer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final coreCurrent =
        (_coreVersion['version'] ?? _coreCheck['currentVersion'] ?? '—')
            .toString();
    final coreLatest = (_coreCheck['latestVersion'] ?? coreCurrent).toString();
    final coreHasUpdate = _coreCheck['hasUpdate'] == true;
    final coreChannel = (_coreConfig['channel'] ?? 'stable').toString();
    final clientVersion =
        _installedApp?.versionName ?? currentUIClientInfo().appVersion;
    final clientVersionCode = _installedApp?.versionCode ?? 0;

    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '版本与更新',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                AppSpacing.md,
                AppSpacing.pagePadding,
                AppSpacing.xl,
              ),
              children: [
                _versionCard(
                  context,
                  icon: Icons.phone_android_outlined,
                  title: '当前 Flutter 客户端',
                  subtitle: '通过自建渠道检查和安装客户端更新',
                  rows: [
                    MapEntry('客户端版本', clientVersion),
                    MapEntry(
                      '版本代码',
                      clientVersionCode > 0
                          ? clientVersionCode.toString()
                          : '—',
                    ),
                    MapEntry(
                      '运行架构',
                      _clientArchitecture.isEmpty ? '—' : _clientArchitecture,
                    ),
                    MapEntry('更新通道', AppUpdateService.updateChannel),
                    const MapEntry('更新来源', '自建渠道'),
                  ],
                ),
                SizedBox(height: AppSpacing.md),
                _buildAppUpdateCard(context),
                SizedBox(height: AppSpacing.lg),
                if (_error != null) ...[
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(15),
                    decoration: BoxDecoration(
                      color: context.surfacePrimary,
                      borderRadius: AppRadius.brMedium,
                      border: Border.all(
                        color: context.borderPrimary,
                        width: 0.6,
                      ),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          '业务 Core 信息加载失败',
                          style: AppTypography.cardTitle(context),
                        ),
                        const SizedBox(height: 8),
                        Text(_error!, style: AppTypography.caption(context)),
                        const SizedBox(height: 12),
                        AmitiaButton(
                          label: '重新加载 Core 信息',
                          icon: Icons.refresh,
                          isFullWidth: true,
                          onPressed: _load,
                        ),
                      ],
                    ),
                  ),
                  SizedBox(height: AppSpacing.lg),
                ],
                _versionCard(
                  context,
                  icon: Icons.dns_outlined,
                  title: '当前连接的业务 Core',
                  subtitle: coreHasUpdate
                      ? 'Core 检测到可用更新'
                      : '这是服务端/Core 版本，不代表当前手机 App 版本',
                  rows: [
                    MapEntry('Core 当前版本', coreCurrent),
                    MapEntry('Core 最新版本', coreLatest),
                    MapEntry('Core 更新通道', coreChannel),
                    MapEntry(
                      'Core 最后检查',
                      (_coreCheck['lastCheckedAt'] ?? '未记录').toString(),
                    ),
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
                  '云端模式下，业务 API 会连接 Cloud Core，因此上面的 Core 版本可能是云端服务版本；客户端版本始终读取本机安装包，客户端更新不依赖当前连接的业务 Core。',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
    );
  }

  Widget _buildAppUpdateCard(BuildContext context) {
    final update = _availableAppUpdate;
    final installed = _installedApp;
    final mandatory =
        update != null &&
        installed != null &&
        update.requiresVersion(installed.versionCode);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(15),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.6),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Icon(Icons.system_update_alt, color: context.accentPrimary),
              const SizedBox(width: 10),
              Expanded(
                child: Text('应用自更新', style: AppTypography.cardTitle(context)),
              ),
              if (mandatory)
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Text(
                    '必须更新',
                    style: AppTypography.caption(context).copyWith(
                      color: context.accentPrimary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
            ],
          ),
          if (update != null) ...[
            const SizedBox(height: 10),
            Text(
              '可更新至 v${update.versionName}（${update.versionCode}）',
              style: AppTypography.body(context),
            ),
            if (update.releaseNotes.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(
                update.releaseNotes,
                style: AppTypography.caption(context),
                maxLines: 6,
                overflow: TextOverflow.ellipsis,
              ),
            ],
          ],
          if (_appMessage != null) ...[
            const SizedBox(height: 10),
            Text(
              _appMessage!,
              style: AppTypography.caption(
                context,
              ).copyWith(color: context.accentPrimary),
            ),
          ],
          if (_appError != null) ...[
            const SizedBox(height: 10),
            Text(
              _appError!,
              style: AppTypography.caption(
                context,
              ).copyWith(color: Theme.of(context).colorScheme.error),
            ),
          ],
          if (_appDownloading) ...[
            const SizedBox(height: 12),
            LinearProgressIndicator(
              value: _totalBytes > 0
                  ? (_downloadedBytes / _totalBytes).clamp(0, 1).toDouble()
                  : null,
            ),
            const SizedBox(height: 6),
            Text(
              _totalBytes > 0
                  ? '${_formatBytes(_downloadedBytes)} / ${_formatBytes(_totalBytes)}'
                  : '正在准备下载',
              style: AppTypography.caption(context),
            ),
          ],
          const SizedBox(height: 12),
          AmitiaButton(
            label: _appChecking ? '检查中...' : '检查应用更新',
            icon: Icons.refresh,
            isFullWidth: true,
            onPressed: _appChecking || _appDownloading
                ? null
                : () => _checkAppUpdate(),
          ),
          if (update != null) ...[
            const SizedBox(height: 8),
            AmitiaButton(
              label: _appDownloading
                  ? '正在下载...'
                  : _installStarted
                  ? '已拉起系统安装程序'
                  : _downloadReady
                  ? '安装 v${update.versionName}'
                  : '下载并安装 v${update.versionName}',
              icon: _downloadReady ? Icons.install_mobile : Icons.download,
              isFullWidth: true,
              onPressed: _appDownloading || _installStarted
                  ? null
                  : _downloadReady
                  ? _installAppUpdate
                  : _startAppDownload,
            ),
          ],
          if (_downloadReady &&
              _installedApp?.canInstallPackages == false &&
              !_appDownloading) ...[
            const SizedBox(height: 8),
            AmitiaButton(
              label: '允许安装未知应用',
              icon: Icons.security,
              isFullWidth: true,
              onPressed: _openInstallPermissionSettings,
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _checkAppUpdate({bool silent = false}) async {
    if (_appChecking) return;
    setState(() {
      _appChecking = true;
      _appError = null;
      if (!silent) {
        _appMessage = null;
      }
    });
    try {
      final service = ref.read(appUpdateServiceProvider);
      final result = await service.check();
      if (!mounted) return;
      setState(() {
        _installedApp = result.installed;
        _availableAppUpdate = result.available;
        _installStarted = false;
        _downloadReady = false;
        _downloadId = null;
        _downloadedBytes = 0;
        _totalBytes = 0;
        if (result.reason == 'already_latest') {
          _appMessage = '当前已是最新版本';
        } else if (result.reason == 'rollout_excluded') {
          _appMessage = '当前设备暂未进入该版本灰度范围';
        } else if (result.hasUpdate) {
          _appMessage = '发现新版本 v${result.available!.versionName}';
        }
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _appError = '检查更新失败：$error';
      });
    } finally {
      if (mounted) setState(() => _appChecking = false);
    }
  }

  Future<void> _startAppDownload() async {
    final update = _availableAppUpdate;
    if (update == null || _appDownloading) return;
    setState(() {
      _appDownloading = true;
      _downloadReady = false;
      _appError = null;
      _appMessage = '正在下载 v${update.versionName}';
      _downloadedBytes = 0;
      _totalBytes = update.apk.size;
    });
    try {
      final downloadId = await ref
          .read(appUpdateServiceProvider)
          .download(update);
      if (!mounted) return;
      setState(() => _downloadId = downloadId);
      _startDownloadPolling();
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _appDownloading = false;
        _appError = '下载启动失败：$error';
      });
    }
  }

  void _startDownloadPolling() {
    _downloadTimer?.cancel();
    _downloadTimer = Timer.periodic(
      const Duration(seconds: 1),
      (_) => unawaited(_pollDownloadStatus()),
    );
    unawaited(_pollDownloadStatus());
  }

  Future<void> _pollDownloadStatus() async {
    if (_appPolling) return;
    final downloadId = _downloadId;
    if (downloadId == null) return;
    _appPolling = true;
    try {
      final status = await ref
          .read(appUpdateServiceProvider)
          .getDownloadStatus(downloadId);
      if (!mounted) return;
      setState(() {
        _downloadedBytes = status.downloadedBytes;
        _totalBytes = status.totalBytes > 0
            ? status.totalBytes
            : _availableAppUpdate?.apk.size ?? 0;
      });
      if (status.successful) {
        _downloadTimer?.cancel();
        if (mounted) {
          setState(() {
            _appDownloading = false;
            _downloadReady = true;
            _appMessage = '下载完成，正在校验并拉起安装程序';
          });
        }
        await _installAppUpdate();
      } else if (status.failed) {
        _downloadTimer?.cancel();
        if (mounted) {
          setState(() {
            _appDownloading = false;
            _downloadReady = false;
            _appError = '下载失败，请检查网络后重试';
          });
        }
      }
    } catch (error) {
      _downloadTimer?.cancel();
      if (mounted) {
        setState(() {
          _appDownloading = false;
          _downloadReady = false;
          _appError = '下载状态检查失败：$error';
        });
      }
    } finally {
      _appPolling = false;
    }
  }

  Future<void> _installAppUpdate() async {
    final update = _availableAppUpdate;
    final downloadId = _downloadId;
    if (update == null || downloadId == null) return;
    try {
      final service = ref.read(appUpdateServiceProvider);
      final installed = await service.getInstalledInfo();
      if (!mounted) return;
      setState(() => _installedApp = installed);
      if (!installed.canInstallPackages) {
        setState(() {
          _appMessage = '请先允许 Amitia 安装未知应用，然后重新点击安装';
          _appError = null;
        });
        return;
      }
      await service.install(downloadId, update);
      if (!mounted) return;
      setState(() {
        _installStarted = true;
        _appMessage = '已拉起 Android 系统安装程序，请确认安装';
        _appError = null;
      });
    } catch (error) {
      if (!mounted) return;
      if (error is PlatformException &&
          error.code == 'INSTALL_PERMISSION_REQUIRED') {
        try {
          final installed = await ref
              .read(appUpdateServiceProvider)
              .getInstalledInfo();
          if (!mounted) return;
          setState(() {
            _installedApp = installed;
            _appMessage = '请先允许 Amitia 安装未知应用，然后再次点击安装';
            _appError = null;
          });
          return;
        } catch (_) {
          return;
        }
      }
      setState(() {
        _downloadReady = false;
        _downloadId = null;
        _appError = '安装启动失败：$error';
      });
    }
  }

  Future<void> _openInstallPermissionSettings() async {
    try {
      await ref.read(appUpdateServiceProvider).openInstallPermissionSettings();
      if (!mounted) return;
      setState(() {
        _appMessage = '授权后返回此页面，再次点击安装更新';
        _appError = null;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() => _appError = '无法打开安装权限设置：$error');
    }
  }

  Future<void> _consumeInstallResult() async {
    try {
      final result = await ref
          .read(appUpdateServiceProvider)
          .consumeInstallResult();
      if (!mounted || result == null) return;
      if (result.successful) {
        setState(() {
          _appMessage = '更新安装成功';
          _appError = null;
          _availableAppUpdate = null;
          _installStarted = false;
        });
        await _checkAppUpdate(silent: true);
      } else {
        setState(() {
          _appError = result.message.isEmpty
              ? '更新安装未完成：${result.status}'
              : '更新安装未完成：${result.message}';
        });
      }
    } catch (_) {
      return;
    }
  }

  String _formatBytes(int value) {
    if (value < 1024) return '$value B';
    if (value < 1024 * 1024) return '${(value / 1024).toStringAsFixed(1)} KB';
    if (value < 1024 * 1024 * 1024) {
      return '${(value / (1024 * 1024)).toStringAsFixed(1)} MB';
    }
    return '${(value / (1024 * 1024 * 1024)).toStringAsFixed(2)} GB';
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
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: BorderRadius.circular(14),
                ),
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
