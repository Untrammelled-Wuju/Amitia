import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class RuntimeModePage extends ConsumerStatefulWidget {
  const RuntimeModePage({super.key});

  @override
  ConsumerState<RuntimeModePage> createState() => _RuntimeModePageState();
}

class _RuntimeModePageState extends ConsumerState<RuntimeModePage> {
  bool _loading = true;
  bool _busy = false;
  String? _error;
  Map<String, dynamic> _mode = const {};
  Map<String, dynamic>? _validation;
  late final TextEditingController _publicUrl;

  @override
  void initState() {
    super.initState();
    _publicUrl = TextEditingController();
    _load();
  }

  @override
  void dispose() {
    _publicUrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final result = await ref.read(backendServiceProvider).get<Map<String, dynamic>>('/api/runtime/mode');
      if (!mounted) return;
      final value = result ?? <String, dynamic>{};
      final web = value['web'] is Map ? Map<String, dynamic>.from(value['web'] as Map) : <String, dynamic>{};
      setState(() {
        _mode = value;
        _publicUrl.text = (web['publicBaseUrl'] ?? '').toString();
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

  Future<void> _switch(String mode) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final result = await ref.read(backendServiceProvider).put<Map<String, dynamic>>(
        '/api/runtime/mode',
        data: {'deployMode': mode, if (mode == 'cloud-web') 'publicBaseUrl': _publicUrl.text.trim()},
      );
      if (!mounted) return;
      setState(() => _mode = result ?? _mode);
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('运行模式已保存；按后端提示建议重启 Core 后生效')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('切换失败：$e'), backgroundColor: context.error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _validate() async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final result = await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/runtime/mode/validate');
      if (!mounted) return;
      setState(() => _validation = result);
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('校验失败：$e')));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final current = (_mode['deployMode'] ?? _mode['mode'] ?? 'desktop-local').toString();
    final web = _mode['web'] is Map ? Map<String, dynamic>.from(_mode['web'] as Map) : <String, dynamic>{};
    final bridge = _mode['bridge'] is Map ? Map<String, dynamic>.from(_mode['bridge'] as Map) : <String, dynamic>{};
    final storage = _mode['storage'] is Map ? Map<String, dynamic>.from(_mode['storage'] as Map) : <String, dynamic>{};
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '运行模式', showBackButton: true, fallbackRoute: AppRoutes.settings, actions: [AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _load)]),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text('加载失败：$_error'))
              : ListView(
                  padding: EdgeInsets.all(AppSpacing.pagePadding),
                  children: [
                    AmitiaCard(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Row(children: [Expanded(child: Text(current == 'cloud-web' ? '私有云模式' : '桌面本地模式', style: AppTypography.cardTitle(context))), AmitiaStatusBadge(label: current, type: BadgeType.accent)]),
                      SizedBox(height: AppSpacing.sm),
                      Text('Core：${_mode['host'] ?? '—'}:${_mode['port'] ?? '—'}', style: AppTypography.bodySmall(context)),
                      Text('Runtime Profile：${_mode['runtimeProfile'] ?? '—'}', style: AppTypography.bodySmall(context)),
                      Text('认证：${web['requireAuth'] == true ? '必须' : '可选'}', style: AppTypography.bodySmall(context)),
                      Text('Bridge：${bridge['mode'] ?? '—'} · ${bridge['host'] ?? '—'}:${bridge['port'] ?? '—'}', style: AppTypography.bodySmall(context)),
                      Text('数据目录：${storage['dataDir'] ?? '—'}', style: AppTypography.bodySmall(context)),
                    ])),
                    SizedBox(height: AppSpacing.sectionGap),
                    AmitiaSectionHeader(title: '切换模式'),
                    SizedBox(height: AppSpacing.sm),
                    AmitiaCard(child: Column(children: [
                      RadioListTile<String>(value: 'desktop-local', groupValue: current, title: const Text('桌面本地模式'), subtitle: const Text('Core 仅本机访问'), onChanged: _busy ? null : (value) { if (value != null) _switch(value); }),
                      RadioListTile<String>(value: 'cloud-web', groupValue: current, title: const Text('私有云模式'), subtitle: const Text('Core 对远端客户端提供服务，并强制登录验证'), onChanged: _busy ? null : (value) { if (value != null) _switch(value); }),
                      if (current == 'cloud-web') ...[
                        TextField(controller: _publicUrl, decoration: const InputDecoration(labelText: 'publicBaseUrl', hintText: 'https://example.com')),
                        SizedBox(height: AppSpacing.sm),
                        AmitiaButton(label: '保存公开地址', isFullWidth: true, isSecondary: true, onPressed: _busy ? null : () => _switch('cloud-web')),
                      ],
                    ])),
                    SizedBox(height: AppSpacing.sectionGap),
                    AmitiaButton(label: '校验当前运行模式', icon: Icons.fact_check_outlined, isFullWidth: true, onPressed: _busy ? null : _validate),
                    if (_validation != null) ...[
                      SizedBox(height: AppSpacing.md),
                      AmitiaCard(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                        Row(children: [Expanded(child: Text(_validation?['valid'] == true ? '配置校验通过' : '配置存在问题', style: AppTypography.cardTitle(context))), AmitiaStatusBadge(label: _validation?['valid'] == true ? '通过' : '异常', type: _validation?['valid'] == true ? BadgeType.success : BadgeType.warning)]),
                        ...((_validation?['errors'] as List?) ?? const []).map((e) => Text('错误：$e', style: AppTypography.caption(context))),
                        ...((_validation?['warnings'] as List?) ?? const []).map((e) => Text('警告：$e', style: AppTypography.caption(context))),
                      ])),
                    ],
                    SizedBox(height: AppSpacing.xl),
                  ],
                ),
    );
  }
}
