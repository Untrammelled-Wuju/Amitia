import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:just_audio/just_audio.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:video_player/video_player.dart';

import '../amitia_message_theme.dart';
import '../amrp.dart';
import '../preview/amitia_html_preview.dart';
import 'renderer_registry.dart';

class AmitiaThinkingBlock extends StatefulWidget {
  final AmrpThinkingBlock block;

  const AmitiaThinkingBlock({super.key, required this.block});

  @override
  State<AmitiaThinkingBlock> createState() => _AmitiaThinkingBlockState();
}

class _AmitiaThinkingBlockState extends State<AmitiaThinkingBlock> {
  bool _expanded = false;

  @override
  void didUpdateWidget(covariant AmitiaThinkingBlock oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (_expanded && widget.block.content.trim().isEmpty) {
      _expanded = false;
    }
  }

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final streaming = widget.block.state == AmrpMessageState.streaming;
    final label = streaming
        ? '思考中'
        : widget.block.duration == null
        ? '思考完成'
        : '思考完成（${(widget.block.duration!.inMilliseconds / 1000).toStringAsFixed(1)} 秒）';
    final hasContent = widget.block.content.trim().isNotEmpty;
    return Padding(
      padding: const EdgeInsets.only(bottom: 13),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Material(
            color: Colors.transparent,
            child: InkWell(
              borderRadius: BorderRadius.circular(8),
              onTap: hasContent
                  ? () => setState(() => _expanded = !_expanded)
                  : null,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 6),
                decoration: BoxDecoration(
                  color: tokens.soft,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (hasContent) ...[
                      AnimatedRotation(
                        turns: _expanded ? 0.25 : 0,
                        duration: const Duration(milliseconds: 150),
                        curve: Curves.easeOut,
                        child: Icon(
                          Icons.chevron_right_rounded,
                          color: tokens.muted,
                          size: 14,
                        ),
                      ),
                      const SizedBox(width: 7),
                    ],
                    Text(
                      label,
                      style: TextStyle(color: tokens.muted, fontSize: 12),
                    ),
                    if (streaming) ...[
                      const SizedBox(width: 7),
                      SizedBox(
                        width: 12,
                        height: 12,
                        child: CircularProgressIndicator(
                          strokeWidth: 1.6,
                          color: tokens.accent,
                          backgroundColor: tokens.accent.withValues(
                            alpha: 0.18,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          ),
          if (_expanded && hasContent)
            Container(
              width: double.infinity,
              constraints: const BoxConstraints(maxWidth: 700),
              margin: const EdgeInsets.only(top: 6),
              padding: const EdgeInsets.fromLTRB(10, 8, 10, 8),
              decoration: BoxDecoration(
                color: tokens.soft,
                border: Border(left: BorderSide(color: tokens.line, width: 2)),
                borderRadius: const BorderRadius.only(
                  topRight: Radius.circular(8),
                  bottomRight: Radius.circular(8),
                ),
              ),
              child: SelectableText(
                widget.block.content,
                style: TextStyle(
                  color: tokens.muted,
                  fontSize: 11.5,
                  height: 1.6,
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class AmitiaToolBlock extends StatefulWidget {
  final AmrpToolBlock block;

  const AmitiaToolBlock({super.key, required this.block});

  @override
  State<AmitiaToolBlock> createState() => _AmitiaToolBlockState();
}

class _AmitiaToolBlockState extends State<AmitiaToolBlock> {
  bool _expanded = false;
  bool _resultExpanded = false;

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final statusColor = switch (widget.block.status) {
      AmrpToolStatus.running => const Color(0xFFD1A24D),
      AmrpToolStatus.failed => const Color(0xFFD46B6B),
      AmrpToolStatus.interrupted => const Color(0xFFA0A1A6),
      AmrpToolStatus.queued => tokens.muted,
      AmrpToolStatus.success => const Color(0xFF77A982),
    };
    final statusLabel = switch (widget.block.status) {
      AmrpToolStatus.queued => '等待',
      AmrpToolStatus.running => '运行中',
      AmrpToolStatus.success => '完成',
      AmrpToolStatus.failed => '失败',
      AmrpToolStatus.interrupted => '已中断',
    };
    final summary = _toolSummary(widget.block);
    final details = _toolDetails(widget.block);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        InkWell(
          borderRadius: BorderRadius.circular(tokens.toolRadius),
          onTap: () => setState(() => _expanded = !_expanded),
          child: Container(
            constraints: const BoxConstraints(maxWidth: 700),
            padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 7),
            decoration: BoxDecoration(
              color: tokens.soft,
              borderRadius: BorderRadius.circular(tokens.toolRadius),
            ),
            child: Row(
              children: [
                Container(
                  width: 7,
                  height: 7,
                  decoration: BoxDecoration(
                    color: statusColor,
                    shape: BoxShape.circle,
                  ),
                ),
                const SizedBox(width: 9),
                Text(
                  widget.block.name,
                  style: TextStyle(
                    color: tokens.text,
                    fontSize: tokens.toolFontSize,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                if (summary.isNotEmpty) ...[
                  const SizedBox(width: 9),
                  Expanded(
                    child: Text(
                      summary,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        color: tokens.muted,
                        fontSize: tokens.toolFontSize,
                      ),
                    ),
                  ),
                ] else
                  const Spacer(),
                const SizedBox(width: 8),
                Text(
                  widget.block.duration == null
                      ? statusLabel
                      : '$statusLabel · ${widget.block.duration!.inMilliseconds} ms',
                  style: TextStyle(
                    color: widget.block.status == AmrpToolStatus.failed
                        ? tokens.danger
                        : tokens.muted,
                    fontSize: 11,
                  ),
                ),
              ],
            ),
          ),
        ),
        if (_expanded)
          Container(
            constraints: const BoxConstraints(maxWidth: 700),
            margin: const EdgeInsets.fromLTRB(16, 0, 0, 9),
            padding: const EdgeInsets.fromLTRB(10, 8, 10, 8),
            decoration: BoxDecoration(
              border: Border(left: BorderSide(color: tokens.line, width: 2)),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (details.isNotEmpty)
                  Container(
                    width: double.infinity,
                    constraints: BoxConstraints(
                      maxHeight: _resultExpanded ? double.infinity : 130,
                    ),
                    clipBehavior: Clip.antiAlias,
                    decoration: BoxDecoration(
                      color: tokens.soft,
                      borderRadius: BorderRadius.circular(7),
                    ),
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.all(9),
                      child: SelectableText(
                        details,
                        style: TextStyle(
                          color: tokens.muted,
                          fontFamily: 'monospace',
                          fontSize: 10.5,
                          height: 1.55,
                        ),
                      ),
                    ),
                  ),
                if (details.length > 1200)
                  TextButton(
                    onPressed: () =>
                        setState(() => _resultExpanded = !_resultExpanded),
                    child: Text(_resultExpanded ? '收起完整结果' : '展开完整结果'),
                  ),
                if (widget.block.error.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 6),
                    child: Text(
                      widget.block.error,
                      style: TextStyle(color: tokens.danger, fontSize: 11),
                    ),
                  ),
                Align(
                  alignment: Alignment.centerRight,
                  child: TextButton.icon(
                    onPressed: () => _copy(context, details),
                    icon: const Icon(Icons.copy_rounded, size: 14),
                    label: const Text('复制详情'),
                  ),
                ),
              ],
            ),
          ),
      ],
    );
  }

  String _toolSummary(AmrpToolBlock block) {
    final arguments = block.arguments;
    if (arguments is Map) {
      for (final key in const <String>['path', 'file', 'query', 'command']) {
        final value = arguments[key]?.toString().trim() ?? '';
        if (value.isNotEmpty) return value;
      }
    }
    return arguments?.toString() ?? '';
  }

  String _toolDetails(AmrpToolBlock block) {
    return [
      if (block.arguments != null) _stringify(block.arguments),
      if (block.result != null) _stringify(block.result),
    ].join('\n\n');
  }

  String _stringify(Object? value) {
    if (value is String) return value;
    const encoder = JsonEncoder.withIndent('  ');
    try {
      return encoder.convert(value);
    } catch (_) {
      return value.toString();
    }
  }

  Future<void> _copy(BuildContext context, String details) async {
    await Clipboard.setData(
      ClipboardData(
        text: [
          widget.block.name,
          details,
          if (widget.block.error.isNotEmpty) widget.block.error,
        ].join('\n\n'),
      ),
    );
    if (!context.mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('已复制工具详情')));
  }
}

class AmitiaFileBlock extends StatefulWidget {
  final AmrpFileBlock block;

  const AmitiaFileBlock({super.key, required this.block});

  @override
  State<AmitiaFileBlock> createState() => _AmitiaFileBlockState();
}

class _AmitiaFileBlockState extends State<AmitiaFileBlock> {
  late AmrpAssetStatus _status = widget.block.status;

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final extension = RegExp(
      r'\.([A-Za-z0-9]+)$',
    ).firstMatch(widget.block.name)?.group(1)?.toUpperCase();
    return Container(
      constraints: const BoxConstraints(maxWidth: 560),
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
      decoration: BoxDecoration(
        color: tokens.surface,
        border: Border.all(color: tokens.line),
        borderRadius: BorderRadius.circular(tokens.codeRadius),
      ),
      child: Row(
        children: [
          Container(
            width: 32,
            height: 36,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: tokens.soft,
              borderRadius: BorderRadius.circular(7),
            ),
            child: Text(
              (extension == null || extension.isEmpty ? 'FILE' : extension)
                  .substring(
                    0,
                    extension == null || extension.length < 4
                        ? extension?.length ?? 4
                        : 4,
                  ),
              style: TextStyle(
                color: tokens.muted,
                fontSize: 10,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  widget.block.name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    color: tokens.text,
                    fontSize: 12.5,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  _status == AmrpAssetStatus.loading
                      ? '加载中'
                      : _status == AmrpAssetStatus.failed
                      ? (widget.block.error.isEmpty
                            ? '加载失败'
                            : widget.block.error)
                      : [
                          _formatBytes(widget.block.size),
                          widget.block.mimeType,
                        ].where((value) => value.isNotEmpty).join(' · '),
                  style: TextStyle(color: tokens.muted, fontSize: 10.5),
                ),
              ],
            ),
          ),
          if (_status == AmrpAssetStatus.failed)
            TextButton(
              onPressed: () => setState(
                () => _status = widget.block.url.trim().isEmpty
                    ? AmrpAssetStatus.failed
                    : AmrpAssetStatus.ready,
              ),
              child: const Text('重试'),
            )
          else if (_status == AmrpAssetStatus.ready)
            TextButton(
              onPressed: () => _open(context),
              child: const Text('打开'),
            ),
        ],
      ),
    );
  }

  String _formatBytes(int value) {
    if (value <= 0) return '';
    if (value >= 1024 * 1024) {
      return '${(value / 1024 / 1024).toStringAsFixed(1)} MB';
    }
    if (value >= 1024) return '${(value / 1024).toStringAsFixed(1)} KB';
    return '$value B';
  }

  Future<void> _open(BuildContext context) async {
    final value = widget.block.url.trim();
    if (value.isEmpty) return;
    final uri = Uri.tryParse(value);
    if (uri == null) return;
    if (uri.scheme == 'http' || uri.scheme == 'https') {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
      return;
    }
    if (uri.scheme == 'file') {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
      return;
    }
    final file = File(value);
    if (await file.exists()) {
      await launchUrl(
        Uri.file(file.path),
        mode: LaunchMode.externalApplication,
      );
    }
  }
}

class AmitiaImageBlock extends StatefulWidget {
  final List<AmrpImageBlock> images;

  const AmitiaImageBlock({super.key, required this.images});

  @override
  State<AmitiaImageBlock> createState() => _AmitiaImageBlockState();
}

class _AmitiaImageBlockState extends State<AmitiaImageBlock> {
  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: widget.images.length > 1 ? 2 : 1,
        mainAxisSpacing: 10,
        crossAxisSpacing: 10,
        childAspectRatio: widget.images.length > 1 ? 1.18 : 1.65,
      ),
      itemCount: widget.images.length,
      itemBuilder: (context, index) => _ImageCard(
        image: widget.images[index],
        onPreview: () => _openPreview(index, tokens),
      ),
    );
  }

  Future<void> _openPreview(int index, AmitiaMessageTheme tokens) async {
    await showDialog<void>(
      context: context,
      builder: (context) => Dialog.fullscreen(
        backgroundColor: Colors.black87,
        child: Stack(
          children: [
            Positioned.fill(
              child: PageView.builder(
                controller: PageController(initialPage: index),
                itemCount: widget.images.length,
                itemBuilder: (context, page) => InteractiveViewer(
                  minScale: 0.7,
                  maxScale: 5,
                  child: Center(child: _image(widget.images[page])),
                ),
              ),
            ),
            SafeArea(
              child: Align(
                alignment: Alignment.topRight,
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    IconButton(
                      tooltip: '保存',
                      onPressed: () => launchUrl(
                        Uri.parse(widget.images[index].url),
                        mode: LaunchMode.externalApplication,
                      ),
                      color: Colors.white,
                      icon: const Icon(Icons.download_rounded),
                    ),
                    IconButton(
                      tooltip: '关闭',
                      onPressed: () => Navigator.of(context).pop(),
                      color: Colors.white,
                      icon: const Icon(Icons.close_rounded),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _image(AmrpImageBlock image) {
    final uri = Uri.tryParse(image.url);
    if (uri == null) {
      return _ImageFailure(alt: image.alt);
    }
    if (uri.scheme == 'http' || uri.scheme == 'https') {
      return Image.network(
        image.url,
        fit: BoxFit.contain,
        errorBuilder: (_, _, _) => _ImageFailure(alt: image.alt),
      );
    }
    return Image.file(
      File(uri.scheme == 'file' ? uri.toFilePath() : image.url),
      fit: BoxFit.contain,
      errorBuilder: (_, _, _) => _ImageFailure(alt: image.alt),
    );
  }
}

class _ImageCard extends StatelessWidget {
  final AmrpImageBlock image;
  final VoidCallback onPreview;

  const _ImageCard({required this.image, required this.onPreview});

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return ClipRRect(
      borderRadius: BorderRadius.circular(tokens.codeRadius),
      child: Material(
        color: tokens.surface,
        child: InkWell(
          onTap: onPreview,
          child: Stack(
            fit: StackFit.expand,
            children: [
              _ImageCardImage(image: image),
              if (image.animated || image.url.toLowerCase().contains('.gif'))
                Positioned(left: 7, top: 7, child: _Badge(label: 'GIF')),
              Positioned(
                right: 7,
                bottom: 7,
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    color: Colors.black54,
                    borderRadius: BorderRadius.circular(5),
                  ),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 6,
                      vertical: 3,
                    ),
                    child: Text(
                      '预览',
                      style: TextStyle(color: Colors.white, fontSize: 9),
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

class _ImageCardImage extends StatelessWidget {
  final AmrpImageBlock image;

  const _ImageCardImage({required this.image});

  @override
  Widget build(BuildContext context) {
    final uri = Uri.tryParse(image.url);
    if (uri == null) return _ImageFailure(alt: image.alt);
    if (uri.scheme == 'http' || uri.scheme == 'https') {
      return Image.network(
        image.url,
        fit: BoxFit.cover,
        loadingBuilder: (context, child, progress) => progress == null
            ? child
            : const Center(child: CircularProgressIndicator()),
        errorBuilder: (_, _, _) => _ImageFailure(alt: image.alt),
      );
    }
    return Image.file(
      File(uri.scheme == 'file' ? uri.toFilePath() : image.url),
      fit: BoxFit.cover,
      errorBuilder: (_, _, _) => _ImageFailure(alt: image.alt),
    );
  }
}

class _ImageFailure extends StatelessWidget {
  final String alt;

  const _ImageFailure({required this.alt});

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return ColoredBox(
      color: tokens.soft,
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.broken_image_outlined, color: tokens.danger),
            if (alt.isNotEmpty) ...[
              const SizedBox(height: 5),
              Text(alt, style: TextStyle(color: tokens.muted, fontSize: 10)),
            ],
          ],
        ),
      ),
    );
  }
}

class AmitiaAudioBlock extends StatefulWidget {
  final AmrpAudioBlock block;

  const AmitiaAudioBlock({super.key, required this.block});

  @override
  State<AmitiaAudioBlock> createState() => _AmitiaAudioBlockState();
}

class _AmitiaAudioBlockState extends State<AmitiaAudioBlock> {
  late final AudioPlayer _player = AudioPlayer();
  String? _error;
  double _speed = 1;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      await _player.setUrl(widget.block.url);
      if (mounted) setState(() => _error = null);
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    }
  }

  @override
  void dispose() {
    _player.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final duration = _player.duration ?? widget.block.duration;
    return Container(
      constraints: const BoxConstraints(maxWidth: 460),
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 9),
      decoration: BoxDecoration(
        color: tokens.surface,
        border: Border.all(color: tokens.line),
        borderRadius: BorderRadius.circular(tokens.codeRadius),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  widget.block.title,
                  style: TextStyle(color: tokens.text, fontSize: 11.5),
                ),
              ),
              Text(
                _formatDuration(duration),
                style: TextStyle(color: tokens.muted, fontSize: 10.5),
              ),
            ],
          ),
          const SizedBox(height: 7),
          if (_error != null)
            Row(
              children: [
                Expanded(
                  child: Text(
                    '音频加载失败',
                    style: TextStyle(color: tokens.danger, fontSize: 11),
                  ),
                ),
                TextButton(onPressed: _load, child: const Text('重试')),
              ],
            )
          else
            Row(
              children: [
                StreamBuilder<PlayerState>(
                  stream: _player.playerStateStream,
                  builder: (context, snapshot) {
                    final playing = snapshot.data?.playing == true;
                    return IconButton.filledTonal(
                      onPressed: () =>
                          playing ? _player.pause() : _player.play(),
                      icon: Icon(
                        playing
                            ? Icons.pause_rounded
                            : Icons.play_arrow_rounded,
                      ),
                    );
                  },
                ),
                Expanded(
                  child: StreamBuilder<Duration>(
                    stream: _player.positionStream,
                    builder: (context, snapshot) {
                      final position = snapshot.data ?? Duration.zero;
                      final total = duration ?? Duration.zero;
                      return Column(
                        children: [
                          Slider(
                            value: total.inMilliseconds == 0
                                ? 0
                                : position.inMilliseconds
                                      .clamp(0, total.inMilliseconds)
                                      .toDouble(),
                            max: total.inMilliseconds == 0
                                ? 1
                                : total.inMilliseconds.toDouble(),
                            onChanged: total.inMilliseconds == 0
                                ? null
                                : (value) => _player.seek(
                                    Duration(milliseconds: value.round()),
                                  ),
                          ),
                          Row(
                            children: [
                              Text(
                                _formatDuration(position),
                                style: TextStyle(
                                  color: tokens.muted,
                                  fontSize: 10,
                                ),
                              ),
                              const Spacer(),
                              Text(
                                _formatDuration(total),
                                style: TextStyle(
                                  color: tokens.muted,
                                  fontSize: 10,
                                ),
                              ),
                            ],
                          ),
                        ],
                      );
                    },
                  ),
                ),
                PopupMenuButton<double>(
                  initialValue: _speed,
                  tooltip: '播放速度',
                  onSelected: (value) {
                    _speed = value;
                    _player.setSpeed(value);
                    setState(() {});
                  },
                  itemBuilder: (context) => const [
                    PopupMenuItem(value: 1, child: Text('1.0×')),
                    PopupMenuItem(value: 1.25, child: Text('1.25×')),
                    PopupMenuItem(value: 1.5, child: Text('1.5×')),
                    PopupMenuItem(value: 2, child: Text('2.0×')),
                  ],
                  child: Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 6),
                    child: Text(
                      '${_speed.toStringAsFixed(_speed == 1 ? 1 : 2)}×',
                      style: TextStyle(color: tokens.text, fontSize: 10),
                    ),
                  ),
                ),
              ],
            ),
        ],
      ),
    );
  }

  String _formatDuration(Duration? value) {
    final duration = value ?? Duration.zero;
    final minutes = duration.inMinutes.remainder(60).toString().padLeft(2, '0');
    final seconds = duration.inSeconds.remainder(60).toString().padLeft(2, '0');
    return '$minutes:$seconds';
  }
}

class AmitiaVideoBlock extends StatefulWidget {
  final AmrpVideoBlock block;

  const AmitiaVideoBlock({super.key, required this.block});

  @override
  State<AmitiaVideoBlock> createState() => _AmitiaVideoBlockState();
}

class _AmitiaVideoBlockState extends State<AmitiaVideoBlock> {
  VideoPlayerController? _controller;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final uri = Uri.parse(widget.block.url);
      final controller = uri.scheme == 'file'
          ? VideoPlayerController.file(File(uri.toFilePath()))
          : VideoPlayerController.networkUrl(uri);
      await controller.initialize();
      if (!mounted) {
        await controller.dispose();
        return;
      }
      setState(() => _controller = controller);
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    }
  }

  @override
  void dispose() {
    _controller?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
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
            color: tokens.soft,
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    widget.block.title,
                    style: TextStyle(color: tokens.text, fontSize: 11.5),
                  ),
                ),
                Text(
                  _controller == null
                      ? ''
                      : _formatDuration(_controller!.value.duration),
                  style: TextStyle(color: tokens.muted, fontSize: 10),
                ),
                IconButton(
                  tooltip: '全屏',
                  onPressed: _controller == null
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
              ],
            ),
          ),
          if (_error != null)
            Row(
              children: [
                Expanded(
                  child: Padding(
                    padding: const EdgeInsets.all(12),
                    child: Text(
                      '视频加载失败',
                      style: TextStyle(color: tokens.danger, fontSize: 11),
                    ),
                  ),
                ),
                TextButton(
                  onPressed: () {
                    setState(() => _error = null);
                    _load();
                  },
                  child: const Text('重试'),
                ),
              ],
            )
          else if (_controller == null)
            const SizedBox(
              height: 220,
              child: Center(child: CircularProgressIndicator()),
            )
          else
            AspectRatio(
              aspectRatio: _controller!.value.aspectRatio,
              child: Stack(
                alignment: Alignment.center,
                children: [
                  VideoPlayer(_controller!),
                  IconButton(
                    iconSize: 46,
                    color: Colors.white,
                    onPressed: () {
                      setState(() {
                        _controller!.value.isPlaying
                            ? _controller!.pause()
                            : _controller!.play();
                      });
                    },
                    icon: Icon(
                      _controller!.value.isPlaying
                          ? Icons.pause_circle_filled
                          : Icons.play_circle_filled,
                    ),
                  ),
                ],
              ),
            ),
          if (_controller != null)
            VideoProgressIndicator(
              _controller!,
              allowScrubbing: true,
              colors: VideoProgressColors(playedColor: tokens.accent),
            ),
        ],
      ),
    );
  }

  String _formatDuration(Duration value) {
    final minutes = value.inMinutes.remainder(60).toString().padLeft(2, '0');
    final seconds = value.inSeconds.remainder(60).toString().padLeft(2, '0');
    return '$minutes:$seconds';
  }

  Future<void> _openFullscreen(BuildContext context) async {
    await showDialog<void>(
      context: context,
      builder: (context) => _FullscreenVideoDialog(
        url: widget.block.url,
        title: widget.block.title,
      ),
    );
  }
}

class _FullscreenVideoDialog extends StatefulWidget {
  final String url;
  final String title;

  const _FullscreenVideoDialog({required this.url, required this.title});

  @override
  State<_FullscreenVideoDialog> createState() => _FullscreenVideoDialogState();
}

class _FullscreenVideoDialogState extends State<_FullscreenVideoDialog> {
  VideoPlayerController? _controller;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final uri = Uri.parse(widget.url);
      final controller = uri.scheme == 'file'
          ? VideoPlayerController.file(File(uri.toFilePath()))
          : VideoPlayerController.networkUrl(uri);
      await controller.initialize();
      await controller.play();
      if (!mounted) {
        await controller.dispose();
        return;
      }
      setState(() => _controller = controller);
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    }
  }

  @override
  void dispose() {
    _controller?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return Dialog.fullscreen(
      backgroundColor: Colors.black,
      child: SafeArea(
        child: Stack(
          children: [
            Positioned.fill(
              child: _error != null
                  ? Center(
                      child: Text(
                        '视频加载失败',
                        style: TextStyle(color: tokens.danger),
                      ),
                    )
                  : _controller == null
                  ? const Center(child: CircularProgressIndicator())
                  : Center(
                      child: AspectRatio(
                        aspectRatio: _controller!.value.aspectRatio,
                        child: VideoPlayer(_controller!),
                      ),
                    ),
            ),
            Positioned(
              right: 12,
              top: 12,
              child: IconButton(
                tooltip: '关闭',
                onPressed: () => Navigator.of(context).pop(),
                color: Colors.white,
                icon: const Icon(Icons.close_rounded),
              ),
            ),
            Positioned(
              left: 16,
              right: 72,
              bottom: 16,
              child: Text(
                widget.title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(color: Colors.white),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class AmitiaAgentTaskBlock extends StatelessWidget {
  final AmrpAgentTaskBlock block;

  const AmitiaAgentTaskBlock({super.key, required this.block});

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
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
                    block.title,
                    style: TextStyle(
                      color: tokens.text,
                      fontSize: 11.5,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                Text(
                  'Agent Task',
                  style: TextStyle(color: tokens.muted, fontSize: 10.5),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              children: [
                ClipRRect(
                  borderRadius: BorderRadius.circular(9),
                  child: LinearProgressIndicator(
                    value: block.progress.clamp(0, 100) / 100,
                    minHeight: 4,
                    color: tokens.accent,
                    backgroundColor: tokens.soft,
                  ),
                ),
                const SizedBox(height: 11),
                for (final step in block.steps)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: Row(
                      children: [
                        Container(
                          width: 20,
                          height: 20,
                          alignment: Alignment.center,
                          decoration: BoxDecoration(
                            color: step.status == AmrpAgentStepStatus.done
                                ? tokens.good.withValues(alpha: 0.16)
                                : step.status == AmrpAgentStepStatus.running
                                ? tokens.accentSoft
                                : tokens.soft,
                            shape: BoxShape.circle,
                          ),
                          child: Text(
                            step.status == AmrpAgentStepStatus.done
                                ? '✓'
                                : '${block.steps.indexOf(step) + 1}',
                            style: TextStyle(
                              color: step.status == AmrpAgentStepStatus.done
                                  ? tokens.good
                                  : step.status == AmrpAgentStepStatus.running
                                  ? tokens.accent
                                  : tokens.muted,
                              fontSize: 9,
                            ),
                          ),
                        ),
                        const SizedBox(width: 9),
                        Expanded(
                          child: Text(
                            step.title,
                            style: TextStyle(color: tokens.text, fontSize: 12),
                          ),
                        ),
                        Text(
                          step.meta.isEmpty
                              ? _statusLabel(step.status)
                              : step.meta,
                          style: TextStyle(color: tokens.muted, fontSize: 10),
                        ),
                      ],
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  String _statusLabel(AmrpAgentStepStatus status) {
    return switch (status) {
      AmrpAgentStepStatus.pending => '等待',
      AmrpAgentStepStatus.running => '进行中',
      AmrpAgentStepStatus.done => '完成',
      AmrpAgentStepStatus.failed => '失败',
    };
  }
}

class AmitiaArtifactBlock extends StatelessWidget {
  final AmrpArtifactBlock block;

  const AmitiaArtifactBlock({super.key, required this.block});

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
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
          Padding(
            padding: const EdgeInsets.all(12),
            child: Row(
              children: [
                Container(
                  width: 38,
                  height: 38,
                  alignment: Alignment.center,
                  decoration: BoxDecoration(
                    color: tokens.accentSoft,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Text(
                    'A',
                    style: TextStyle(
                      color: tokens.accent,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ),
                const SizedBox(width: 11),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        block.title,
                        style: TextStyle(
                          color: tokens.text,
                          fontSize: 12.5,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      Text(
                        block.artifactKind,
                        style: TextStyle(color: tokens.muted, fontSize: 10.5),
                      ),
                    ],
                  ),
                ),
                TextButton(
                  onPressed: () => _showPreview(context),
                  child: const Text('预览'),
                ),
                if (block.url.isNotEmpty)
                  TextButton(
                    onPressed: () => _open(context),
                    child: const Text('打开'),
                  ),
                if (block.url.isNotEmpty)
                  TextButton(
                    onPressed: () => _open(context),
                    child: const Text('保存'),
                  ),
                TextButton(
                  onPressed: () => _copy(context),
                  child: const Text('复制'),
                ),
              ],
            ),
          ),
          if (block.content.isNotEmpty &&
              (block.artifactKind.toLowerCase().contains('html') ||
                  block.mimeType.toLowerCase().contains('html')))
            Padding(
              padding: const EdgeInsets.fromLTRB(10, 0, 10, 10),
              child: AmitiaHtmlPreview(
                source: block.content,
                filename: block.title,
              ),
            ),
        ],
      ),
    );
  }

  Future<void> _showPreview(BuildContext context) async {
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
                    Expanded(
                      child: Text(
                        block.title,
                        style: TextStyle(
                          color: tokens.text,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
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
                child:
                    block.artifactKind.toLowerCase().contains('html') ||
                        block.mimeType.toLowerCase().contains('html')
                    ? AmitiaHtmlPreview(
                        source: block.content,
                        filename: block.title,
                      )
                    : SingleChildScrollView(
                        padding: const EdgeInsets.all(16),
                        child: SelectableText(
                          block.content,
                          style: TextStyle(
                            color: tokens.text,
                            fontFamily: 'monospace',
                            fontSize: 12,
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

  Future<void> _copy(BuildContext context) async {
    await Clipboard.setData(
      ClipboardData(text: block.content.isEmpty ? block.url : block.content),
    );
    if (!context.mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('已复制 Artifact')));
  }

  Future<void> _open(BuildContext context) async {
    final uri = Uri.tryParse(block.url);
    if (uri == null) return;
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  }
}

class AmitiaExtensionBlock extends StatelessWidget {
  final AmrpExtensionBlock block;

  const AmitiaExtensionBlock({super.key, required this.block});

  @override
  Widget build(BuildContext context) {
    final renderer = AmrpRendererRegistry.resolveExtension(block.rendererId);
    if (renderer != null) return renderer(context, block);
    return AmitiaFallbackBlock(
      type: block.rendererId,
      payload: block.payload,
      extension: true,
    );
  }
}

class AmitiaFallbackBlock extends StatelessWidget {
  final String type;
  final Object? payload;
  final bool extension;

  const AmitiaFallbackBlock({
    super.key,
    required this.type,
    this.payload,
    this.extension = false,
  });

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final data = _stringify(payload);
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
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
            color: tokens.soft,
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    extension ? '暂不支持的扩展内容' : '未知 Block',
                    style: TextStyle(
                      color: tokens.text,
                      fontSize: 11.5,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                Text(
                  extension ? 'Fallback Renderer' : type,
                  style: TextStyle(color: tokens.muted, fontSize: 10.5),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  extension
                      ? '当前客户端没有注册 $type Renderer，因此显示安全降级内容。'
                      : '未知类型：$type',
                  style: TextStyle(color: tokens.muted, fontSize: 11.5),
                ),
                const SizedBox(height: 8),
                Container(
                  width: double.infinity,
                  constraints: const BoxConstraints(maxHeight: 260),
                  padding: const EdgeInsets.all(9),
                  decoration: BoxDecoration(
                    color: tokens.soft,
                    borderRadius: BorderRadius.circular(7),
                  ),
                  child: SingleChildScrollView(
                    child: SelectableText(
                      data,
                      style: TextStyle(
                        color: tokens.text,
                        fontFamily: 'monospace',
                        fontSize: 10.5,
                        height: 1.55,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  String _stringify(Object? value) {
    if (value == null) return '{}';
    if (value is String) return value;
    const encoder = JsonEncoder.withIndent('  ');
    try {
      return encoder.convert(value);
    } catch (_) {
      return value.toString();
    }
  }
}

class AmitiaRichBlockRenderer extends StatelessWidget {
  final AmrpRichBlock block;

  const AmitiaRichBlockRenderer({super.key, required this.block});

  @override
  Widget build(BuildContext context) {
    return switch (block) {
      final AmrpToolBlock typed => AmitiaToolBlock(block: typed),
      final AmrpFileBlock typed => AmitiaFileBlock(block: typed),
      final AmrpImageBlock typed => AmitiaImageBlock(images: [typed]),
      final AmrpAudioBlock typed => AmitiaAudioBlock(block: typed),
      final AmrpVideoBlock typed => AmitiaVideoBlock(block: typed),
      final AmrpArtifactBlock typed => AmitiaArtifactBlock(block: typed),
      final AmrpAgentTaskBlock typed => AmitiaAgentTaskBlock(block: typed),
      final AmrpExtensionBlock typed => AmitiaExtensionBlock(block: typed),
      final AmrpUnknownBlock typed => AmitiaFallbackBlock(
        type: typed.type,
        payload: typed.payload,
      ),
    };
  }
}

class _Badge extends StatelessWidget {
  final String label;

  const _Badge({required this.label});

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: Colors.black54,
        borderRadius: BorderRadius.circular(4),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
        child: Text(
          label,
          style: const TextStyle(color: Colors.white, fontSize: 9),
        ),
      ),
    );
  }
}
