import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../amitia_message_theme.dart';
import '../../../../core/artifact/artifact_providers.dart';
import 'html_document.dart';

class AmitiaHtmlPreview extends StatefulWidget {
  final String source;
  final String filename;
  final bool streaming;
  final String url;
  final double height;

  const AmitiaHtmlPreview({
    super.key,
    required this.source,
    this.filename = 'index.html',
    this.streaming = false,
    this.url = '',
    this.height = 190,
  });

  @override
  State<AmitiaHtmlPreview> createState() => _AmitiaHtmlPreviewState();
}

class _AmitiaHtmlPreviewState extends State<AmitiaHtmlPreview> {
  WebViewController? _controller;
  bool _showSource = false;
  String _source = '';
  bool _loading = false;
  String? _error;
  int _generation = 0;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_controller == null) {
      _controller = WebViewController()
        ..setJavaScriptMode(JavaScriptMode.unrestricted)
        ..setBackgroundColor(Colors.white)
        ..setNavigationDelegate(
          NavigationDelegate(
            onNavigationRequest: (request) => request.isMainFrame
                ? NavigationDecision.prevent
                : NavigationDecision.navigate,
          ),
        );
      _load();
    }
  }

  @override
  void didUpdateWidget(covariant AmitiaHtmlPreview oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.source != widget.source ||
        oldWidget.streaming != widget.streaming ||
        oldWidget.url != widget.url) {
      _load();
    }
  }

  Future<void> _load() async {
    final generation = ++_generation;
    if (_controller == null || widget.streaming) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      var source = widget.source;
      var baseUrl = widget.url;
      if (source.isEmpty && widget.url.isNotEmpty) {
        final service = await ProviderScope.containerOf(
          context,
          listen: false,
        ).read(artifactServiceProvider.future);
        source = await service.readText(widget.url);
        baseUrl = (await service.resolveMediaUri(widget.url)).toString();
      }
      if (!mounted || generation != _generation) return;
      final document = await buildHtmlPreviewDocument(
        source,
        baseUrl: baseUrl,
        resolveMedia: (raw) async {
          final service = await ProviderScope.containerOf(
            context,
            listen: false,
          ).read(artifactServiceProvider.future);
          return service.resolveMediaUri(raw);
        },
        embedMedia: (raw) async {
          final service = await ProviderScope.containerOf(
            context,
            listen: false,
          ).read(artifactServiceProvider.future);
          return service.resolveMediaUri(raw, embed: true);
        },
      );
      if (!mounted || generation != _generation) return;
      _source = source;
      final escaped = const HtmlEscape(
        HtmlEscapeMode.attribute,
      ).convert(document);
      await _controller!.loadHtmlString(
        '<!doctype html><html><body style="margin:0"><iframe sandbox="allow-scripts" referrerpolicy="no-referrer" style="border:0;width:100%;height:100vh" srcdoc="$escaped"></iframe></body></html>',
      );
    } catch (error) {
      if (mounted && generation == _generation) _error = 'HTML 加载失败：$error';
    } finally {
      if (mounted && generation == _generation) {
        setState(() {
          _loading = false;
        });
      }
    }
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
                widget.streaming ? widget.source : _source,
                style: TextStyle(
                  color: const Color(0xFFD5D7DD),
                  fontFamily: 'monospace',
                  fontSize: tokens.codeFontSize,
                  height: 1.7,
                ),
              ),
            )
          else if (_loading)
            const SizedBox(
              height: 190,
              child: Center(child: CircularProgressIndicator()),
            )
          else if (_error != null)
            TextButton(onPressed: _load, child: Text('$_error，点击重试'))
          else if (_controller != null)
            SizedBox(
              height: widget.height,
              child: WebViewWidget(controller: _controller!),
            ),
        ],
      ),
    );
  }

  Future<void> _copy(BuildContext context) async {
    await Clipboard.setData(
      ClipboardData(text: widget.streaming ? widget.source : _source),
    );
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
                child: SingleChildScrollView(
                  child: AmitiaHtmlPreview(
                    source: _source,
                    filename: widget.filename,
                    url: widget.url,
                    height: MediaQuery.sizeOf(context).height - 140,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
