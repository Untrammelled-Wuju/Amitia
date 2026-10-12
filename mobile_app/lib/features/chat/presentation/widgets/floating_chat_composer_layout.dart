import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';

class FloatingChatComposerLayout extends StatefulWidget {
  const FloatingChatComposerLayout({
    super.key,
    required this.composer,
    required this.contentBuilder,
    this.onComposerHeightChanged,
  });

  final Widget composer;
  final Widget Function(BuildContext context, double composerHeight)
  contentBuilder;
  final VoidCallback? onComposerHeightChanged;

  @override
  State<FloatingChatComposerLayout> createState() =>
      _FloatingChatComposerLayoutState();
}

class _FloatingChatComposerLayoutState
    extends State<FloatingChatComposerLayout> {
  double _composerHeight = 0;

  void _updateComposerSize(Size size) {
    if (!mounted || _composerHeight == size.height) return;
    setState(() => _composerHeight = size.height);
    widget.onComposerHeightChanged?.call();
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        Positioned.fill(child: widget.contentBuilder(context, _composerHeight)),
        Positioned(
          left: 0,
          right: 0,
          bottom: 0,
          child: _ComposerSizeReporter(
            onSizeChanged: _updateComposerSize,
            child: widget.composer,
          ),
        ),
      ],
    );
  }
}

class _ComposerSizeReporter extends SingleChildRenderObjectWidget {
  const _ComposerSizeReporter({
    required this.onSizeChanged,
    required super.child,
  });

  final ValueChanged<Size> onSizeChanged;

  @override
  RenderObject createRenderObject(BuildContext context) {
    return _ComposerSizeRenderBox(onSizeChanged);
  }

  @override
  void updateRenderObject(
    BuildContext context,
    covariant _ComposerSizeRenderBox renderObject,
  ) {
    renderObject.onSizeChanged = onSizeChanged;
  }
}

class _ComposerSizeRenderBox extends RenderProxyBox {
  _ComposerSizeRenderBox(this.onSizeChanged);

  ValueChanged<Size> onSizeChanged;
  Size? _reportedSize;

  @override
  void performLayout() {
    super.performLayout();
    if (_reportedSize == size) return;
    _reportedSize = size;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (attached) onSizeChanged(size);
    });
  }
}
