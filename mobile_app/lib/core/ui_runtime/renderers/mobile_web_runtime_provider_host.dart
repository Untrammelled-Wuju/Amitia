import 'dart:async';
import 'dart:convert';
import 'dart:developer' as developer;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../../backend_connection/backend_connection_availability.dart';
import '../../backend_connection/backend_uri_builder.dart';
import '../../backend_connection/providers/backend_connection_providers.dart';
import '../../services/providers.dart';
import '../mobile_right_pane_runtime.dart';
import '../mobile_web_runtime_layout.dart';
import '../mobile_web_runtime_uri.dart';
import '../ui_provider.dart';

class MobileWebRuntimeProviderHost extends ConsumerStatefulWidget {
  const MobileWebRuntimeProviderHost({
    super.key,
    required this.provider,
    required this.entry,
    this.context = const {},
    this.actions = const {},
    this.fallback,
    this.onFailure,
    this.contributionId,
  });

  final UIProviderDefinition provider;
  final UIProviderEntry entry;
  final Map<String, dynamic> context;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;
  final Widget? fallback;
  final ValueChanged<Object>? onFailure;
  final String? contributionId;

  @override
  ConsumerState<MobileWebRuntimeProviderHost> createState() =>
      _MobileWebRuntimeProviderHostState();
}

class _MobileWebRuntimeProviderHostState
    extends ConsumerState<MobileWebRuntimeProviderHost> {
  WebViewController? _controller;
  Object? _error;
  bool _loading = true;
  int _loadToken = 0;
  double _height = 320;
  Map<String, dynamic>? _uiSession;
  int _nonceSequence = 0;

  @override
  void initState() {
    super.initState();
    _height = mobileWebRuntimeInitialHeight(widget.context);
    _start();
  }

  @override
  void didUpdateWidget(covariant MobileWebRuntimeProviderHost oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.provider.providerId != widget.provider.providerId ||
        oldWidget.provider.generation != widget.provider.generation ||
        oldWidget.entry.path != widget.entry.path ||
        oldWidget.entry.stylePath != widget.entry.stylePath ||
        oldWidget.entry.styleHash != widget.entry.styleHash ||
        oldWidget.entry.contentHash != widget.entry.contentHash ||
        oldWidget.entry.moduleToken != widget.entry.moduleToken ||
        oldWidget.contributionId != widget.contributionId) {
      _start();
      return;
    }
    _pushContext();
  }

  Uri _providerResourceUri(
    String rawPath, {
    required String moduleToken,
    required String contentHash,
  }) {
    final availability = ref.read(backendConnectionProvider).valueOrNull;
    if (availability is! BackendConnectionAvailable) {
      throw StateError('backend connection unavailable');
    }
    final base = BackendUriBuilder().httpBase(availability.config);
    final contributionId = widget.contributionId?.trim() ?? '';
    if (contributionId.isNotEmpty) {
      return buildMobileWebRuntimeContributionUri(
        base: base,
        extensionId: widget.provider.extensionId,
        contributionId: contributionId,
        path: rawPath,
        generation: widget.provider.generation,
        contentHash: contentHash,
        moduleToken: moduleToken,
      );
    }
    return buildMobileWebRuntimeResourceUri(
      base: base,
      providerId: widget.provider.providerId,
      path: rawPath,
      generation: widget.provider.generation,
      contentHash: contentHash,
      moduleToken: moduleToken,
    );
  }

  Uri _moduleUri() {
    return _providerResourceUri(
      widget.entry.path ?? '',
      moduleToken: widget.entry.moduleToken?.trim() ?? '',
      contentHash: widget.entry.contentHash?.trim() ?? '',
    );
  }

  Uri? _styleUri() {
    final path = widget.entry.stylePath?.trim() ?? '';
    if (path.isEmpty) return null;
    return _providerResourceUri(
      path,
      moduleToken: widget.entry.moduleToken?.trim() ?? '',
      contentHash: widget.entry.styleHash?.trim() ?? '',
    );
  }

  Future<void> _start() async {
    final token = ++_loadToken;
    await _revokeUISession();
    if (!mounted || token != _loadToken) return;
    setState(() {
      _loading = true;
      _error = null;
      _controller = null;
    });
    try {
      final moduleUri = _moduleUri();
      _uiSession = await _createUISession();
      final styleUri = _styleUri();
      final html = _html(moduleUri, styleUri);
      final controller = WebViewController()
        ..setJavaScriptMode(JavaScriptMode.unrestricted)
        ..setBackgroundColor(Colors.transparent)
        ..setOnConsoleMessage(
          (message) => developer.log(
            message.message,
            name: 'amitia.ui.web-runtime.${widget.entry.contributionId}',
            level: message.level.name == 'error' ? 1000 : 500,
          ),
        )
        ..addJavaScriptChannel(
          'AmitiaNativeBridge',
          onMessageReceived: (message) => _handleBridge(message.message),
        )
        ..setNavigationDelegate(
          NavigationDelegate(
            onPageFinished: (_) {
              if (!mounted || token != _loadToken) return;
              setState(() => _loading = false);
              _pushContext();
            },
            onWebResourceError: (error) {
              if (error.isForMainFrame != true ||
                  !mounted ||
                  token != _loadToken) {
                return;
              }
              final failure = StateError(error.description);
              setState(() {
                _loading = false;
                _error = failure;
              });
              widget.onFailure?.call(failure);
            },
          ),
        );
      if (!mounted || token != _loadToken) return;
      _controller = controller;
      await controller.loadHtmlString(html);
    } catch (error) {
      if (!mounted || token != _loadToken) return;
      setState(() {
        _loading = false;
        _error = error;
      });
      widget.onFailure?.call(error);
    }
  }

  Future<Map<String, dynamic>?> _createUISession() async {
    final contributionId = widget.entry.contributionId?.trim() ?? '';
    if (contributionId.isEmpty) return null;
    final result = await ref
        .read(extensionServiceProvider)
        .createSchemaUISession(
          contributionId: contributionId,
          characterId: (widget.context['characterId'] ?? '').toString(),
          conversationId: (widget.context['conversationId'] ?? '').toString(),
          surface: (widget.context['surfaceRole'] ?? 'mobile').toString(),
        );
    if ((result['sessionId'] ?? '').toString().trim().isEmpty) {
      throw StateError(
        'mobile web runtime UI session did not return sessionId',
      );
    }
    return result;
  }

  String _html(Uri moduleUri, Uri? styleUri) {
    final context = jsonEncode(widget.context);
    final module = jsonEncode(moduleUri.toString());
    final rootMinHeight = isCompactMobileWebRuntimeContext(widget.context)
        ? '0'
        : '100%';
    final style = styleUri == null
        ? ''
        : '<link rel="stylesheet" href="${_htmlAttribute(styleUri.toString())}">';
    return '''
<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no">
$style
<style>
html,body,#root{width:100%;min-height:$rootMinHeight;margin:0;padding:0;background:transparent;color-scheme:light dark}
body{overflow:auto;font-family:system-ui,-apple-system,sans-serif}
</style>
</head>
<body>
<div id="root"></div>
<script type="module">
window.__AMITIA_MOBILE_CONTEXT__ = $context;
const pending = new Map();
const contextListeners = new Set();
let sequence = 0;
function post(method, input) {
  window.AmitiaNativeBridge?.postMessage(JSON.stringify({method, input}));
}
function request(method, input) {
  const id = "amd-" + Date.now().toString(36) + "-" + (++sequence).toString(36);
  return new Promise((resolve, reject) => {
    pending.set(id, {resolve, reject});
    window.AmitiaNativeBridge?.postMessage(JSON.stringify({id, method, input}));
  });
}
window.addEventListener("amitia:native-bridge-response", (event) => {
  const detail = event.detail || {};
  const id = String(detail.id || "");
  const entry = pending.get(id);
  if (!entry) return;
  pending.delete(id);
  if (detail.ok === true) entry.resolve(detail.output);
  else entry.reject(new Error(String(detail.error || "mobile host request failed")));
});
window.__amitiaUpdateContext = (context) => {
  window.__AMITIA_MOBILE_CONTEXT__ = context;
  window.dispatchEvent(new CustomEvent('amitia:host-context', {detail: context}));
  for (const listener of contextListeners) listener(context);
};
window.AmitiaHost = Object.freeze({
  get context() {
    return window.__AMITIA_MOBILE_CONTEXT__;
  },
  post,
  request,
  invokeAction(actionId, input) {
    return request("ui.action.invoke", {actionId, input});
  },
  action(actionId, input) {
    return this.invokeAction(actionId, input);
  },
  resize(height) {
    return request("ui.resize.request", {height});
  },
  openRightPane(input) {
    return request("ui.right-pane.open", input);
  },
  closeRightPane() {
    return request("ui.right-pane.close", {});
  },
  subscribeContext(listener) {
    if (typeof listener !== "function") throw new Error("context listener must be a function");
    contextListeners.add(listener);
    return () => contextListeners.delete(listener);
  },
});
try {
  const mod = await import($module);
  const mount = typeof mod.mount === 'function' ? mod.mount : mod.default?.mount;
  if (typeof mount !== 'function') throw new Error('mobile_web_runtime must export mount(element, context)');
  await mount(document.getElementById('root'), window.__AMITIA_MOBILE_CONTEXT__);
} catch (error) {
  post('module.error', {message: error?.message ?? String(error), stack: error?.stack ?? '', errorCode: error?.code});
}
</script>
</body>
</html>
''';
  }

  String _htmlAttribute(String value) =>
      value.replaceAll('&', '&amp;').replaceAll('"', '&quot;');

  Future<void> _pushContext() async {
    final controller = _controller;
    if (controller == null) return;
    final encoded = jsonEncode(widget.context);
    await controller.runJavaScript('window.__amitiaUpdateContext?.($encoded);');
  }

  Future<void> _handleBridge(String raw) async {
    final controller = _controller;
    if (controller == null) return;
    Map<String, dynamic> payload;
    try {
      final decoded = jsonDecode(raw);
      payload = decoded is Map
          ? decoded.cast<String, dynamic>()
          : <String, dynamic>{'payload': decoded};
    } catch (error) {
      await _respond(controller, '', '', <String, dynamic>{
        'ok': false,
        'error': error.toString(),
      });
      return;
    }
    final method = payload['method']?.toString() ?? '';
    final requestId = payload['id']?.toString() ?? '';
    final input = payload['input'];
    final inputMap = input is Map
        ? input.cast<String, dynamic>()
        : const <String, dynamic>{};
    try {
      if (method == 'module.error') {
        final error = StateError(
          '${inputMap['message'] ?? raw}\n${inputMap['stack'] ?? ''}',
        );
        setState(() => _error = error);
        widget.onFailure?.call(error);
        return;
      }
      if (method == 'ui.resize.request') {
        final height = (inputMap['height'] as num?)?.toDouble();
        if (height != null && height >= 1 && height <= 20000) {
          setState(() => _height = height);
        }
        await _respond(controller, requestId, method, <String, dynamic>{
          'ok': true,
          'output': <String, dynamic>{'height': _height},
        });
        return;
      }
      if (method == 'ui.right-pane.open') {
        final address = (inputMap['address'] ?? inputMap['uri'] ?? '')
            .toString()
            .trim();
        ref
            .read(mobileRightPaneProvider.notifier)
            .open(
              address,
              title: inputMap['title']?.toString(),
              params:
                  (inputMap['params'] as Map?)?.cast<String, dynamic>() ??
                  const <String, dynamic>{},
              sessionId: inputMap['sessionId']?.toString(),
            );
        await _respond(controller, requestId, method, <String, dynamic>{
          'ok': true,
          'output': <String, dynamic>{'open': true, 'address': address},
        });
        return;
      }
      if (method == 'ui.right-pane.close') {
        ref.read(mobileRightPaneProvider.notifier).close();
        await _respond(controller, requestId, method, <String, dynamic>{
          'ok': true,
          'output': <String, dynamic>{'open': false},
        });
        return;
      }
      if (method == 'ui.action.invoke') {
        final actionId = (inputMap['actionId'] ?? inputMap['action_id'])
            ?.toString();
        final action = actionId == null ? null : widget.actions[actionId];
        dynamic output;
        if (action != null) {
          output = await Future.sync(
            () => action(inputMap['input'] ?? inputMap),
          );
        } else {
          final session = _uiSession;
          final sessionId = (session?['sessionId'] ?? '').toString().trim();
          if (session == null || sessionId.isEmpty || actionId == null) {
            throw StateError(
              'mobile_web_runtime action $actionId is unavailable',
            );
          }
          output = await ref
              .read(extensionServiceProvider)
              .invokeSchemaUIBridge(
                sessionId,
                contributionId:
                    (session['contributionId'] ?? widget.entry.contributionId)
                        .toString(),
                origin: (session['origin'] ?? '').toString(),
                contractVersion:
                    (session['contractVersion'] as num?)?.toInt() ?? 1,
                token: (session['token'] ?? '').toString(),
                generation: (session['generation'] as num?)?.toInt() ?? 0,
                nonce: _nextNonce(),
                actionId: actionId,
                input:
                    (inputMap['input'] as Map?)?.cast<String, dynamic>() ??
                    inputMap,
              );
        }
        await _respond(controller, requestId, method, <String, dynamic>{
          'ok': true,
          'output': output,
        });
        return;
      }
      throw StateError(
        'mobile_web_runtime bridge method $method is unavailable',
      );
    } catch (error) {
      await _respond(controller, requestId, method, <String, dynamic>{
        'ok': false,
        'error': error.toString(),
      });
    }
  }

  Future<void> _respond(
    WebViewController controller,
    String id,
    String method,
    Map<String, dynamic> response,
  ) async {
    final event = <String, dynamic>{
      ...response,
      'type': 'bridge.response',
      'id': id,
      'method': method,
    };
    await controller.runJavaScript(
      'window.dispatchEvent(new CustomEvent('
      "'amitia:native-bridge-response',"
      '{detail:${jsonEncode(event)}}));',
    );
  }

  String _nextNonce() {
    _nonceSequence += 1;
    return '${DateTime.now().microsecondsSinceEpoch}-'
        '${_nonceSequence.toRadixString(36)}-'
        '${identityHashCode(this).toRadixString(36)}';
  }

  Future<void> _revokeUISession([
    Future<void> Function(String sessionId)? revoke,
  ]) async {
    final session = _uiSession;
    _uiSession = null;
    final sessionId = (session?['sessionId'] ?? '').toString().trim();
    if (sessionId.isEmpty) return;
    await (revoke ??
        ref.read(extensionServiceProvider).revokeSchemaUISession)(sessionId);
  }

  @override
  void dispose() {
    final revoke = ref.read(extensionServiceProvider).revokeSchemaUISession;
    unawaited(_revokeUISession(revoke));
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_error != null && widget.fallback != null) return widget.fallback!;
    if (_loading) {
      return const SizedBox(
        height: 48,
        child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
      );
    }
    final controller = _controller;
    if (controller == null) {
      return Text('${_error ?? 'mobile_web_runtime unavailable'}');
    }
    return SizedBox(
      height: _height,
      width: double.infinity,
      child: WebViewWidget(controller: controller),
    );
  }
}
