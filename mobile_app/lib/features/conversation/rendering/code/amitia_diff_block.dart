import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../amitia_message_theme.dart';

class AmitiaDiffBlock extends StatefulWidget {
  final String diff;
  final String filename;
  final bool streaming;

  const AmitiaDiffBlock({
    super.key,
    required this.diff,
    this.filename = '',
    this.streaming = false,
  });

  @override
  State<AmitiaDiffBlock> createState() => _AmitiaDiffBlockState();
}

class _AmitiaDiffBlockState extends State<AmitiaDiffBlock> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final lines = widget.diff.split('\n');
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
          _DiffHeader(
            filename: widget.filename,
            expanded: _expanded,
            onExpand: () => setState(() => _expanded = !_expanded),
            onFullscreen: _openFullscreen,
            onCopy: _copy,
          ),
          ConstrainedBox(
            constraints: BoxConstraints(
              maxHeight: _expanded ? double.infinity : tokens.codeMaxHeight,
            ),
            child: SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: SingleChildScrollView(
                child: _DiffLines(lines: lines, tokens: tokens),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _copy() async {
    await Clipboard.setData(ClipboardData(text: widget.diff));
    if (!mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('已复制 Diff')));
  }

  Future<void> _openFullscreen() async {
    final tokens = AmitiaMessageTheme.of(context);
    final lines = widget.diff.split('\n');
    await showDialog<void>(
      context: context,
      builder: (context) => Dialog.fullscreen(
        backgroundColor: tokens.codeBackground,
        child: SafeArea(
          child: Column(
            children: [
              _DiffHeader(
                filename: widget.filename,
                expanded: true,
                onExpand: () {},
                onFullscreen: () => Navigator.of(context).pop(),
                onCopy: () => _copy(),
                fullscreenClose: true,
              ),
              Expanded(
                child: SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: SingleChildScrollView(
                    child: _DiffLines(lines: lines, tokens: tokens),
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

enum _DiffLineKind { add, remove, hunk, meta, context }

class _DiffHeader extends StatelessWidget {
  final String filename;
  final bool expanded;
  final VoidCallback onExpand;
  final VoidCallback onFullscreen;
  final VoidCallback onCopy;
  final bool fullscreenClose;

  const _DiffHeader({
    required this.filename,
    required this.expanded,
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
          Expanded(
            child: Text(
              filename.isEmpty ? 'Diff' : filename,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: Color(0xFFD2D3D6), fontSize: 11.5),
            ),
          ),
          IconButton(
            tooltip: expanded ? '收起' : '展开',
            onPressed: onExpand,
            iconSize: 15,
            color: const Color(0xFFC9CBD0),
            visualDensity: VisualDensity.compact,
            padding: EdgeInsets.zero,
            constraints: const BoxConstraints.tightFor(width: 32, height: 32),
            style: IconButton.styleFrom(
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
              minimumSize: const Size(32, 32),
              maximumSize: const Size(32, 32),
            ),
            icon: Icon(
              expanded ? Icons.unfold_less_rounded : Icons.unfold_more_rounded,
            ),
          ),
          IconButton(
            tooltip: fullscreenClose ? '关闭全屏' : '全屏',
            onPressed: onFullscreen,
            iconSize: 15,
            color: const Color(0xFFC9CBD0),
            visualDensity: VisualDensity.compact,
            padding: EdgeInsets.zero,
            constraints: const BoxConstraints.tightFor(width: 32, height: 32),
            style: IconButton.styleFrom(
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
              minimumSize: const Size(32, 32),
              maximumSize: const Size(32, 32),
            ),
            icon: Icon(
              fullscreenClose
                  ? Icons.close_fullscreen_rounded
                  : Icons.fullscreen_rounded,
            ),
          ),
          IconButton(
            tooltip: '复制 Diff',
            onPressed: onCopy,
            iconSize: 15,
            color: const Color(0xFFC9CBD0),
            visualDensity: VisualDensity.compact,
            padding: EdgeInsets.zero,
            constraints: const BoxConstraints.tightFor(width: 32, height: 32),
            style: IconButton.styleFrom(
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
              minimumSize: const Size(32, 32),
              maximumSize: const Size(32, 32),
            ),
            icon: const Icon(Icons.copy_rounded),
          ),
        ],
      ),
    );
  }
}

class _DiffLines extends StatelessWidget {
  final List<String> lines;
  final AmitiaMessageTheme tokens;

  const _DiffLines({required this.lines, required this.tokens});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (var index = 0; index < lines.length; index++)
          _DiffLine(
            index: index + 1,
            kind: _lineKind(lines[index]),
            line: lines[index],
            fontSize: tokens.codeFontSize,
          ),
      ],
    );
  }

  _DiffLineKind _lineKind(String line) {
    if (line.startsWith('@@')) return _DiffLineKind.hunk;
    if (line.startsWith('+++') || line.startsWith('---')) {
      return _DiffLineKind.meta;
    }
    if (line.startsWith('+')) return _DiffLineKind.add;
    if (line.startsWith('-')) return _DiffLineKind.remove;
    return _DiffLineKind.context;
  }
}

class _DiffLine extends StatelessWidget {
  final int index;
  final _DiffLineKind kind;
  final String line;
  final double fontSize;

  const _DiffLine({
    required this.index,
    required this.kind,
    required this.line,
    required this.fontSize,
  });

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final color = switch (kind) {
      _DiffLineKind.add => const Color(0xFF9ADCAA),
      _DiffLineKind.remove => const Color(0xFFF0A0A0),
      _DiffLineKind.hunk => const Color(0xFFBCB3FF),
      _DiffLineKind.meta => const Color(0xFF8C8E95),
      _DiffLineKind.context => const Color(0xFFC9CBD0),
    };
    final background = switch (kind) {
      _DiffLineKind.add => tokens.good.withValues(alpha: 0.22),
      _DiffLineKind.remove => tokens.danger.withValues(alpha: 0.20),
      _DiffLineKind.hunk => tokens.accent.withValues(alpha: 0.16),
      _ => Colors.transparent,
    };
    return Container(
      color: background,
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 44,
            child: Text(
              '$index',
              textAlign: TextAlign.right,
              style: TextStyle(
                color: const Color(0xFF6E7077),
                fontFamily: 'monospace',
                fontSize: fontSize - 1,
                height: 1.7,
              ),
            ),
          ),
          const SizedBox(width: 8),
          SelectableText(
            line,
            style: TextStyle(
              color: color,
              fontFamily: 'monospace',
              fontSize: fontSize,
              height: 1.7,
            ),
          ),
        ],
      ),
    );
  }
}
