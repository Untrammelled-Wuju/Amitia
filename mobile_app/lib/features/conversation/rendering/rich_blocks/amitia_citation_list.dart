import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

import '../amitia_message_theme.dart';
import '../amrp.dart';

class AmitiaCitationList extends StatefulWidget {
  final List<AmrpCitationSource> sources;
  final String highlightId;

  const AmitiaCitationList({
    super.key,
    required this.sources,
    this.highlightId = '',
  });

  @override
  State<AmitiaCitationList> createState() => _AmitiaCitationListState();
}

class _AmitiaCitationListState extends State<AmitiaCitationList> {
  AmrpCitationSource? _active;

  @override
  void didUpdateWidget(covariant AmitiaCitationList oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.highlightId.isNotEmpty &&
        widget.highlightId != oldWidget.highlightId) {
      _active = widget.sources
          .where((source) => source.id == widget.highlightId)
          .firstOrNull;
    }
  }

  @override
  Widget build(BuildContext context) {
    if (widget.sources.isEmpty) return const SizedBox.shrink();
    final tokens = AmitiaMessageTheme.of(context);
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
      margin: const EdgeInsets.only(top: 17),
      padding: const EdgeInsets.only(top: 12),
      decoration: BoxDecoration(
        border: Border(top: BorderSide(color: tokens.line)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '引用',
            style: TextStyle(color: tokens.muted, fontSize: 11.5),
          ),
          const SizedBox(height: 5),
          Wrap(
            spacing: 5,
            runSpacing: 5,
            children: [
              for (final source in widget.sources)
                ActionChip(
                  visualDensity: VisualDensity.compact,
                  backgroundColor: tokens.soft,
                  side: BorderSide.none,
                  label: Text(
                    '${source.id} · ${source.title}',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  labelStyle: TextStyle(color: tokens.text, fontSize: 11.5),
                  onPressed: () => setState(() => _active = source),
                ),
            ],
          ),
          if (_active != null) ...[
            const SizedBox(height: 9),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: tokens.surface,
                border: Border.all(color: tokens.line),
                borderRadius: BorderRadius.circular(tokens.codeRadius),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '[${_active!.id}] ${_active!.title}',
                    style: TextStyle(
                      color: tokens.text,
                      fontSize: 11.5,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  if (_active!.snippet.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(
                      _active!.snippet,
                      style: TextStyle(color: tokens.muted, fontSize: 10.5),
                    ),
                  ] else if (_active!.url.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(
                      _active!.url,
                      style: TextStyle(color: tokens.muted, fontSize: 10.5),
                    ),
                  ],
                  const SizedBox(height: 7),
                  Wrap(
                    spacing: 6,
                    children: [
                      if (_active!.url.isNotEmpty)
                        TextButton(
                          onPressed: () => _open(_active!),
                          child: const Text('打开来源'),
                        ),
                      TextButton(
                        onPressed: () => _copy(_active!),
                        child: const Text('复制引用'),
                      ),
                      TextButton(
                        onPressed: () => setState(() => _active = null),
                        child: const Text('关闭'),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _open(AmrpCitationSource source) async {
    final uri = Uri.tryParse(source.url);
    if (uri == null) return;
    if (uri.scheme == 'http' ||
        uri.scheme == 'https' ||
        uri.scheme == 'mailto') {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
  }

  Future<void> _copy(AmrpCitationSource source) async {
    await Clipboard.setData(
      ClipboardData(
        text:
            '[${source.id}] ${source.title}'
            '${source.url.isEmpty ? '' : '\n${source.url}'}',
      ),
    );
    if (!mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('已复制引用')));
  }
}

