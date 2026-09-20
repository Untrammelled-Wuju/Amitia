import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../amitia_message_theme.dart';

class AmitiaTerminalBlock extends StatefulWidget {
  final String content;
  final String language;
  final String filename;
  final bool streaming;

  const AmitiaTerminalBlock({
    super.key,
    required this.content,
    this.language = 'shell',
    this.filename = '',
    this.streaming = false,
  });

  @override
  State<AmitiaTerminalBlock> createState() => _AmitiaTerminalBlockState();
}

class _AmitiaTerminalBlockState extends State<AmitiaTerminalBlock> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final lines = widget.content.split('\n');
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(bottom: 17),
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: const Color(0xFF111216),
        borderRadius: BorderRadius.circular(tokens.codeRadius),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _TerminalHeader(
            filename: widget.filename,
            language: widget.language,
            exitCode: _exitCode(widget.content),
            expanded: _expanded,
            onExpand: () => setState(() => _expanded = !_expanded),
            onCopy: _copy,
          ),
          ConstrainedBox(
            constraints: BoxConstraints(
              maxHeight: _expanded ? double.infinity : 260,
            ),
            child: SingleChildScrollView(
              padding: const EdgeInsets.fromLTRB(16, 14, 16, 16),
              child: SelectableText.rich(
                TextSpan(
                  children: [
                    for (final line in lines)
                      TextSpan(
                        text: '${_renderLine(line)}\n',
                        style: TextStyle(
                          color: _lineColor(line),
                          fontFamily: 'monospace',
                          fontSize: tokens.codeFontSize,
                          height: 1.7,
                        ),
                      ),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  String _renderLine(String line) {
    final trimmed = line.trimLeft();
    if (RegExp(r'^(?:\$|>|PS>|❯)\s?').hasMatch(trimmed) ||
        RegExp(r'^(?:pnpm|npm|yarn|flutter|go|git|dart)\b').hasMatch(trimmed)) {
      return line.startsWith(r'$') ? line : r'$ ' + line;
    }
    return line;
  }

  Color _lineColor(String line) {
    final trimmed = line.trimLeft();
    if (RegExp(r'^(?:\$|>|PS>|❯)\s?').hasMatch(trimmed) ||
        RegExp(r'^(?:pnpm|npm|yarn|flutter|go|git|dart)\b').hasMatch(trimmed)) {
      return const Color(0xFF86D39B);
    }
    if (RegExp(
      r'^(?:\[stderr\]|stderr:|error\b|ERR!|✗)',
      caseSensitive: false,
    ).hasMatch(trimmed)) {
      return const Color(0xFFF19494);
    }
    return const Color(0xFFD5D7DD);
  }

  int? _exitCode(String value) {
    final match = RegExp(
      r'exit(?:\s+code)?[:\s]+(-?\d+)',
      caseSensitive: false,
    ).firstMatch(value);
    return match == null ? null : int.tryParse(match.group(1) ?? '');
  }

  Future<void> _copy() async {
    await Clipboard.setData(ClipboardData(text: widget.content));
    if (!mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('已复制终端输出')));
  }
}

class _TerminalHeader extends StatelessWidget {
  final String filename;
  final String language;
  final int? exitCode;
  final bool expanded;
  final VoidCallback onExpand;
  final VoidCallback onCopy;

  const _TerminalHeader({
    required this.filename,
    required this.language,
    required this.exitCode,
    required this.expanded,
    required this.onExpand,
    required this.onCopy,
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
              filename.isEmpty ? 'Terminal' : filename,
              style: const TextStyle(
                color: Color(0xFFD2D3D6),
                fontSize: 11.5,
              ),
            ),
          ),
          Text(
            '$language${exitCode == null ? '' : ' · exit $exitCode'}',
            style: const TextStyle(color: Color(0xFF81838A), fontSize: 10),
          ),
          const SizedBox(width: 6),
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
            tooltip: '复制',
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
