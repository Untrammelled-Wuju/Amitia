import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_math_fork/flutter_math.dart';

import '../amitia_message_theme.dart';

class AmitiaLatexBlock extends StatelessWidget {
  final String source;
  final bool streaming;

  const AmitiaLatexBlock({
    super.key,
    required this.source,
    this.streaming = false,
  });

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
      child: Stack(
        children: [
          if (streaming)
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 14, 48, 14),
              child: SelectableText(
                source,
                style: TextStyle(
                  color: tokens.muted,
                  fontFamily: 'monospace',
                  fontSize: tokens.codeFontSize,
                  height: 1.6,
                ),
              ),
            )
          else
            SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.fromLTRB(16, 14, 48, 14),
              child: Math.tex(
                source,
                mathStyle: MathStyle.display,
                textStyle: TextStyle(color: tokens.text, fontSize: 18),
                onErrorFallback: (error) => SelectableText(
                  source,
                  style: TextStyle(
                    color: tokens.muted,
                    fontFamily: 'monospace',
                    fontSize: tokens.codeFontSize,
                  ),
                ),
              ),
            ),
          Positioned(
            right: 8,
            top: 8,
            child: Tooltip(
              message: '复制公式',
              child: IconButton(
                onPressed: () => _copy(context),
                iconSize: 16,
                color: tokens.muted,
                icon: const Icon(Icons.copy_rounded),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _copy(BuildContext context) async {
    await Clipboard.setData(ClipboardData(text: source));
    if (!context.mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('已复制公式源码')));
  }
}
