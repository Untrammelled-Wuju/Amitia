import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../amitia_message_theme.dart';

class AmitiaHtmlPreview extends StatefulWidget {
  final String source;
  final String filename;
  final bool streaming;

  const AmitiaHtmlPreview({
    super.key,
    required this.source,
    this.filename = 'index.html',
    this.streaming = false,
  });

  @override
  State<AmitiaHtmlPreview> createState() => _AmitiaHtmlPreviewState();
}

class _AmitiaHtmlPreviewState extends State<AmitiaHtmlPreview> {
  WebViewController? _controller;
  bool _showSource = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_controller == null) {
      _controller = WebViewController()
        ..setJavaScriptMode(JavaScriptMode.unrestricted)
        ..setBackgroundColor(Colors.white)
        ..setNavigationDelegate(
          NavigationDelegate(
            onNavigationRequest: (_) => NavigationDecision.prevent,
          ),
        );
      _load();
    }
  }

  @override
  void didUpdateWidget(covariant AmitiaHtmlPreview oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.source != widget.source ||
        oldWidget.streaming != widget.streaming) {
      _load();
    }
  }

  Future<void> _load() async {
    if (_controller == null || widget.streaming) return;
    await _controller!.loadHtmlString(_sandboxDocument(widget.source));
  }

  String _sandboxDocument(String source) {
    return '''<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data: blob:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; font-src data:; connect-src 'none'; frame-src 'none'; media-src data: blob:;">
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
html,body{min-height:100%;margin:0;background:#fff;color:#19191c;font:14px/1.6 Inter,sans-serif}
body{padding:20px}*{box-sizing:border-box}
</style>
</head>
<body>$source</body>
</html>''';
  }

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(bottom: 16),
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: tokens.surface,
        border: Border.all(color: tokens.line),
        borderRadius: BorderRadius.circular(tokens.codeRadius),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Container(
            height: tokens.codeToolbarHeight,
            padding: const EdgeInsets.symmetric(horizontal: 10),
            decoration: BoxDecoration(
              color: tokens.soft,
              border: Border(bottom: BorderSide(color: tokens.line)),
            ),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    widget.filename,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      color: tokens.text,
                      fontSize: 11.5,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                Text(
                  'sandbox preview',
                  style: TextStyle(color: tokens.muted, fontSize: 10.5),
                ),
                Tooltip(
                  message: _showSource ? '预览' : '源码',
                  child: IconButton(
                    onPressed: () => setState(() => _showSource = !_showSource),
                    iconSize: 16,
                    color: tokens.muted,
                    icon: Icon(
                      _showSource
                          ? Icons.visibility_outlined
                          : Icons.code_rounded,
                    ),
                  ),
                ),
                Tooltip(
                  message: '复制',
                  child: IconButton(
                    onPressed: () => _copy(context),
                    iconSize: 16,
                    color: tokens.muted,
                    icon: const Icon(Icons.copy_rounded),
                  ),
                ),
                Tooltip(
                  message: '全屏',
                  child: IconButton(
                    onPressed: widget.streaming
                        ? null
                        : () => _openFullscreen(context),
                    iconSize: 16,
                    color: tokens.muted,
                    icon: const Icon(Icons.fullscreen_rounded),
                  ),
                ),
              ],
            ),
          ),
          if (_showSource || widget.streaming)
            SingleChildScrollView(
              padding: const EdgeInsets.fromLTRB(16, 14, 16, 16),
              child: SelectableText(
                widget.source,
                style: TextStyle(
                  color: const Color(0xFFD5D7DD),
                  fontFamily: 'monospace',
                  fontSize: tokens.codeFontSize,
                  height: 1.7,
                ),
              ),
            )
          else if (_controller != null)
            SizedBox(height: 190, child: WebViewWidget(controller: _controller!)),
        ],
      ),
    );
  }

  Future<void> _copy(BuildContext context) async {
    await Clipboard.setData(ClipboardData(text: widget.source));
    if (!context.mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('已复制 HTML 源码')));
  }

  Future<void> _openFullscreen(BuildContext context) async {
    final tokens = AmitiaMessageTheme.of(context);
    await showDialog<void>(
      context: context,
      builder: (context) => Dialog.fullscreen(
        backgroundColor: tokens.surface,
        child: SafeArea(
          child: Column(
            children: [
              SizedBox(
                height: tokens.codeToolbarHeight,
                child: Row(
                  children: [
                    const SizedBox(width: 12),
                    const Expanded(child: Text('Sandboxed HTML Preview')),
                    IconButton(
                      tooltip: '复制',
                      onPressed: () => _copy(context),
                      icon: const Icon(Icons.copy_rounded),
                    ),
                    IconButton(
                      tooltip: '关闭',
                      onPressed: () => Navigator.of(context).pop(),
                      icon: const Icon(Icons.close_rounded),
                    ),
                  ],
                ),
              ),
              Expanded(
                child: _controller == null
                    ? const SizedBox()
                    : WebViewWidget(controller: _controller!),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
