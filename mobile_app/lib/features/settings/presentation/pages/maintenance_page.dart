import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class MaintenancePage extends ConsumerStatefulWidget {
  const MaintenancePage({super.key});

  @override
  ConsumerState<MaintenancePage> createState() => _MaintenancePageState();
}

class _MaintenancePageState extends ConsumerState<MaintenancePage> {
  Map<String, dynamic>? _status;
  Map<String, dynamic>? _diagnosis;
  bool _loading = true;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = ref.read(backendServiceProvider);
      final values = await Future.wait([
        api.get<Map<String, dynamic>>('/api/maintenance/status'),
        api.post<Map<String, dynamic>>('/api/maintenance/diagnose'),
      ]);
      if (!mounted) return;
      setState(() {
        _status = values[0];
        _diagnosis = values[1];
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

  List<Map<String, dynamic>> get _checks {
    final diagnosis = _diagnosis?['diagnosis'];
    final rows = diagnosis is Map ? diagnosis['checks'] : null;
    if (rows is! List) return const [];
    return rows.whereType<Map>().map((row) => Map<String, dynamic>.from(row)).toList(growable: false);
  }

  Future<void> _action(String label, String path, {Map<String, dynamic>? data}) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final result = await ref.read(backendServiceProvider).post<Map<String, dynamic>>(path, data: data);
      if (!mounted) return;
      final detail = result?['file'] ?? result?['reloadedAt'] ?? result?['status'];
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(detail == null ? '$label完成' : '$label完成：$detail')),
      );
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('$label失败：$e'), backgroundColor: context.error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '维护诊断',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
        actions: [AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _busy ? null : _load)],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? _errorView()
              : RefreshIndicator(onRefresh: _load, child: _content()),
    );
  }

  Widget _errorView() => Center(
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.xl),
          child: Column(mainAxisSize: MainAxisSize.min, children: [
            Icon(Icons.error_outline, size: 48, color: context.error),
            SizedBox(height: AppSpacing.md),
            Text('加载维护状态失败：$_error', textAlign: TextAlign.center),
            SizedBox(height: AppSpacing.md),
            AmitiaButton(label: '重试', onPressed: _load),
          ]),
        ),
      );

  Widget _content() {
    final status = (_status?['status'] ?? 'unknown').toString();
    final issues = _status?['issues'] is List ? _status!['issues'] as List : const [];
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaSectionHeader(title: '系统维护状态'),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Row(children: [
              Expanded(child: Text(status == 'healthy' ? '系统健康' : '系统存在待处理项', style: AppTypography.cardTitle(context))),
              AmitiaStatusBadge(label: status, type: status == 'healthy' ? BadgeType.success : BadgeType.warning),
            ]),
            SizedBox(height: AppSpacing.sm),
            Text('最近检查：${_status?['lastCheck'] ?? '—'}', style: AppTypography.caption(context)),
            if (issues.isNotEmpty) ...[
              SizedBox(height: AppSpacing.md),
              ...issues.whereType<Map>().map((item) => Padding(
                    padding: EdgeInsets.only(bottom: AppSpacing.xs),
                    child: Text('• ${item['type'] ?? 'ISSUE'}：${item['msg'] ?? ''}', style: AppTypography.bodySmall(context)),
                  )),
            ],
          ]),
        ),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '诊断结果'),
        SizedBox(height: AppSpacing.sm),
        if (_checks.isEmpty)
          const AmitiaCard(child: Text('后端没有返回诊断项目'))
        else
          ..._checks.map((check) {
            final pass = check['pass'] == true;
            return Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.sm),
              child: AmitiaCard(
                child: Row(children: [
                  Icon(pass ? Icons.check_circle_outline : Icons.error_outline, color: pass ? context.success : context.warning),
                  SizedBox(width: AppSpacing.md),
                  Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                    Text((check['name'] ?? '未命名检查').toString(), style: AppTypography.body(context)),
                    if (!pass) Text((check['error'] ?? '异常').toString(), style: AppTypography.caption(context)),
                  ])),
                  AmitiaStatusBadge(label: pass ? '通过' : '异常', type: pass ? BadgeType.success : BadgeType.warning),
                ]),
              ),
            );
          }),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '真实维护操作'),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(label: '重新运行诊断', icon: Icons.health_and_safety_outlined, isFullWidth: true, onPressed: _busy ? null : _load),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(label: '重新加载配置', icon: Icons.settings_backup_restore, isSecondary: true, isFullWidth: true, onPressed: _busy ? null : () => _action('重新加载配置', '/api/maintenance/reload-config', data: const {'confirmToken': 'reload-config-confirm'})),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(label: '重启微信 Bridge', icon: Icons.wechat, isSecondary: true, isFullWidth: true, onPressed: _busy ? null : () => _action('微信 Bridge 重启', '/api/maintenance/restart-bridge', data: const {'confirmToken': 'restart-bridge-confirm'})),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(label: '重启 QQ Bridge', icon: Icons.chat_bubble_outline, isSecondary: true, isFullWidth: true, onPressed: _busy ? null : () => _action('QQ Bridge 重启', '/api/maintenance/restart-qq-bridge', data: const {'confirmToken': 'restart-qq-bridge-confirm'})),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(label: '导出后端诊断报告', icon: Icons.file_download_outlined, isSecondary: true, isFullWidth: true, onPressed: _busy ? null : () => _action('诊断报告生成', '/api/maintenance/export-diagnostic')),
        SizedBox(height: AppSpacing.xl),
      ],
    );
  }
}
