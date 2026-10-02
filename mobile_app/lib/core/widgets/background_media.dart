import 'dart:async';
import 'dart:io';
import 'dart:ui';
import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';
import '../settings/background_preferences.dart';

class BackgroundMedia extends StatefulWidget {
  final BackgroundPreferences preferences;
  final Color baseColor;
  final bool animate;
  final bool preview;
  final Widget? child;
  const BackgroundMedia({
    super.key,
    required this.preferences,
    required this.baseColor,
    this.animate = true,
    this.preview = false,
    this.child,
  });
  @override
  State<BackgroundMedia> createState() => _BackgroundMediaState();
}

class _BackgroundMediaState extends State<BackgroundMedia>
    with WidgetsBindingObserver {
  VideoPlayerController? _video;
  bool _failed = false;
  bool _foreground =
      WidgetsBinding.instance.lifecycleState == null ||
      WidgetsBinding.instance.lifecycleState == AppLifecycleState.resumed;
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _load();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _syncPlayback();
  }

  @override
  void didUpdateWidget(BackgroundMedia oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.preferences.path != widget.preferences.path ||
        oldWidget.preferences.kind != widget.preferences.kind) {
      _load();
    } else {
      _syncPlayback();
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    _foreground = state == AppLifecycleState.resumed;
    _syncPlayback();
  }

  Future<void> _load() async {
    final previous = _video;
    _video = null;
    _failed = false;
    if (previous != null) {
      previous.removeListener(_videoChanged);
      unawaited(previous.dispose());
    }
    if (widget.preferences.kind != BackgroundMediaKind.video ||
        widget.preferences.path.isEmpty) {
      return;
    }
    final controller = VideoPlayerController.file(
      File(widget.preferences.path),
      videoPlayerOptions: VideoPlayerOptions(mixWithOthers: true),
    );
    _video = controller;
    controller.addListener(_videoChanged);
    try {
      await controller.initialize();
      if (!mounted || _video != controller) return;
      await controller.setVolume(0);
      await controller.setLooping(true);
      if (!mounted || _video != controller) return;
      setState(() {});
      _syncPlayback();
    } catch (_) {
      if (mounted && _video == controller) setState(() => _failed = true);
    }
  }

  void _syncPlayback() {
    final video = _video;
    if (video == null || !video.value.isInitialized) return;
    if (widget.animate &&
        widget.preferences.opacity > 0 &&
        _foreground &&
        !MediaQuery.disableAnimationsOf(context)) {
      unawaited(
        video.play().catchError((Object _) {
          if (mounted && _video == video) setState(() => _failed = true);
        }),
      );
    } else {
      unawaited(video.pause());
    }
  }

  void _videoChanged() {
    final video = _video;
    if (mounted && !_failed && video != null && video.value.hasError) {
      setState(() => _failed = true);
      unawaited(video.pause());
    }
  }

  Widget _imageError(String path) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted && !_failed && widget.preferences.path == path) {
        setState(() => _failed = true);
      }
    });
    return const SizedBox.shrink();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    final video = _video;
    _video = null;
    if (video != null) {
      video.removeListener(_videoChanged);
      unawaited(video.dispose());
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final prefs = widget.preferences;
    final video = _video;
    Widget? media;
    if (!_failed && prefs.path.isNotEmpty) {
      if (prefs.kind == BackgroundMediaKind.image) {
        media = Image.file(
          File(prefs.path),
          fit: BoxFit.cover,
          gaplessPlayback: true,
          errorBuilder: (_, _, _) => _imageError(prefs.path),
        );
      } else if (video != null && video.value.isInitialized) {
        media = FittedBox(
          fit: BoxFit.cover,
          clipBehavior: Clip.hardEdge,
          child: SizedBox(
            width: video.value.size.width,
            height: video.value.size.height,
            child: VideoPlayer(video),
          ),
        );
      }
    }
    return ClipRect(
      child: Stack(
        fit: StackFit.expand,
        children: [
          ColoredBox(color: widget.baseColor),
          if (media != null)
            Positioned.fill(
              left: prefs.blurEnabled ? -prefs.blurRadius * 2 : 0,
              right: prefs.blurEnabled ? -prefs.blurRadius * 2 : 0,
              top: prefs.blurEnabled ? -prefs.blurRadius * 2 : 0,
              bottom: prefs.blurEnabled ? -prefs.blurRadius * 2 : 0,
              child: IgnorePointer(
                child: Opacity(
                  opacity: prefs.opacity,
                  child: ImageFiltered(
                    imageFilter: ImageFilter.blur(
                      sigmaX: prefs.blurEnabled ? prefs.blurRadius : 0,
                      sigmaY: prefs.blurEnabled ? prefs.blurRadius : 0,
                    ),
                    enabled: prefs.blurEnabled && prefs.blurRadius > 0,
                    child: media,
                  ),
                ),
              ),
            ),
          if (_failed && widget.preview)
            Align(
              alignment: Alignment.bottomCenter,
              child: Padding(
                padding: const EdgeInsets.all(8),
                child: Text(
                  prefs.kind == BackgroundMediaKind.video
                      ? '视频无法播放，请更换文件或编码'
                      : '图片无法显示，请更换背景',
                ),
              ),
            ),
          if (widget.child != null) widget.child!,
        ],
      ),
    );
  }
}
