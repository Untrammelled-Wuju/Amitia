class AmitiaMarkdownSegment {
  final String id;
  final AmitiaMarkdownSegmentType type;
  final String content;
  final String language;
  final String filename;
  final bool streaming;

  const AmitiaMarkdownSegment({
    required this.id,
    required this.type,
    required this.content,
    this.language = '',
    this.filename = '',
    this.streaming = false,
  });
}

enum AmitiaMarkdownSegmentType {
  markdown,
  code,
  diff,
  terminal,
  mermaid,
  latex,
  htmlPreview,
}

List<AmitiaMarkdownSegment> splitAmitiaMarkdown(String source) {
  final normalized = source.replaceAll('\r\n', '\n').replaceAll('\r', '\n');
  final lines = normalized.split('\n');
  final segments = <AmitiaMarkdownSegment>[];
  final markdownLines = <String>[];
  var segmentIndex = 0;

  void flushMarkdown() {
    if (markdownLines.isEmpty) return;
    final content = markdownLines.join('\n');
    if (content.trim().isNotEmpty) {
      segments.add(
        AmitiaMarkdownSegment(
          id: 'md:${segmentIndex++}',
          type: AmitiaMarkdownSegmentType.markdown,
          content: content,
        ),
      );
    }
    markdownLines.clear();
  }

  for (var index = 0; index < lines.length; index++) {
    final line = lines[index];
    final fence = RegExp(
      r'^\s*(`{3,}|~{3,})\s*([^\s`]*)?\s*(.*?)\s*$',
    ).firstMatch(line);
    if (fence != null) {
      flushMarkdown();
      final marker = fence.group(1)!;
      final language = (fence.group(2) ?? '').trim();
      final meta = (fence.group(3) ?? '').trim();
      final body = <String>[];
      var closed = false;
      final closePattern = RegExp(
        '^\\s*${RegExp.escape(marker[0])}{${marker.length},}\\s*\$',
      );
      while (index + 1 < lines.length) {
        index++;
        if (closePattern.hasMatch(lines[index])) {
          closed = true;
          break;
        }
        body.add(lines[index]);
      }
      final filenameMatch = RegExp(
        r'(?:filename|file|title)=("[^"]+"|'
        r"'[^']+'|\S+)",
        caseSensitive: false,
      ).firstMatch(meta);
      final filename = (filenameMatch?.group(1) ?? '').replaceAll(
        RegExp(r'''^["']|["']$'''),
        '',
      );
      segments.add(
        _fenceSegment(
          id: 'fence:${segmentIndex++}',
          language: language,
          filename: filename,
          content: body.join('\n'),
          streaming: !closed,
        ),
      );
      continue;
    }

    final blockMath = RegExp(r'^\s*(\$\$|\\\[)\s*$').firstMatch(line);
    if (blockMath != null) {
      flushMarkdown();
      final marker = blockMath.group(1)!;
      final closePattern = marker == r'$$'
          ? RegExp(r'^\s*\$\$\s*$')
          : RegExp(r'^\s*\\\]\s*$');
      final body = <String>[];
      var closed = false;
      while (index + 1 < lines.length) {
        index++;
        if (closePattern.hasMatch(lines[index])) {
          closed = true;
          break;
        }
        body.add(lines[index]);
      }
      segments.add(
        AmitiaMarkdownSegment(
          id: 'latex:${segmentIndex++}',
          type: AmitiaMarkdownSegmentType.latex,
          content: body.join('\n'),
          language: 'latex',
          streaming: !closed,
        ),
      );
      continue;
    }

    markdownLines.add(line);
  }
  flushMarkdown();
  return segments;
}

AmitiaMarkdownSegment _fenceSegment({
  required String id,
  required String language,
  required String filename,
  required String content,
  required bool streaming,
}) {
  final normalized = language.trim().toLowerCase();
  if (normalized == 'diff' || normalized == 'patch') {
    return AmitiaMarkdownSegment(
      id: id,
      type: AmitiaMarkdownSegmentType.diff,
      content: content,
      language: 'diff',
      filename: filename,
      streaming: streaming,
    );
  }
  const terminals = <String>{
    'terminal',
    'console',
    'shell',
    'bash',
    'sh',
    'powershell',
    'cmd',
  };
  if (terminals.contains(normalized)) {
    return AmitiaMarkdownSegment(
      id: id,
      type: AmitiaMarkdownSegmentType.terminal,
      content: content,
      language: normalized,
      filename: filename,
      streaming: streaming,
    );
  }
  if (normalized == 'mermaid') {
    return AmitiaMarkdownSegment(
      id: id,
      type: AmitiaMarkdownSegmentType.mermaid,
      content: content,
      language: 'mermaid',
      filename: filename,
      streaming: streaming,
    );
  }
  if (normalized == 'html-preview') {
    return AmitiaMarkdownSegment(
      id: id,
      type: AmitiaMarkdownSegmentType.htmlPreview,
      content: content,
      language: 'html',
      filename: filename,
      streaming: streaming,
    );
  }
  if (normalized == 'latex' || normalized == 'tex' || normalized == 'math') {
    return AmitiaMarkdownSegment(
      id: id,
      type: AmitiaMarkdownSegmentType.latex,
      content: content,
      language: normalized,
      filename: filename,
      streaming: streaming,
    );
  }
  return AmitiaMarkdownSegment(
    id: id,
    type: AmitiaMarkdownSegmentType.code,
    content: content,
    language: normalized.isEmpty ? 'text' : normalized,
    filename: filename,
    streaming: streaming,
  );
}
