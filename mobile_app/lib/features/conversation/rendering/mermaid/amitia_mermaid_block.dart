import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:mermaid_core/mermaid_core.dart' hide Color, Size;

import '../amitia_message_theme.dart';

String _renderMermaidScene(Map<String, Object> input) {
  final source = input['source']! as String;
  final dark = input['dark']! as bool;
  const measurer = ApproximateTextMeasurer();
  final mermaid = Mermaid(
    measurer: measurer,
    theme: dark ? MermaidTheme.darkTheme : MermaidTheme.defaultTheme,
  );
  return renderSceneToSvg(mermaid.render(source));
}

class AmitiaMermaidBlock extends StatefulWidget {
  final String source;
  final bool streaming;

  const AmitiaMermaidBlock({
    super.key,
    required this.source,
    this.streaming = false,
  });

  @override
  State<AmitiaMermaidBlock> createState() => _AmitiaMermaidBlockState();
}

class _AmitiaMermaidBlockState extends State<AmitiaMermaidBlock> {
  bool _showSource = false;
  Future<String>? _renderFuture;
  bool _dark = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final dark = Theme.of(context).brightness == Brightness.dark;
    if (_dark != dark || _renderFuture == null) {
      _dark = dark;
      _startRender();
    }
  }

  @override
  void didUpdateWidget(covariant AmitiaMermaidBlock oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.source != widget.source ||
        oldWidget.streaming != widget.streaming) {
      _startRender();
    }
  }

  void _startRender() {
    _renderFuture = widget.streaming || widget.source.trim().isEmpty
        ? null
        : compute(
            _renderMermaidScene,
            <String, Object>{'source': widget.source, 'dark': _dark},
          );
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
                Text(
                  'flowchart',
                  style: TextStyle(
                    color: tokens.text,
                    fontSize: 11.5,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(width: 8),
                Text(
                  'Mermaid',
                  style: TextStyle(color: tokens.muted, fontSize: 10.5),
                ),
                const Spacer(),
                Tooltip(
                  message: _showSource ? '预览' : '源码',
                  child: IconButton(
                    onPressed: () => setState(() => _showSource = !_showSource),
                    iconSize: 16,
                    color: tokens.muted,
                    visualDensity: VisualDensity.compact,
                    padding: EdgeInsets.zero,
                    constraints: const BoxConstraints.tightFor(
                      width: 32,
                      height: 32,
                    ),
                    style: IconButton.styleFrom(
                      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      minimumSize: const Size(32, 32),
                      maximumSize: const Size(32, 32),
                    ),
                    icon: Icon(
                      _showSource
                          ? Icons.visibility_outlined
                          : Icons.code_rounded,
                    ),
                  ),
                ),
                Tooltip(
                  message: '复制源码',
                  child: IconButton(
                    onPressed: () => _copySource(context),
                    iconSize: 16,
                    color: tokens.muted,
                    visualDensity: VisualDensity.compact,
                    padding: EdgeInsets.zero,
                    constraints: const BoxConstraints.tightFor(
                      width: 32,
                      height: 32,
                    ),
                    style: IconButton.styleFrom(
                      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      minimumSize: const Size(32, 32),
                      maximumSize: const Size(32, 32),
                    ),
                    icon: const Icon(Icons.copy_rounded),
                  ),
                ),
                Tooltip(
                  message: '全屏',
                  child: IconButton(
                    onPressed: _renderFuture == null
                        ? null
                        : () => _openFullscreen(context),
                    iconSize: 16,
                    color: tokens.muted,
                    visualDensity: VisualDensity.compact,
                    padding: EdgeInsets.zero,
                    constraints: const BoxConstraints.tightFor(
                      width: 32,
                      height: 32,
                    ),
                    style: IconButton.styleFrom(
                      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      minimumSize: const Size(32, 32),
                      maximumSize: const Size(32, 32),
                    ),
                    icon: const Icon(Icons.fullscreen_rounded),
                  ),
                ),
              ],
            ),
          ),
          if (_showSource || widget.streaming)
            Stack(
              children: [
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
                ),
                if (widget.streaming)
                  Positioned(
                    right: 10,
                    bottom: 8,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 6,
                        vertical: 3,
                      ),
                      decoration: BoxDecoration(
                        color: tokens.soft,
                        borderRadius: BorderRadius.circular(5),
                      ),
                      child: Text(
                        '等待代码块闭合后渲染',
                        style: TextStyle(color: tokens.muted, fontSize: 10),
                      ),
                    ),
                  ),
              ],
            )
          else
            _MermaidPreview(future: _renderFuture, tokens: tokens),
        ],
      ),
    );
  }

  Future<void> _copySource(BuildContext context) async {
    await Clipboard.setData(ClipboardData(text: widget.source));
    if (!context.mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('已复制 Mermaid 源码')));
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
                    Text(
                      'Mermaid',
                      style: TextStyle(
                        color: tokens.text,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const Spacer(),
                    IconButton(
                      tooltip: '复制源码',
                      onPressed: () => _copySource(context),
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
                child: _MermaidPreview(
                  future: _renderFuture,
                  tokens: tokens,
                  interactive: true,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _MermaidPreview extends StatelessWidget {
  final Future<String>? future;
  final AmitiaMessageTheme tokens;
  final bool interactive;

  const _MermaidPreview({
    required this.future,
    required this.tokens,
    this.interactive = false,
  });

  @override
  Widget build(BuildContext context) {
    if (future == null) {
      return SizedBox(
        height: 180,
        child: Center(
          child: Text(
            '等待 Mermaid 源码',
            style: TextStyle(color: tokens.muted, fontSize: 11),
          ),
        ),
      );
    }
    return FutureBuilder<String>(
      future: future,
      builder: (context, snapshot) {
        if (snapshot.hasError) {
          return Container(
            padding: const EdgeInsets.all(12),
            color: tokens.danger.withValues(alpha: 0.06),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Mermaid Renderer 发生异常',
                  style: TextStyle(
                    color: tokens.danger,
                    fontSize: 11.5,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  snapshot.error.toString(),
                  style: TextStyle(color: tokens.muted, fontSize: 10.5),
                ),
              ],
            ),
          );
        }
        if (!snapshot.hasData) {
          return const SizedBox(
            height: 170,
            child: Center(child: CircularProgressIndicator()),
          );
        }
        final diagram = InteractiveViewer(
          minScale: 0.7,
          maxScale: 4,
          child: SvgPicture.string(
            snapshot.data!,
            fit: BoxFit.contain,
            placeholderBuilder: (_) => const SizedBox(
              height: 170,
              child: Center(child: CircularProgressIndicator()),
            ),
          ),
        );
        return Container(
          constraints: BoxConstraints(minHeight: interactive ? 0 : 210),
          padding: const EdgeInsets.all(18),
          color: tokens.surface,
          child: interactive ? diagram : SizedBox(height: 210, child: diagram),
        );
      },
    );
  }
}
