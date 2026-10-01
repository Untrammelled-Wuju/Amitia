import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../../backend_connection/backend_connection_availability.dart';
import '../../backend_connection/backend_uri_builder.dart';
import '../../backend_connection/providers/backend_connection_providers.dart';
import '../../services/providers.dart';
import '../ui_provider.dart';

class SandboxWebProviderHost extends ConsumerStatefulWidget {
  const SandboxWebProviderHost({
    super.key,
    required this.provider,
    required this.entry,
    this.context = const {},
    this.actions = const {},
    this.fallback,
    this.onFailure,
    this.inlineComposerAction = false,
  });

  final UIProviderDefinition provider;
  final UIProviderEntry entry;
  final Map<String, dynamic> context;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;
  final Widget? fallback;
  final ValueChanged<Object>? onFailure;
  final bool inlineComposerAction;

  @override
  ConsumerState<SandboxWebProviderHost> createState() =>
      _SandboxWebProviderHostState();
}

class _SandboxWebProviderHostState extends ConsumerState<SandboxWebProviderHost>
    with WidgetsBindingObserver {
  WebViewController? _controller;
  String? _sessionId;
  Object? _error;
  bool _loading = true;
  int _loadToken = 0;
  bool _inlineOverlayOpen = false;
  OverlayEntry? _inlineOverlay;
  Size _inlineOverlaySize = const Size(360, 480);

  void _closeInlineOverlay({bool notifyWeb = false, bool rebuild = true}) {
    _inlineOverlay?.remove();
    _inlineOverlay?.dispose();
    _inlineOverlay = null;
    if (_inlineOverlayOpen && mounted && rebuild) {
      setState(() => _inlineOverlayOpen = false);
    } else {
      _inlineOverlayOpen = false;
    }
    if (notifyWeb && _controller != null) {
      unawaited(
        _controller!
            .runJavaScript(
              'window.dispatchEvent(new CustomEvent('
              '"amitia:host-context", '
              '{detail:{surfaceState:{open:false,dismissToken:Date.now()}}}));',
            )
            .catchError((_) {}),
      );
    }
  }

  void _openInlineOverlay(Size size) {
    _inlineOverlaySize = size;
    if (_inlineOverlayOpen) {
      _inlineOverlay?.markNeedsBuild();
      return;
    }
    setState(() => _inlineOverlayOpen = true);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_inlineOverlayOpen || _controller == null) return;
      final overlay = Overlay.of(context, rootOverlay: true);
      _inlineOverlay = OverlayEntry(
        builder: (overlayContext) {
          final screen = MediaQuery.sizeOf(overlayContext);
          final padding = MediaQuery.paddingOf(overlayContext);
          final anchor = context.findRenderObject();
          final anchorTop = anchor is RenderBox && anchor.hasSize
              ? anchor.localToGlobal(Offset.zero)
              : Offset(screen.width / 2, screen.height);
          final width = _inlineOverlaySize.width
              .clamp(100.0, screen.width - 16)
              .toDouble();
          final height = _inlineOverlaySize.height
              .clamp(100.0, screen.height - padding.top - padding.bottom - 16)
              .toDouble();
          final left = anchorTop.dx
              .clamp(8.0, screen.width - width - 8)
              .toDouble();
          final top = (anchorTop.dy - height - 8)
              .clamp(
                padding.top + 8,
                screen.height - padding.bottom - height - 8,
              )
              .toDouble();
          return Stack(
            children: [
              Positioned.fill(
                child: GestureDetector(
                  behavior: HitTestBehavior.translucent,
                  onTap: () => _closeInlineOverlay(notifyWeb: true),
                ),
              ),
              Positioned(
                left: left,
                top: top,
                width: width,
                height: height,
                child: Material(
                  elevation: 12,
                  borderRadius: BorderRadius.circular(14),
                  clipBehavior: Clip.antiAlias,
                  child: WebViewWidget(controller: _controller!),
                ),
              ),
            ],
          );
        },
      );
      overlay.insert(_inlineOverlay!);
    });
  }

  String _contextValue(String directKey, String nestedKey) {
    final direct = widget.context[directKey];
    if (direct != null && direct.toString().isNotEmpty) {
      return direct.toString();
    }
    final nested = widget.context[nestedKey];
    if (nested is Map) {
      final value = nested['id'];
      if (value != null) return value.toString();
    }
    return '';
  }

  String get _characterId => _contextValue('characterId', 'character');
  String get _conversationId => _contextValue('conversationId', 'conversation');

  String _surfaceRoleFor(Map<String, dynamic> runtimeContext) {
    const roles = <String>{
      'header',
      'status',
      'sidebar',
      'message',
      'composer',
      'main',
      'overlay',
    };
    String normalize(dynamic value) =>
        value?.toString().trim().toLowerCase() ?? '';

    final explicit = normalize(runtimeContext['surfaceRole']);
    if (roles.contains(explicit)) return explicit;
    final surface = runtimeContext['surface'];
    if (surface is Map) {
      final nested = normalize(surface['role']);
      if (roles.contains(nested)) return nested;
    }
    final surfaceText = normalize(surface);
    if (roles.contains(surfaceText)) return surfaceText;
    for (final role in roles) {
      if (surfaceText.startsWith('$role-') ||
          surfaceText.startsWith('$role.')) {
        return role;
      }
    }
    return 'main';
  }

  String get _surfaceRole => _surfaceRoleFor(widget.context);
  String get _scopeIdentity =>
      '${widget.provider.providerId}:${widget.provider.generation}:'
      '$_characterId:$_conversationId:$_surfaceRole';

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _restart();
  }

  @override
  void didChangeMetrics() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        _inlineOverlay?.markNeedsBuild();
        _pushHostState();
      }
    });
  }

  @override
  void didUpdateWidget(covariant SandboxWebProviderHost oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (_scopeFor(oldWidget) != _scopeIdentity) {
      _restart();
    } else {
      _pushHostState();
    }
  }

  String _scopeFor(SandboxWebProviderHost value) {
    String read(String directKey, String nestedKey) {
      final direct = value.context[directKey];
      if (direct != null && direct.toString().isNotEmpty) {
        return direct.toString();
      }
      final nested = value.context[nestedKey];
      if (nested is Map && nested['id'] != null) {
        return nested['id'].toString();
      }
      return '';
    }

    return '${value.provider.providerId}:${value.provider.generation}:'
        '${read('characterId', 'character')}:'
        '${read('conversationId', 'conversation')}:'
        '${_surfaceRoleFor(value.context)}';
  }

  Future<void> _restart() async {
    final token = ++_loadToken;
    await _disposeSession();
    if (!mounted || token != _loadToken) return;
    await _start(token);
  }

  Future<void> _start(int token) async {
    final contributionId = widget.entry.contributionId;
    if (contributionId == null || contributionId.isEmpty) {
      if (!mounted || token != _loadToken) return;
      final error = StateError('Sandbox provider requires contributionId');
      setState(() {
        _loading = false;
        _error = error;
      });
      widget.onFailure?.call(error);
      return;
    }

    if (mounted) {
      setState(() {
        _loading = true;
        _error = null;
        _controller = null;
      });
    }

    String? createdSessionId;
    try {
      final service = ref.read(extensionServiceProvider);
      final session = await service.createWebUISession({
        'contributionId': contributionId,
        'surface': _surfaceRole,
        'surfaceRole': _surfaceRole,
        'host': 'mobile',
        'platform': currentUIPlatform(),
        'characterId': _characterId,
        'conversationId': _conversationId,
        'locale': WidgetsBinding.instance.platformDispatcher.locale
            .toLanguageTag(),
      });

      createdSessionId = session['sessionId']?.toString();
      if (createdSessionId == null || createdSessionId.isEmpty) {
        throw StateError('web UI session did not return sessionId');
      }
      if (!mounted || token != _loadToken) {
        await service.revokeWebUISession(createdSessionId);
        return;
      }
      _sessionId = createdSessionId;

      final rawUrl = (session['resourceUrl'] ?? session['entryUrl'] ?? '')
          .toString();
      if (rawUrl.isEmpty) {
        throw StateError('web UI session did not return resourceUrl');
      }

      final availability = ref.read(backendConnectionProvider).valueOrNull;
      if (availability is! BackendConnectionAvailable) {
        throw StateError('backend connection unavailable');
      }
      final base = BackendUriBuilder().httpBase(availability.config);
      final parsed = Uri.tryParse(rawUrl);
      final resolved = (parsed != null && parsed.hasScheme)
          ? parsed
          : base.resolve(rawUrl);
      final resourcePrefix =
          '/api/extension/webui/resource/${Uri.encodeComponent(createdSessionId)}/';

      late final WebViewController controller;
      controller = WebViewController()
        ..setJavaScriptMode(JavaScriptMode.unrestricted)
        ..addJavaScriptChannel(
          'AmitiaNativeBridge',
          onMessageReceived: (message) async {
            final sid = _sessionId;
            if (sid == null || sid != createdSessionId) return;
            try {
              final decoded = jsonDecode(message.message);
              final payload = decoded is Map<String, dynamic>
                  ? decoded
                  : <String, dynamic>{'payload': decoded};
              Map<String, dynamic> result;
              final method = payload['method']?.toString();
              final input = payload['input'];
              final inputMap = input is Map
                  ? input.cast<String, dynamic>()
                  : const <String, dynamic>{};
              final actionId = (inputMap['actionId'] ?? inputMap['action_id'])
                  ?.toString();
              final localAction =
                  method == 'ui.action.invoke' && actionId != null
                  ? widget.actions[actionId]
                  : null;
              if (localAction != null) {
                try {
                  final value = await Future.sync(
                    () => localAction(inputMap['input'] ?? inputMap),
                  );
                  result = <String, dynamic>{'ok': true, 'output': value};
                } catch (error) {
                  result = <String, dynamic>{
                    'ok': false,
                    'error': error.toString(),
                  };
                }
              } else if (widget.inlineComposerAction &&
                  method == 'ui.resize.request') {
                final width = (inputMap['width'] as num?)?.toDouble() ?? 0;
                final height = (inputMap['height'] as num?)?.toDouble() ?? 0;
                if (width >= 100 && height >= 100) {
                  _openInlineOverlay(Size(width, height));
                } else {
                  _closeInlineOverlay();
                }
                result = <String, dynamic>{
                  'ok': true,
                  'output': <String, dynamic>{
                    'ok': true,
                    'width': width,
                    'height': height,
                  },
                };
              } else {
                result = await service.invokeWebUIBridge(sid, payload);
              }
              final response = <String, dynamic>{
                ...result,
                'type': 'bridge.response',
                'method': payload['method'],
                'id': payload['id'],
              };
              final script =
                  'window.dispatchEvent(new CustomEvent('
                  '"amitia:native-bridge-response", '
                  '{detail:${jsonEncode(response)}}));';
              await controller.runJavaScript(script);
            } catch (_) {
              // Invalid bridge payloads are intentionally ignored. The backend
              // bridge remains fail-closed for all privileged operations.
            }
          },
        )
        ..setNavigationDelegate(
          NavigationDelegate(
            onNavigationRequest: (request) {
              final target = Uri.tryParse(request.url);
              if (target == null) return NavigationDecision.prevent;
              if (target.scheme == 'about' && target.path == 'blank') {
                return NavigationDecision.navigate;
              }
              final sameOrigin =
                  target.scheme == resolved.scheme &&
                  target.host == resolved.host &&
                  target.port == resolved.port;
              final sameSession = target.path.startsWith(resourcePrefix);
              return sameOrigin && sameSession
                  ? NavigationDecision.navigate
                  : NavigationDecision.prevent;
            },
            onPageFinished: (_) async {
              _controller ??= controller;
              await _pushHostState(controller: controller);
            },
          ),
        )
        ..loadRequest(resolved);

      if (!mounted || token != _loadToken) {
        await service.revokeWebUISession(createdSessionId);
        if (_sessionId == createdSessionId) _sessionId = null;
        return;
      }
      setState(() {
        _controller = controller;
        _loading = false;
      });
    } catch (error) {
      if (_sessionId == createdSessionId) {
        await _disposeSession();
      } else if (createdSessionId != null) {
        try {
          await ref
              .read(extensionServiceProvider)
              .revokeWebUISession(createdSessionId);
        } catch (_) {}
      }
      if (!mounted || token != _loadToken) return;
      setState(() {
        _error = error;
        _loading = false;
        _controller = null;
      });
      widget.onFailure?.call(error);
    }
  }

  String _cssColor(Color color) {
    final value = color.toARGB32();
    final rgb = value & 0x00ffffff;
    return '#${rgb.toRadixString(16).padLeft(6, '0')}';
  }

  Future<void> _pushHostState({WebViewController? controller}) async {
    final target = controller ?? _controller;
    if (target == null || !mounted) return;
    final theme = Theme.of(context);
    final size = MediaQuery.sizeOf(context);
    final themePayload = <String, dynamic>{
      'mode': theme.brightness == Brightness.dark ? 'dark' : 'light',
      'tokens': <String, String>{
        '--amitia-color-primary': _cssColor(theme.colorScheme.primary),
        '--amitia-color-accent': _cssColor(theme.colorScheme.primary),
        '--amitia-color-surface': _cssColor(theme.colorScheme.surface),
        '--amitia-color-on-surface': _cssColor(theme.colorScheme.onSurface),
        '--amitia-color-error': _cssColor(theme.colorScheme.error),
      },
    };
    final resizePayload = <String, dynamic>{
      'width': size.width,
      'height': size.height,
      'devicePixelRatio': MediaQuery.devicePixelRatioOf(context),
      'breakpoint': size.width < 600 ? 'xs' : (size.width < 1024 ? 'md' : 'lg'),
      'surfaceRole': _surfaceRole,
    };
    final hostContext = <String, dynamic>{
      ...widget.context,
      'surfaceRole': _surfaceRole,
      if (widget.context['surface'] == null) 'surface': _surfaceRole,
    };
    try {
      await target.runJavaScript('''
        window.dispatchEvent(new CustomEvent('amitia:host-context', {
          detail:${jsonEncode(hostContext)}
        }));
        window.dispatchEvent(new CustomEvent('amitia:host-theme', {
          detail:${jsonEncode(themePayload)}
        }));
        window.dispatchEvent(new CustomEvent('amitia:host-resize', {
          detail:${jsonEncode(resizePayload)}
        }));
      ''');
    } catch (_) {}
  }

  Future<void> _disposeSession() async {
    _closeInlineOverlay();
    final sid = _sessionId;
    _sessionId = null;
    _controller = null;
    if (sid == null) return;
    try {
      await ref.read(extensionServiceProvider).revokeWebUISession(sid);
    } catch (_) {}
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _loadToken++;
    _closeInlineOverlay(rebuild: false);
    _disposeSession();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      final loading = const Center(
        child: CircularProgressIndicator(strokeWidth: 2),
      );
      return widget.inlineComposerAction
          ? SizedBox(width: 44, height: 44, child: loading)
          : loading;
    }
    if (_error != null || _controller == null) {
      final unavailable =
          widget.fallback ??
          Center(
            child: Text(
              'Web UI provider unavailable: ${_error ?? 'unknown error'}',
            ),
          );
      return widget.inlineComposerAction
          ? SizedBox(width: 44, height: 44, child: unavailable)
          : unavailable;
    }
    if (!widget.inlineComposerAction) {
      return WebViewWidget(controller: _controller!);
    }
    return SizedBox(
      width: 44,
      height: 44,
      child: _inlineOverlayOpen
          ? const SizedBox.shrink()
          : WebViewWidget(controller: _controller!),
    );
  }
}
