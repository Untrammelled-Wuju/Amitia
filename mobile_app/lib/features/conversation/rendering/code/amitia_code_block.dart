import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:syntax_highlighter_plus/syntax_highlighter_plus.dart';

import '../amitia_message_theme.dart';

class AmitiaCodeBlock extends StatefulWidget {
  final String code;
  final String language;
  final String filename;
  final bool streaming;

  const AmitiaCodeBlock({
    super.key,
    required this.code,
    this.language = 'text',
    this.filename = '',
    this.streaming = false,
  });

  @override
  State<AmitiaCodeBlock> createState() => _AmitiaCodeBlockState();
}

class _AmitiaCodeBlockState extends State<AmitiaCodeBlock> {
  bool _wrap = false;
  bool _expanded = false;
  String _themeName = 'github-light';
  Future<TextSpan?>? _highlightFuture;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final dark = Theme.of(context).brightness == Brightness.dark;
    final themeName = dark ? 'github-dark' : 'github-light';
    if (_themeName != themeName) {
      _themeName = themeName;
      _loadHighlight();
    } else {
      _highlightFuture ??= _loadHighlight();
    }
  }

  @override
  void didUpdateWidget(covariant AmitiaCodeBlock oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.code != widget.code ||
        oldWidget.language != widget.language ||
        oldWidget.streaming != widget.streaming) {
      _loadHighlight();
    }
  }

  Future<TextSpan?> _loadHighlight() async {
    if (widget.streaming ||
        Platform.isWindows ||
        widget.code.length > 240000 ||
        widget.code.split('\n').length > 4000) {
      return null;
    }
    final language = widget.language.trim().toLowerCase();
    final resolved = switch (language) {
      '' => 'text',
      'js' => 'javascript',
      'ts' => 'typescript',
      'py' => 'python',
      'bash' || 'sh' || 'shell' => 'shellscript',
      'yml' => 'yaml',
      'md' => 'markdown',
      _ => language,
    };
    try {
      return SyntaxHighlighterPlus(
        theme: _themeName,
      ).highlight(resolved, widget.code);
    } catch (_) {
      return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(bottom: 17),
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: tokens.codeBackground,
        borderRadius: BorderRadius.circular(tokens.codeRadius),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _CodeHeader(
            filename: widget.filename,
            language: widget.language,
            wrap: _wrap,
            expanded: _expanded,
            onWrap: () => setState(() => _wrap = !_wrap),
            onExpand: () => setState(() => _expanded = !_expanded),
            onFullscreen: _openFullscreen,
            onCopy: _copy,
          ),
          ConstrainedBox(
            constraints: BoxConstraints(
              maxHeight: _expanded ? double.infinity : tokens.codeMaxHeight,
            ),
            child: ClipRect(
              child: _wrap
                  ? _HighlightedCode(
                      future: _highlightFuture,
                      code: widget.code,
                      wrap: true,
                      fontSize: tokens.codeFontSize,
                    )
                  : SingleChildScrollView(
                      scrollDirection: Axis.horizontal,
                      child: _HighlightedCode(
                        future: _highlightFuture,
                        code: widget.code,
                        wrap: false,
                        fontSize: tokens.codeFontSize,
                      ),
                    ),
            ),
          ),
          if (!_expanded &&
              (widget.code.split('\n').length > 24 ||
                  widget.code.length > 1800))
            InkWell(
              onTap: () => setState(() => _expanded = true),
              child: Container(
                height: 30,
                alignment: Alignment.center,
                color: tokens.codeHeader,
                child: Text(
                  '显示更多代码',
                  style: TextStyle(color: tokens.muted, fontSize: 10),
                ),
              ),
            ),
        ],
      ),
    );
  }

  Future<void> _copy() async {
    await Clipboard.setData(ClipboardData(text: widget.code));
    if (!mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('已复制代码')));
  }

  Future<void> _openFullscreen() async {
    final tokens = AmitiaMessageTheme.of(context);
    await showDialog<void>(
      context: context,
      builder: (context) => Dialog.fullscreen(
        backgroundColor: tokens.codeBackground,
        child: SafeArea(
          child: Column(
            children: [
              _CodeHeader(
                filename: widget.filename,
                language: widget.language,
                wrap: _wrap,
                expanded: true,
                onWrap: () => setState(() => _wrap = !_wrap),
                onExpand: () {},
                onFullscreen: () => Navigator.of(context).pop(),
                onCopy: _copy,
                fullscreenClose: true,
              ),
              Expanded(
                child: SingleChildScrollView(
                  scrollDirection: _wrap ? Axis.vertical : Axis.horizontal,
                  child: SingleChildScrollView(
                    child: _HighlightedCode(
                      future: _highlightFuture,
                      code: widget.code,
                      wrap: _wrap,
                      fontSize: tokens.codeFontSize,
                    ),
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

class _CodeHeader extends StatelessWidget {
  final String filename;
  final String language;
  final bool wrap;
  final bool expanded;
  final bool fullscreenClose;
  final VoidCallback onWrap;
  final VoidCallback onExpand;
  final VoidCallback onFullscreen;
  final VoidCallback onCopy;

  const _CodeHeader({
    required this.filename,
    required this.language,
    required this.wrap,
    required this.expanded,
    required this.onWrap,
    required this.onExpand,
    required this.onFullscreen,
    required this.onCopy,
    this.fullscreenClose = false,
  });

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return Container(
      height: tokens.codeToolbarHeight,
      padding: const EdgeInsets.symmetric(horizontal: 11),
      color: tokens.codeHeader,
      child: Row(
        children: [
          if (filename.isNotEmpty)
            Flexible(
              child: Text(
                filename,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  color: Color(0xFFD2D3D6),
                  fontSize: 11.5,
                ),
              ),
            ),
          if (filename.isNotEmpty) const SizedBox(width: 8),
          Text(
            language.isEmpty ? 'text' : language,
            style: const TextStyle(color: Color(0xFF81838A), fontSize: 10),
          ),
          const Spacer(),
          _HeaderAction(
            icon: wrap ? Icons.notes_rounded : Icons.wrap_text_rounded,
            tooltip: wrap ? '不换行' : '换行',
            onTap: onWrap,
          ),
          if (!fullscreenClose)
            _HeaderAction(
              icon: expanded
                  ? Icons.unfold_less_rounded
                  : Icons.unfold_more_rounded,
              tooltip: expanded ? '收起' : '展开',
              onTap: onExpand,
            ),
          _HeaderAction(
            icon: fullscreenClose
                ? Icons.close_fullscreen_rounded
                : Icons.fullscreen_rounded,
            tooltip: fullscreenClose ? '关闭全屏' : '全屏',
            onTap: onFullscreen,
          ),
          _HeaderAction(
            icon: Icons.copy_rounded,
            tooltip: '复制代码',
            onTap: onCopy,
          ),
        ],
      ),
    );
  }
}

class _HeaderAction extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final VoidCallback onTap;

  const _HeaderAction({
    required this.icon,
    required this.tooltip,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: InkWell(
        borderRadius: BorderRadius.circular(6),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(6),
          child: Icon(icon, size: 14, color: const Color(0xFFC9CBD0)),
        ),
      ),
    );
  }
}

class _HighlightedCode extends StatelessWidget {
  final Future<TextSpan?>? future;
  final String code;
  final bool wrap;
  final double fontSize;

  const _HighlightedCode({
    required this.future,
    required this.code,
    required this.wrap,
    required this.fontSize,
  });

  @override
  Widget build(BuildContext context) {
    final baseStyle = TextStyle(
      color: const Color(0xFFE8E8EB),
      fontFamily: 'monospace',
      fontSize: fontSize,
      height: 1.7,
    );
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 14, 16, 16),
      child: FutureBuilder<TextSpan?>(
        future: future,
        builder: (context, snapshot) {
          final span = snapshot.data ?? TextSpan(text: code, style: baseStyle);
          return SelectableText.rich(
            span,
            style: baseStyle,
            textWidthBasis: wrap
                ? TextWidthBasis.parent
                : TextWidthBasis.longestLine,
          );
        },
      ),
    );
  }
}
