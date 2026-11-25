import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';

class ConversationListScroll extends StatefulWidget {
  const ConversationListScroll({
    super.key,
    required this.child,
    required this.enabled,
    required this.onEnd,
    required this.controller,
    this.loading = false,
    this.error = '',
  });

  final Widget child;
  final bool enabled;
  final VoidCallback onEnd;
  final ScrollController controller;
  final bool loading;
  final String error;

  @override
  State<ConversationListScroll> createState() => _ConversationListScrollState();
}

class _ConversationListScrollState extends State<ConversationListScroll> {
  bool _scheduled = false;

  void _checkAfterLayout() {
    if (_scheduled || !widget.enabled) return;
    _scheduled = true;
    WidgetsBinding.instance.ensureVisualUpdate();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _scheduled = false;
      if (!mounted || !widget.enabled) return;
      final positions = widget.controller.positions;
      if (positions.length != 1) return;
      final position = positions.single;
      if (position.hasContentDimensions && position.extentAfter <= 24) {
        widget.onEnd();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    _checkAfterLayout();
    final list = NotificationListener<ScrollMetricsNotification>(
      onNotification: (_) {
        _checkAfterLayout();
        return false;
      },
      child: NotificationListener<ScrollNotification>(
        onNotification: (notification) {
          if (notification.depth == 0) _checkAfterLayout();
          return false;
        },
        child: widget.child,
      ),
    );
    return Column(
      children: [
        Expanded(child: list),
        if (widget.error.isNotEmpty)
          Flexible(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(8),
              child: SelectableText(widget.error),
            ),
          )
        else if (widget.loading)
          const LinearProgressIndicator(),
      ],
    );
  }
}

class ConversationListEnd extends StatefulWidget {
  const ConversationListEnd({super.key, required this.onEnd});

  final VoidCallback onEnd;

  @override
  State<ConversationListEnd> createState() => _ConversationListEndState();
}

class _ConversationListEndState extends State<ConversationListEnd> {
  ScrollPosition? _position;
  bool _scheduled = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final position = Scrollable.maybeOf(context)?.position;
    if (identical(position, _position)) return;
    _position?.removeListener(_check);
    _position = position;
    position?.addListener(_check);
  }

  @override
  void dispose() {
    _position?.removeListener(_check);
    super.dispose();
  }

  void _check() {
    if (_scheduled) return;
    _scheduled = true;
    WidgetsBinding.instance.ensureVisualUpdate();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _scheduled = false;
      if (!mounted) return;
      final position = _position;
      final target = context.findRenderObject();
      if (position == null ||
          !position.hasContentDimensions ||
          target == null ||
          !target.attached) {
        return;
      }
      final viewport = RenderAbstractViewport.maybeOf(target);
      if (viewport == null) return;
      final offset = viewport.getOffsetToReveal(target, 0).offset;
      if (offset >= position.pixels &&
          offset <= position.pixels + position.viewportDimension) {
        widget.onEnd();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    _check();
    return const SizedBox(height: 1);
  }
}
