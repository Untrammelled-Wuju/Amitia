import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class StoragePage extends ConsumerStatefulWidget {
  const StoragePage({super.key});

  @override
  ConsumerState<StoragePage> createState() => _StoragePageState();
}

class _StoragePageState extends ConsumerState<StoragePage> {
  Map<String, dynamic>? _info;
  Map<String, dynamic>? _migrations;
  Map<String, dynamic>? _integrity;
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
      final values = await Future.wait<Map<String, dynamic>?>([
        api.get<Map<String, dynamic>>('/api/storage/info'),
        api.get<Map<String, dynamic>>('/api/storage/migrations'),
      ]);
      if (!mounted) return;
      setState(() {
        _info = values[0];
        _migrations = values[1];
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

  Future<void> _run(String label, String path) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final result = await ref.read(backendServiceProvider).post<Map<String, dynamic>>(path);
      if (!mounted) return;
      if (path.endsWith('check-db-integrity')) _integrity = result;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('$label完成')));
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
        title: '存储与备份',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
        actions: [AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _busy ? null : _load)],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Padding(padding: EdgeInsets.all(AppSpacing.xl), child: Text('加载失败：$_error')))
              : RefreshIndicator(onRefresh: _load, child: _content()),
    );
  }

  Widget _row(String label, Object? value, {BadgeType? badge}) => Padding(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
        child: Row(children: [
          Expanded(child: Text(label, style: AppTypography.body(context))),
          badge == null ? Text('${value ?? '—'}', style: AppTypography.caption(context)) : AmitiaStatusBadge(label: '${value ?? '—'}', type: badge),
        ]),
      );

  Widget _content() {
    final migrations = _migrations?['migrations'];
    final integrityOk = _integrity == null ? null : (_integrity?['ok'] == true || _integrity?['passed'] == true);
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaSectionHeader(title: '真实存储信息'),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(child: Column(children: [
          _row('数据目录', _info?['path']),
          _row('数据库大小', _info?['dbSize']),
          _row('数据目录占用', '${_info?['usedMB'] ?? 0} MB'),
          _row('消息数', _info?['messageCount']),
          _row('对话数', _info?['conversationCount']),
          _row('记忆数', _info?['memoryCount']),
          if (integrityOk != null) _row('最近完整性检查', integrityOk ? '通过' : '异常', badge: integrityOk ? BadgeType.success : BadgeType.error),
        ])),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: 'Schema 迁移'),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          _row('当前版本', _migrations?['currentVersion']),
          _row('已应用', _migrations?['appliedCount']),
          _row('待处理', _migrations?['pendingCount'], badge: (_migrations?['pendingCount'] as num?)?.toInt() == 0 ? BadgeType.success : BadgeType.warning),
          if (migrations is List && migrations.isNotEmpty) ...migrations.whereType<Map>().take(8).map((item) => Padding(
            padding: EdgeInsets.only(top: AppSpacing.xs),
            child: Text('${item['version'] ?? ''} · ${item['name'] ?? ''} · ${item['status'] ?? (item['applied'] == true ? 'applied' : 'pending')}', style: AppTypography.caption(context)),
          )),
        ])),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '维护与备份'),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(label: '检查数据库完整性', icon: Icons.fact_check_outlined, isFullWidth: true, isSecondary: true, onPressed: _busy ? null : () => _run('数据库完整性检查', '/api/runtime/check-db-integrity')),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(label: '清理临时文件', icon: Icons.cleaning_services_outlined, isFullWidth: true, isSecondary: true, onPressed: _busy ? null : () => _run('临时文件清理', '/api/runtime/cleanup-temp')),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(label: '检查数据迁移', icon: Icons.schema_outlined, isFullWidth: true, isSecondary: true, onPressed: _busy ? null : () => _run('数据迁移检查', '/api/storage/migrations/check')),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(label: '打开完整备份与导入', icon: Icons.backup_outlined, isFullWidth: true, onPressed: () => context.push(AppRoutes.settingsBackup)),
        SizedBox(height: AppSpacing.xl),
      ],
    );
  }
}
