import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/ui_runtime/renderers/sandbox_web_provider_host.dart';
import '../../../../core/ui_runtime/renderers/schema_provider_host.dart';
import '../../../../core/ui_runtime/ui_provider.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ExtensionPageHostPage extends ConsumerStatefulWidget {
  const ExtensionPageHostPage({
    super.key,
    required this.pageId,
    required this.extensionId,
  });

  final String pageId;
  final String extensionId;

  @override
  ConsumerState<ExtensionPageHostPage> createState() => _ExtensionPageHostPageState();
}

class _ExtensionPageHostPageState extends ConsumerState<ExtensionPageHostPage> {
  Map<String, dynamic>? _definition;
  String? _sessionId;
  String _state = 'resolving';
  String? _error;
  List<String> _missingPermissions = const [];
  Timer? _pollTimer;

  @override
  void initState() {
    super.initState();
    Future.microtask(_open);
  }

  @override
  void didUpdateWidget(covariant ExtensionPageHostPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.pageId != widget.pageId || oldWidget.extensionId != widget.extensionId) {
      _restart();
    }
  }

  Future<void> _restart() async {
    await _close();
    if (mounted) await _open();
  }

  Future<void> _open() async {
    if (widget.extensionId.trim().isEmpty || widget.pageId.trim().isEmpty) {
      if (mounted) {
        setState(() {
          _state = 'failed';
          _error = '缺少 extensionId。请通过扩展中心打开该页面。';
        });
      }
      return;
    }
    _pollTimer?.cancel();
    setState(() {
      _state = 'resolving';
      _error = null;
      _definition = null;
      _missingPermissions = const [];
    });
    try {
      final result = await ref.read(extensionServiceProvider).openExtensionPage(
        widget.extensionId,
        widget.pageId,
        scopeSnapshot: jsonEncode({
          'platform': currentUIPlatform(),
          'host': 'mobile',
          'timestamp': DateTime.now().millisecondsSinceEpoch,
        }),
      );
      if (!mounted) return;
      _applyResult(result);
      if (_state == 'loading' || _state == 'runtime_starting') _schedulePoll();
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _state = 'failed';
        _error = error.toString();
      });
    }
  }

  void _applyResult(Map<String, dynamic> result) {
    final rawDefinition = result['definition'];
    setState(() {
      _sessionId = result['sessionId']?.toString();
      _state = (result['state'] ?? 'failed').toString();
      if (rawDefinition is Map) {
        _definition = rawDefinition.cast<String, dynamic>();
      }
      _missingPermissions = ((result['missingPermissions'] as List?) ?? const [])
          .map((item) => item.toString())
          .toList(growable: false);
      _error = result['reason']?.toString();
    });
  }

  void _schedulePoll() {
    _pollTimer?.cancel();
    _pollTimer = Timer(const Duration(seconds: 1), _poll);
  }

  Future<void> _poll() async {
    final sessionId = _sessionId;
    if (sessionId == null || sessionId.isEmpty || !mounted) return;
    try {
      final result = await ref.read(extensionServiceProvider).getExtensionPageSessionStatus(sessionId);
      if (!mounted) return;
      if (result == null) return;
      final preserved = _definition;
      _applyResult(result);
      if (_definition == null && preserved != null) {
        setState(() => _definition = preserved);
      }
      if (_state == 'loading' || _state == 'runtime_starting') _schedulePoll();
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _state = 'failed';
        _error = error.toString();
      });
    }
  }

  Future<void> _close() async {
    _pollTimer?.cancel();
    _pollTimer = null;
    final sessionId = _sessionId;
    _sessionId = null;
    if (sessionId == null || sessionId.isEmpty) return;
    try {
      await ref.read(extensionServiceProvider).closeExtensionPageSession(sessionId);
    } catch (_) {}
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    final sessionId = _sessionId;
    if (sessionId != null && sessionId.isNotEmpty) {
      unawaited(ref.read(extensionServiceProvider).closeExtensionPageSession(sessionId));
    }
    super.dispose();
  }

  UIProviderDefinition? _runtimeProvider() {
    final definition = _definition;
    if (definition == null) return null;
    final kind = definition['entryKind']?.toString();
    final contributionId = widget.pageId;
    final entry = switch (kind) {
      'schema_page' => UIProviderEntry(
          contributionId: contributionId,
          type: UIProviderEntryType.schemaRenderer,
          schemaPath: definition['schemaPath']?.toString(),
        ),
      'web_page' => UIProviderEntry(
          contributionId: contributionId,
          type: UIProviderEntryType.webRestricted,
          path: definition['entryPath']?.toString(),
        ),
      _ => null,
    };
    if (entry == null) return null;
    return UIProviderDefinition(
      providerId: 'extension-page:${widget.extensionId}:${widget.pageId}',
      extensionId: widget.extensionId,
      capability: UICapability.extensionPage,
      mode: UIProviderMode.replace,
      priority: 0,
      platforms: const [],
      entries: {'mobile': entry},
      permissions: ((definition['permissions'] as List?) ?? const [])
          .map((item) => item.toString())
          .toList(growable: false),
      generation: 0,
      enabled: true,
      builtin: false,
      metadata: const {},
    );
  }

  @override
  Widget build(BuildContext context) {
    final title = ((_definition?['title'] as Map?)?['default'] ?? widget.pageId).toString();
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: title,
        showBackButton: true,
        actions: [
          AmitiaIconButton(icon: Icons.refresh, onPressed: _restart, tooltip: '刷新'),
        ],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_state == 'failed' ||
        _state == 'disabled' ||
        _state == 'not_installed' ||
        _state == 'incompatible') {
      final suffix = _missingPermissions.isEmpty
          ? ''
          : '\n缺少权限：${_missingPermissions.join(', ')}';
      return AmitiaEmptyState(
        icon: Icons.extension_off_outlined,
        title: '扩展页面不可用',
        subtitle: '${_error ?? _state}$suffix',
        actionText: '重试',
        onAction: _restart,
      );
    }
    if (_state != 'ready') {
      final label = _state == 'permission_check' && _missingPermissions.isNotEmpty
          ? '等待权限：${_missingPermissions.join(', ')}'
          : '正在加载扩展页面…';
      return AmitiaLoadingState(message: label);
    }

    final provider = _runtimeProvider();
    if (provider == null) {
      return const AmitiaEmptyState(
        icon: Icons.warning_amber_outlined,
        title: '不支持的扩展页面类型',
      );
    }
    final entry = provider.entryFor('mobile')!;
    final runtimeContext = <String, dynamic>{
      'extensionId': widget.extensionId,
      'pageId': widget.pageId,
      'sessionId': _sessionId,
      'platform': currentUIPlatform(),
      'host': 'mobile',
    };
    if (entry.type == UIProviderEntryType.schemaRenderer) {
      return SchemaProviderHost(
        provider: provider,
        entry: entry,
        context: runtimeContext,
      );
    }
    return SandboxWebProviderHost(
      provider: provider,
      entry: entry,
      context: runtimeContext,
    );
  }
}
