import 'package:flutter/material.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../../ui_runtime/ui_provider.dart';

class ExternalWebUIProviderHost extends StatefulWidget {
  final UIProviderDefinition provider;
  final UIProviderEntry entry;
  final Map<String, dynamic> context;
  final Widget? fallback;

  const ExternalWebUIProviderHost({
    super.key,
    required this.provider,
    required this.entry,
    this.context = const {},
    this.fallback,
  });

  @override
  State<ExternalWebUIProviderHost> createState() =>
      _ExternalWebUIProviderHostState();
}

class _ExternalWebUIProviderHostState extends State<ExternalWebUIProviderHost> {
  WebViewController? _controller;
  int _progress = 0;
  String _error = '';

  @override
  void initState() {
    super.initState();
    _start();
  }

  @override
  void didUpdateWidget(covariant ExternalWebUIProviderHost oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.provider.providerId != widget.provider.providerId ||
        oldWidget.provider.generation != widget.provider.generation ||
        oldWidget.entry.url != widget.entry.url) {
      _start();
    }
  }

  void _start() {
    final rawUrl = widget.entry.url?.trim() ?? '';
    final initial = Uri.tryParse(rawUrl);
    if (initial == null || !initial.hasScheme) {
      setState(() => _error = '插件未提供有效的 Web 页面地址');
      return;
    }
    final allowed = widget.entry.allowedOrigins.isEmpty
        ? <String>{_origin(initial)}
        : widget.entry.allowedOrigins
              .map((item) => Uri.tryParse(item))
              .whereType<Uri>()
              .map(_origin)
              .where((item) => item.isNotEmpty)
              .toSet();
    late final WebViewController controller;
    controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setNavigationDelegate(
        NavigationDelegate(
          onNavigationRequest: (request) {
            final uri = Uri.tryParse(request.url);
            if (uri == null || !allowed.contains(_origin(uri))) {
              return NavigationDecision.prevent;
            }
            return NavigationDecision.navigate;
          },
          onPageStarted: (_) {
            if (mounted) setState(() => _progress = 0);
          },
          onProgress: (value) {
            if (mounted) setState(() => _progress = value);
          },
          onPageFinished: (_) {
            if (mounted) setState(() => _progress = 100);
          },
          onWebResourceError: (error) {
            if (error.isForMainFrame != true || !mounted) return;
            setState(() => _error = error.description);
          },
        ),
      )
      ..loadRequest(initial);
    _controller = controller;
  }

  String _origin(Uri uri) {
    if (!uri.hasScheme || uri.host.isEmpty) return '';
    final port = uri.hasPort ? ':${uri.port}' : '';
    return '${uri.scheme}://${uri.host}$port';
  }

  @override
  Widget build(BuildContext context) {
    final controller = _controller;
    if (_error.isNotEmpty || controller == null) {
      return widget.fallback ??
          Center(child: Text(_error.isEmpty ? 'Web 页面不可用' : _error));
    }
    return Stack(
      children: [
        WebViewWidget(controller: controller),
        if (_progress > 0 && _progress < 100)
          LinearProgressIndicator(value: _progress / 100),
      ],
    );
  }
}
