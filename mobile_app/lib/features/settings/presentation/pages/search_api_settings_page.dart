import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class SearchApiSettingsPage extends ConsumerStatefulWidget {
  const SearchApiSettingsPage({super.key});

  @override
  ConsumerState<SearchApiSettingsPage> createState() =>
      _SearchApiSettingsPageState();
}

class _SearchApiSettingsPageState extends ConsumerState<SearchApiSettingsPage> {
  final _formKey = GlobalKey<FormState>();
  final _controllers = <String, TextEditingController>{};
  final _saving = <String>{};
  final _clearing = <String>{};
  final _visible = <String>{};
  List<Map<String, dynamic>> _items = const [];
  bool _loading = true;
  String? _loadError;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    for (final controller in _controllers.values) {
      controller.dispose();
    }
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _loadError = null;
    });
    try {
      final items = await ref.read(searchApiServiceProvider).listCredentials();
      for (final item in items) {
        final engineId = _engineId(item);
        if (engineId.isEmpty) continue;
        _controllers.putIfAbsent(engineId, TextEditingController.new);
      }
      if (!mounted) return;
      setState(() {
        _items = items;
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _loadError = error.toString();
        _loading = false;
      });
    }
  }

  Future<void> _save(Map<String, dynamic> item) async {
    final engineId = _engineId(item);
    final controller = _controllers[engineId];
    final value = controller?.text.trim() ?? '';
    if (engineId.isEmpty || value.isEmpty || _saving.contains(engineId)) return;
    setState(() => _saving.add(engineId));
    try {
      final updated = await ref
          .read(searchApiServiceProvider)
          .saveCredential(engineId, value);
      if (!mounted) return;
      setState(() {
        final index = _items.indexWhere(
          (candidate) => _engineId(candidate) == engineId,
        );
        if (index >= 0) {
          final next = Map<String, dynamic>.from(_items[index]);
          next['configured'] = updated['configured'] == true;
          next['updatedAt'] = updated['updatedAt'] ?? '';
          _items = List<Map<String, dynamic>>.from(_items)..[index] = next;
        }
        controller?.clear();
      });
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('${_displayName(item)} 已保存')));
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('搜索凭据保存失败')));
      }
    } finally {
      if (mounted) setState(() => _saving.remove(engineId));
    }
  }

  Future<void> _clear(Map<String, dynamic> item) async {
    final engineId = _engineId(item);
    if (engineId.isEmpty || _clearing.contains(engineId)) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('清除凭据'),
        content: Text('确定清除 ${_displayName(item)} 吗？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('清除'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    setState(() => _clearing.add(engineId));
    try {
      await ref.read(searchApiServiceProvider).clearCredential(engineId);
      if (!mounted) return;
      setState(() {
        final index = _items.indexWhere(
          (candidate) => _engineId(candidate) == engineId,
        );
        if (index >= 0) {
          final next = Map<String, dynamic>.from(_items[index]);
          next['configured'] = false;
          next['updatedAt'] = '';
          _items = List<Map<String, dynamic>>.from(_items)..[index] = next;
        }
        _controllers[engineId]?.clear();
      });
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('${_displayName(item)} 已清除')));
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('搜索凭据清除失败')));
      }
    } finally {
      if (mounted) setState(() => _clearing.remove(engineId));
    }
  }

  Future<void> _openKeyURL(Map<String, dynamic> item) async {
    final raw = (item['keyUrl'] ?? '').toString().trim();
    final uri = Uri.tryParse(raw);
    if (uri == null || !uri.hasScheme) return;
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '搜索 API',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
      ),
      body: Form(
        key: _formKey,
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _loadError != null
            ? _buildLoadError()
            : ListView(
                padding: EdgeInsets.only(
                  top: AppSpacing.md,
                  bottom: AppSpacing.xl,
                ),
                children: [
                  for (final item in _items) _buildCredentialItem(item),
                ],
              ),
      ),
    );
  }

  Widget _buildLoadError() {
    return Center(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('搜索 API 配置加载失败', style: TextStyle(color: context.error)),
            SizedBox(height: AppSpacing.md),
            OutlinedButton(onPressed: _load, child: const Text('重试')),
          ],
        ),
      ),
    );
  }

  Widget _buildCredentialItem(Map<String, dynamic> item) {
    final engineId = _engineId(item);
    final configured = item['configured'] == true;
    final controller = _controllers[engineId];
    final saving = _saving.contains(engineId);
    final clearing = _clearing.contains(engineId);
    final visible = _visible.contains(engineId);
    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(color: context.borderPrimary, width: 0.5),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  _displayName(item),
                  style: const TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              Text(
                configured ? '已配置' : '未配置',
                style: TextStyle(
                  fontSize: 12,
                  color: configured ? context.success : context.textTertiary,
                ),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          TextFormField(
            controller: controller,
            obscureText: !visible,
            keyboardType: TextInputType.visiblePassword,
            autocorrect: false,
            enableSuggestions: false,
            decoration: InputDecoration(
              hintText: '请输入 API Key',
              suffixIcon: IconButton(
                onPressed: () {
                  setState(() {
                    if (visible) {
                      _visible.remove(engineId);
                    } else {
                      _visible.add(engineId);
                    }
                  });
                },
                icon: Icon(
                  visible
                      ? Icons.visibility_off_outlined
                      : Icons.visibility_outlined,
                ),
                tooltip: visible ? '隐藏' : '显示',
              ),
            ),
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              if ((item['keyUrl'] ?? '').toString().isNotEmpty)
                TextButton(
                  onPressed: () => _openKeyURL(item),
                  child: const Text('获取 API Key'),
                )
              else
                const SizedBox.shrink(),
              const Spacer(),
              TextButton(
                onPressed: !configured || clearing ? null : () => _clear(item),
                style: TextButton.styleFrom(foregroundColor: context.error),
                child: clearing
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('清除'),
              ),
              SizedBox(width: AppSpacing.sm),
              if (controller == null)
                const FilledButton(onPressed: null, child: Text('保存'))
              else
                ValueListenableBuilder<TextEditingValue>(
                  valueListenable: controller,
                  builder: (context, value, child) {
                    return FilledButton(
                      onPressed: value.text.trim().isEmpty || saving
                          ? null
                          : () => _save(item),
                      child: saving
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Text('保存'),
                    );
                  },
                ),
            ],
          ),
        ],
      ),
    );
  }

  String _engineId(Map<String, dynamic> item) {
    return (item['engineId'] ?? '').toString().trim();
  }

  String _displayName(Map<String, dynamic> item) {
    final name = (item['name'] ?? '').toString().trim();
    return name.isEmpty ? _engineId(item) : name;
  }
}
