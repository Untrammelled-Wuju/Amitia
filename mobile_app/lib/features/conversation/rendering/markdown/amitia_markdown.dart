import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_markdown_plus/flutter_markdown_plus.dart';
import 'package:flutter_markdown_plus_latex/flutter_markdown_plus_latex.dart';
import 'package:markdown/markdown.dart' as md;
import 'package:url_launcher/url_launcher.dart';

import '../amitia_message_theme.dart';
import '../code/amitia_code_block.dart';
import '../code/amitia_diff_block.dart';
import '../code/amitia_terminal_block.dart';
import '../math/amitia_latex_block.dart';
import '../mermaid/amitia_mermaid_block.dart';
import '../preview/amitia_html_preview.dart';
import 'markdown_parser.dart';

class AmitiaMarkdownView extends StatelessWidget {
  final String source;
  final bool streaming;
  final ValueChanged<String>? onCitation;

  const AmitiaMarkdownView({
    super.key,
    required this.source,
    this.streaming = false,
    this.onCitation,
  });

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final segments = splitAmitiaMarkdown(source);
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (final segment in segments)
          switch (segment.type) {
            AmitiaMarkdownSegmentType.markdown => ConstrainedBox(
              constraints: BoxConstraints(maxWidth: tokens.textMaxWidth),
              child: _MarkdownBody(
                source: segment.content,
                onCitation: onCitation,
              ),
            ),
            AmitiaMarkdownSegmentType.code => AmitiaCodeBlock(
              code: segment.content,
              language: segment.language,
              filename: segment.filename,
              streaming: segment.streaming,
            ),
            AmitiaMarkdownSegmentType.diff => AmitiaDiffBlock(
              diff: segment.content,
              filename: segment.filename,
              streaming: segment.streaming,
            ),
            AmitiaMarkdownSegmentType.terminal => AmitiaTerminalBlock(
              content: segment.content,
              language: segment.language,
              filename: segment.filename,
              streaming: segment.streaming,
            ),
            AmitiaMarkdownSegmentType.mermaid => AmitiaMermaidBlock(
              source: segment.content,
              streaming: segment.streaming,
            ),
            AmitiaMarkdownSegmentType.latex => AmitiaLatexBlock(
              source: segment.content,
              streaming: segment.streaming,
            ),
            AmitiaMarkdownSegmentType.htmlPreview => AmitiaHtmlPreview(
              source: segment.content,
              filename: segment.filename,
              streaming: segment.streaming,
            ),
          },
      ],
    );
  }
}

class _MarkdownBody extends StatelessWidget {
  final String source;
  final ValueChanged<String>? onCitation;

  const _MarkdownBody({
    required this.source,
    this.onCitation,
  });

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return MarkdownBody(
      data: source,
      selectable: true,
      softLineBreak: true,
      styleSheet: MarkdownStyleSheet(
        p: TextStyle(
          color: tokens.text,
          fontSize: 14,
          height: tokens.paragraphLineHeight,
        ),
        pPadding: const EdgeInsets.only(bottom: 11),
        a: TextStyle(color: tokens.accent, decoration: TextDecoration.none),
        code: TextStyle(
          color: tokens.text,
          backgroundColor: tokens.soft,
          fontFamily: 'monospace',
          fontSize: 12.5,
        ),
        h1: TextStyle(
          color: tokens.text,
          fontSize: 22,
          fontWeight: FontWeight.w700,
          height: 1.4,
        ),
        h2: TextStyle(
          color: tokens.text,
          fontSize: 19,
          fontWeight: FontWeight.w700,
          height: 1.4,
        ),
        h3: TextStyle(
          color: tokens.text,
          fontSize: 16,
          fontWeight: FontWeight.w700,
          height: 1.45,
        ),
        h4: TextStyle(
          color: tokens.text,
          fontSize: 14.5,
          fontWeight: FontWeight.w700,
        ),
        h5: TextStyle(
          color: tokens.text,
          fontSize: 13.5,
          fontWeight: FontWeight.w700,
        ),
        h6: TextStyle(
          color: tokens.text,
          fontSize: 13.5,
          fontWeight: FontWeight.w700,
        ),
        h1Padding: EdgeInsets.only(top: tokens.headingSpacing, bottom: 8),
        h2Padding: EdgeInsets.only(top: tokens.headingSpacing, bottom: 8),
        h3Padding: const EdgeInsets.only(top: 19, bottom: 7),
        h4Padding: const EdgeInsets.only(top: 16, bottom: 6),
        h5Padding: const EdgeInsets.only(top: 14, bottom: 6),
        h6Padding: const EdgeInsets.only(top: 14, bottom: 6),
        em: const TextStyle(fontStyle: FontStyle.italic),
        strong: const TextStyle(fontWeight: FontWeight.w700),
        del: const TextStyle(decoration: TextDecoration.lineThrough),
        blockquote: TextStyle(color: tokens.muted, height: 1.65),
        blockquotePadding: const EdgeInsets.fromLTRB(12, 5, 0, 5),
        blockquoteDecoration: BoxDecoration(
          border: Border(
            left: BorderSide(
              color: tokens.accent.withValues(alpha: 0.45),
              width: 2,
            ),
          ),
        ),
        listBullet: TextStyle(color: tokens.text, height: tokens.paragraphLineHeight),
        listBulletPadding: const EdgeInsets.only(right: 6),
        listIndent: 20,
        tableHead: TextStyle(
          color: tokens.muted,
          fontSize: 11,
          fontWeight: FontWeight.w600,
        ),
        tableBody: TextStyle(color: tokens.text, fontSize: 12.5),
        tableBorder: TableBorder.all(color: tokens.line, width: 1),
        tableColumnWidth: const FixedColumnWidth(150),
        tableCellsPadding: tokens.tableCellPadding,
        tableHeadCellsDecoration: BoxDecoration(color: tokens.soft),
        tableScrollbarThumbVisibility: true,
        horizontalRuleDecoration: BoxDecoration(
          border: Border(top: BorderSide(color: tokens.line)),
        ),
      ),
      blockSyntaxes: [LatexBlockSyntax()],
      inlineSyntaxes: [LatexInlineSyntax(), AmitiaCitationSyntax()],
      builders: {
        'latex': LatexElementBuilder(
          textStyle: TextStyle(color: tokens.text, fontSize: 18),
        ),
        'citation': _CitationBuilder(onCitation: onCitation),
      },
      imageBuilder: (uri, title, alt) => _AmitiaMarkdownImage(
        uri: uri,
        alt: alt ?? title ?? '',
      ),
      onTapLink: (text, href, title) {
        final value = href?.trim() ?? '';
        final uri = Uri.tryParse(value);
        if (uri == null) return;
        final scheme = uri.scheme.toLowerCase();
        if (scheme == 'http' ||
            scheme == 'https' ||
            scheme == 'mailto' ||
            scheme == 'amitia') {
          launchUrl(uri, mode: LaunchMode.externalApplication);
        }
      },
    );
  }
}

class AmitiaCitationSyntax extends md.InlineSyntax {
  AmitiaCitationSyntax() : super(r'\[(\d+)\]');

  @override
  bool onMatch(md.InlineParser parser, Match match) {
    parser.addNode(md.Element.text('citation', match.group(1) ?? ''));
    return true;
  }
}

class _CitationBuilder extends MarkdownElementBuilder {
  final ValueChanged<String>? onCitation;

  _CitationBuilder({this.onCitation});

  @override
  Widget visitElementAfterWithContext(
    BuildContext context,
    md.Element element,
    TextStyle? preferredStyle,
    TextStyle? parentStyle,
  ) {
    final id = element.textContent;
    return GestureDetector(
      onTap: () => onCitation?.call(id),
      child: Text(
        '[$id]',
        style: TextStyle(
          color: AmitiaMessageTheme.of(context).accent,
          fontSize: 13,
        ),
      ),
    );
  }
}

class _AmitiaMarkdownImage extends StatelessWidget {
  final Uri uri;
  final String alt;

  const _AmitiaMarkdownImage({
    required this.uri,
    required this.alt,
  });

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final image = ClipRRect(
      borderRadius: BorderRadius.circular(tokens.codeRadius),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 560, maxHeight: 420),
        child: _image(),
      ),
    );
    return GestureDetector(
      onTap: () => showDialog<void>(
        context: context,
        builder: (context) => Dialog.fullscreen(
          backgroundColor: Colors.black87,
          child: Stack(
            children: [
              Positioned.fill(
                child: InteractiveViewer(
                  minScale: 0.7,
                  maxScale: 5,
                  child: Center(child: _image()),
                ),
              ),
              SafeArea(
                child: Align(
                  alignment: Alignment.topRight,
                  child: IconButton(
                    tooltip: '关闭',
                    onPressed: () => Navigator.of(context).pop(),
                    color: Colors.white,
                    icon: const Icon(Icons.close_rounded),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
      child: image,
    );
  }

  Widget _image() {
    if (uri.scheme == 'data') {
      final comma = uri.toString().indexOf(',');
      if (comma >= 0) {
        try {
          final data = base64Decode(uri.toString().substring(comma + 1));
          return Image.memory(data, fit: BoxFit.contain);
        } catch (_) {}
      }
    }
    if (uri.scheme == 'http' || uri.scheme == 'https') {
      return Image.network(
        uri.toString(),
        fit: BoxFit.contain,
        loadingBuilder: (context, child, progress) => progress == null
            ? child
            : const SizedBox(
                height: 160,
                child: Center(child: CircularProgressIndicator()),
              ),
        errorBuilder: (_, _, _) => _ImageError(alt: alt),
      );
    }
    return Image.asset(
      uri.toFilePath(),
      fit: BoxFit.contain,
      errorBuilder: (_, _, _) => _ImageError(alt: alt),
    );
  }
}

class _ImageError extends StatelessWidget {
  final String alt;

  const _ImageError({required this.alt});

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return Container(
      height: 140,
      alignment: Alignment.center,
      color: tokens.soft,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.broken_image_outlined, color: tokens.danger),
          const SizedBox(height: 6),
          Text(
            alt.isEmpty ? '图片加载失败' : alt,
            style: TextStyle(color: tokens.muted, fontSize: 11),
          ),
        ],
      ),
    );
  }
}
