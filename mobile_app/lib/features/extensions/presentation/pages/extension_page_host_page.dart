import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/services/providers.dart';
import '../../../../core/ui_runtime/mobile_style_runtime.dart';
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
  ConsumerState<ExtensionPageHostPage> createState() =>
      _ExtensionPageHostPageState();
}

class _ExtensionPageHostPageState extends ConsumerState<ExtensionPageHostPage> {
  Map<String, dynamic>? _definition;
  String? _sessionId;
  String _state = 'resolving';
  String? _error;
  List<String> _missingPermissions = const [];
  Timer? _pollTimer;
  String? _registeredStyleTarget;

  String get _pageStyleTarget {
    return 'plugin.page.${_styleTargetSegment(widget.extensionId)}.'
        '${_styleTargetSegment(widget.pageId)}';
  }

  @override
  void initState() {
    super.initState();
    _registerStyleTarget();
    Future.microtask(_open);
  }

  @override
  void didUpdateWidget(covariant ExtensionPageHostPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.pageId != widget.pageId ||
        oldWidget.extensionId != widget.extensionId) {
      _registerStyleTarget();
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
      final result = await ref
          .read(extensionServiceProvider)
          .openExtensionPage(
            widget.extensionId,
            widget.pageId,
            scopeSnapshot: jsonEncode({
              'platform': currentUIPlatform(),
              'host': 'mobile',
              'timestamp': DateTime.now().millisecondsSinceEpoch,
            }),
          );
      if (!mounted) return;
      if (result != null) _applyResult(result);
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
      _missingPermissions =
          ((result['missingPermissions'] as List?) ?? const [])
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
      final result = await ref
          .read(extensionServiceProvider)
          .getExtensionPageSessionStatus(sessionId);
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
      await ref
          .read(extensionServiceProvider)
          .closeExtensionPageSession(sessionId);
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
}
  }

  @override
  void dispose() {
    final target = _registeredStyleTarget;
    if (target != null) AmitiaStyleTargetRegistry.unregister(target);
    _pollTimer?.cancel();
    final sessionId = _sessionId;
    if (sessionId != null && sessionId.isNotEmpty) {
      unawaited(
        ref.read(extensionServiceProvider).closeExtensionPageSession(sessionId),
      );
    }
    super.dispose();
  }

  void _registerStyleTarget() {
    final previous = _registeredStyleTarget;
    if (previous != null) AmitiaStyleTargetRegistry.unregister(previous);
    final target = _pageStyleTarget;
    _registeredStyleTarget = target;
    AmitiaStyleTargetRegistry.register(
      AmitiaStyleTargetDescriptor(
        target: target,
        page:
            'plugin.${_styleTargetSegment(widget.extensionId)}.'
            '${_styleTargetSegment(widget.pageId)}',
        properties: const <String, String>{
          'backgroundColor': 'color',
          'foregroundColor': 'color',
          'borderColor': 'color',
          'borderWidth': 'number',
          'borderRadius': 'length',
          'padding': 'insets',
          'margin': 'insets',
          'opacity': 'number',
          'width': 'length',
          'height': 'length',
        },
      ),
    );
  }

  String _styleTargetSegment(String value) {
    final normalized = value.trim().toLowerCase();
    if (normalized.isEmpty) return 'root';
    return normalized.replaceAll(RegExp(r'[^a-z0-9_-]+'), '-');
  }

  Widget _applyPageStyle(
    BuildContext context,
    AmitiaResolvedStyle style,
    Widget child,
  ) {
    if (style.properties.isEmpty) return child;
    final hasBoxStyle =
        style.has('backgroundColor') ||
        style.has('background') ||
        style.has('borderColor') ||
        style.has('borderWidth') ||
        style.has('borderRadius') ||
        style.has('boxShadow') ||
        style.has('boxShadows') ||
        style.has('shadow') ||
        style.has('gradient');
    final padding = style.edgeInsets('padding');
    final margin = style.edgeInsets('margin');
    final width = style.number('width');
    final height = style.number('height');
    Widget result = child;
    if (hasBoxStyle ||
        padding != null ||
        margin != null ||
        width != null ||
        height != null) {
      result = Container(
        width: width,
        height: height,
        padding: padding,
        margin: margin,
        decoration: hasBoxStyle
            ? style.applyBoxDecoration(const BoxDecoration())
            : null,
        child: result,
      );
    }
    final opacity = style.number('opacity');
    if (opacity != null) {
      result = Opacity(
        opacity: opacity.clamp(0.0, 1.0).toDouble(),
        child: result,
      );
    }
    return result;
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
      placement: UIProviderPlacement.any,
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
    final title = ((_definition?['title'] as Map?)?['default'] ?? widget.pageId)
        .toString();
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: title,
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            onPressed: _restart,
            tooltip: '刷新',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: AmitiaStyleTarget(
          target: _pageStyleTarget,
          builder: (context, style) =>
              _applyPageStyle(context, style, _buildBody(context)),
          child: const SizedBox.shrink(),
        ),
      ),
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
      final label =
          _state == 'permission_check' && _missingPermissions.isNotEmpty
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
