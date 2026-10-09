import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../artifact/artifact_providers.dart';

class ConversationMediaImage extends ConsumerStatefulWidget {
  final String url;
  final BoxFit fit;
  final String alt;
  final int? cacheWidth;

  const ConversationMediaImage({
    super.key,
    required this.url,
    this.fit = BoxFit.contain,
    this.alt = '图片加载失败',
    this.cacheWidth,
  });

  @override
  ConsumerState<ConversationMediaImage> createState() =>
      _ConversationMediaImageState();
}

class _ConversationMediaImageState
    extends ConsumerState<ConversationMediaImage> {
  Future<Uri>? _future;
  Object? _service;
  int _retry = 0;

  @override
  void didUpdateWidget(covariant ConversationMediaImage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.url != widget.url) _future = null;
  }

  @override
  Widget build(BuildContext context) {
    final service = ref.watch(artifactServiceProvider);
    if (!identical(_service, service)) {
      _service = service;
      _future = null;
    }
    final raw = Uri.tryParse(widget.url);
    final standalone =
        raw != null &&
        ['data', 'file', 'http', 'https'].contains(raw.scheme) &&
        !raw.path.contains('/api/artifacts/v1/');
    _future ??= standalone
        ? Future.value(raw)
        : ref
              .read(artifactServiceProvider.future)
              .then((service) => service.resolveMediaUri(widget.url));
    return FutureBuilder<Uri>(
      future: _future,
      builder: (context, snapshot) {
        if (snapshot.hasError) return _failure();
        final uri = snapshot.data;
        if (uri == null) {
          return const SizedBox(
            height: 140,
            child: Center(child: CircularProgressIndicator()),
          );
        }
        try {
          if (uri.scheme == 'data') {
            return Image.memory(
              uri.data!.contentAsBytes(),
              fit: widget.fit,
              cacheWidth: widget.cacheWidth,
              errorBuilder: (_, _, _) => _failure(),
            );
          }
          if (uri.scheme == 'file') {
            return Image.file(
              File(uri.toFilePath()),
              fit: widget.fit,
              cacheWidth: widget.cacheWidth,
              errorBuilder: (_, _, _) => _failure(),
            );
          }
          return Image.network(
            uri.toString(),
            key: ValueKey('${uri.toString()}:$_retry'),
            fit: widget.fit,
            cacheWidth: widget.cacheWidth,
            errorBuilder: (_, _, _) => _failure(),
          );
        } catch (_) {
          return _failure();
        }
      },
    );
  }

  Widget _failure() => Center(
    child: TextButton.icon(
      onPressed: () {
        setState(() {
          _retry++;
          _future = null;
        });
      },
      icon: const Icon(Icons.broken_image_outlined),
      label: Text('${widget.alt}，点击重试'),
    ),
  );
}
