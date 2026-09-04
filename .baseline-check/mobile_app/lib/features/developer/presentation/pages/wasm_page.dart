import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/artifact/artifact_providers.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class WasmPage extends ConsumerStatefulWidget {
  const WasmPage({super.key});

  @override
  ConsumerState<WasmPage> createState() => _WasmPageState();
}

class _WasmPageState extends ConsumerState<WasmPage> {
  int _tab = 0;
  bool _loading = true;
  bool _busy = false;
  String? _error;
  List<Map<String, dynamic>> _definitions = const [];
  List<Map<String, dynamic>> _modules = const [];
  List<Map<String, dynamic>> _instances = const [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final api = ref.read(backendServiceProvider);
      final results = await Future.wait([
        api.get<List<dynamic>>('/api/wasm/definitions'),
        api.get<List<dynamic>>('/api/wasm/modules'),
        api.get<List<dynamic>>('/api/wasm/instances'),
      ]);
      if (!mounted) return;
      setState(() {
        _definitions = (results[0] ?? const [])
            .whereType<Map>()
            .map((e) => Map<String, dynamic>.from(e))
            .toList();
        _modules = (results[1] ?? const [])
            .whereType<Map>()
            .map((e) => Map<String, dynamic>.from(e))
            .toList();
        _instances = (results[2] ?? const [])
            .whereType<Map>()
            .map((e) => Map<String, dynamic>.from(e))
            .toList();
        _loading = false;
      });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  Future<void> _run(Future<void> Function() action) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await action();
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败：$e')));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<Dio> _dio() async {
    final availability = await ref.read(backendConnectionProvider.future);
    if (availability is! BackendConnectionAvailable) throw StateError('后端当前不可用');
    return createAuthenticatedDio(availability.config);
  }

  Future<void> _uploadModule() async {
    final result = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['wasm'],
      withData: false,
    );
    final path = result?.files.single.path;
    if (path == null || path.isEmpty || !mounted) return;
    final nameController = TextEditingController(
      text: result!.files.single.name.replaceFirst(RegExp(r'\.wasm$', caseSensitive: false), ''),
    );
    final moduleId = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('上传 WASM 模块'),
        content: TextField(
          controller: nameController,
          decoration: const InputDecoration(labelText: 'Module ID'),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, nameController.text.trim()),
            child: const Text('上传'),
          ),
        ],
      ),
    );
    if (moduleId == null || moduleId.isEmpty) return;
    await _run(() async {
      final dio = await _dio();
      try {
        await dio.post(
          '/api/wasm/modules',
          data: FormData.fromMap({
            'module_id': moduleId,
            'module': await MultipartFile.fromFile(path, filename: result.files.single.name),
          }),
        );
      } finally {
        dio.close(force: true);
      }
    });
  }

  void _createDefinition() {
    final controller = TextEditingController(
      text: const JsonEncoder.withIndent('  ').convert({
        'module_id': 'module.example',
        'extension_id': 'manual',
        'module_path': 'module.example.wasm',
        'module_hash': '',
        'module_sha256': '',
        'engine_type': 'wazero',
        'abi': 'amitia.v1',
        'wasi_version': 'preview1',
        'entry_export': 'invoke',
        'allowed_imports': [],
        'memory_limit_bytes': 67108864,
        'fuel_limit': 10000000,
        'instance_policy': 'per_call',
        'deterministic': false,
        'max_output_bytes': 1048576,
        'max_host_calls': 1000,
        'call_timeout': 30000000000,
        'definition_version': 1,
        'version': '1.0.0',
        'generation': 1,
      }),
    );
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) => Padding(
        padding: EdgeInsets.fromLTRB(20, 20, 20, MediaQuery.of(sheetContext).viewInsets.bottom + 28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('新建 WASM 定义', style: AppTypography.pageTitle(sheetContext)),
            const SizedBox(height: 8),
            Text('填写完整 WASMRuntimeDefinition JSON', style: AppTypography.caption(sheetContext)),
            const SizedBox(height: 12),
            SizedBox(
              height: 330,
              child: TextField(
                controller: controller,
                expands: true,
                minLines: null,
                maxLines: null,
                style: AppTypography.bodySmall(sheetContext).copyWith(fontFamily: 'monospace'),
                decoration: const InputDecoration(border: OutlineInputBorder()),
              ),
            ),
            const SizedBox(height: 14),
            AmitiaButton(
              label: '创建定义',
              isFullWidth: true,
              onPressed: () async {
                try {
                  final decoded = jsonDecode(controller.text);
                  if (decoded is! Map) throw const FormatException('JSON 必须为对象');
                  await ref.read(backendServiceProvider).post(
                    '/api/wasm/definitions',
                    data: Map<String, dynamic>.from(decoded),
                  );
                  if (!sheetContext.mounted) return;
                  Navigator.pop(sheetContext);
                  await _load();
                } catch (e) {
                  if (sheetContext.mounted) {
                    ScaffoldMessenger.of(sheetContext).showSnackBar(SnackBar(content: Text('创建失败：$e')));
                  }
                }
              },
            ),
          ],
        ),
      ),
    );
  }

  void _showJson(String title, Map<String, dynamic> data) {
    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: SizedBox(
          width: 680,
          child: SingleChildScrollView(
            child: SelectableText(
              const JsonEncoder.withIndent('  ').convert(data),
              style: const TextStyle(fontFamily: 'monospace'),
            ),
          ),
        ),
        actions: [TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('关闭'))],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState(message: '正在读取 WASM Runtime...');
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'WASM Runtime',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(
            icon: _tab == 0 ? Icons.add : Icons.upload_file,
            onPressed: _busy ? null : (_tab == 0 ? _createDefinition : _uploadModule),
            color: context.accentPrimary,
            tooltip: _tab == 0 ? '新建定义' : '上传模块',
          ),
        ],
      ),
      body: Column(
        children: [
          Padding(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            child: AmitiaSegmentedControl(
              segments: const ['运行时定义', '模块', '实例'],
              selectedIndex: _tab,
              onChanged: (value) => setState(() => _tab = value),
            ),
          ),
          Expanded(child: _buildCurrent()),
        ],
      ),
    );
  }

  Widget _buildCurrent() {
    final rows = switch (_tab) {
      0 => _definitions,
      1 => _modules,
      _ => _instances,
    };
    if (rows.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.extension_outlined,
        title: _tab == 0 ? '暂无运行时定义' : _tab == 1 ? '暂无 WASM 模块' : '暂无运行实例',
        subtitle: _tab == 0 ? '创建定义后会显示在这里' : _tab == 1 ? '上传 .wasm 文件后会显示在这里' : '运行模块后会生成实例',
      );
    }
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.separated(
        padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.xl),
        itemCount: rows.length,
        separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
        itemBuilder: (context, index) {
          final item = rows[index];
          final id = _tab == 0
              ? (item['runtime_definition_id'] ?? item['runtimeDefinitionId'] ?? '').toString()
              : _tab == 1
                  ? (item['module_id'] ?? '').toString()
                  : (item['instance_id'] ?? item['identity']?['instance_id'] ?? '').toString();
          final subtitle = _tab == 0
              ? '${item['module_id'] ?? ''} · ${item['engine_type'] ?? ''}'
              : _tab == 1
                  ? '${item['size'] ?? 0} bytes · ${item['valid'] == true ? 'valid' : 'invalid'}'
                  : '${item['identity']?['state'] ?? item['stats']?['state'] ?? ''}';
          return AmitiaCard(
            onTap: () => _showJson(id.isEmpty ? 'WASM' : id, item),
            child: Row(
              children: [
                Icon(Icons.extension, color: context.accentPrimary),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(id.isEmpty ? 'WASM' : id, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 3),
                      Text(subtitle, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                if (_tab < 2 && id.isNotEmpty)
                  IconButton(
                    icon: const Icon(Icons.delete_outline),
                    color: context.error,
                    onPressed: _busy
                        ? null
                        : () => _run(() async {
                              if (_tab == 0) {
                                await ref.read(backendServiceProvider).delete('/api/wasm/definitions/$id');
                              } else {
                                await ref.read(backendServiceProvider).delete('/api/wasm/modules/$id');
                              }
                            }),
                  ),
              ],
            ),
          );
        },
      ),
    );
  }
}
