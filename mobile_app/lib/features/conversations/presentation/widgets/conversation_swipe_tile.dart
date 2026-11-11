import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';

class ConversationSwipeTile extends StatefulWidget {
  const ConversationSwipeTile({
    super.key,
    required this.child,
    required this.pinned,
    required this.readable,
    required this.onPin,
    required this.onArchive,
    required this.onMarkRead,
    this.enabled = true,
  });

  final Widget child;
  final bool pinned;
  final bool readable;
  final bool enabled;
  final Future<void> Function() onPin;
  final Future<void> Function() onArchive;
  final Future<void> Function() onMarkRead;

  @override
  State<ConversationSwipeTile> createState() => _ConversationSwipeTileState();
}

class _ConversationSwipeTileState extends State<ConversationSwipeTile> {
  static const _actionExtent = 72.0;
  static const _actionCount = 3;
  double _offset = 0;
  bool _busy = false;
  bool _dragging = false;

  double get _maxOffset => _actionExtent * _actionCount;

  @override
  void didUpdateWidget(covariant ConversationSwipeTile oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!widget.enabled && oldWidget.enabled) {
      _offset = 0;
      _dragging = false;
    }
  }

  void _handleDragUpdate(DragUpdateDetails details) {
    if (!widget.enabled || _busy) return;
    setState(() {
      _offset = (_offset + details.delta.dx).clamp(-_maxOffset, 0);
    });
  }

  void _handleDragEnd(DragEndDetails details) {
    if (!widget.enabled || _busy) return;
    final velocity = details.primaryVelocity ?? 0;
    final shouldOpen = velocity.abs() > 250
        ? velocity < 0
        : _offset.abs() > _maxOffset / 2;
    setState(() {
      _dragging = false;
      _offset = shouldOpen ? -_maxOffset : 0;
    });
  }

  Future<void> _run(Future<void> Function() action) async {
    if (_busy || !widget.enabled) return;
    setState(() => _busy = true);
    try {
      await action();
    } finally {
      if (mounted) {
        setState(() {
          _busy = false;
          _offset = 0;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final open = _offset.abs() > 1;
    return ClipRect(
      child: LayoutBuilder(
        builder: (context, constraints) => GestureDetector(
          behavior: HitTestBehavior.translucent,
          onHorizontalDragStart: (_) {
            if (widget.enabled && !_busy) setState(() => _dragging = true);
          },
          onHorizontalDragUpdate: _handleDragUpdate,
          onHorizontalDragEnd: _handleDragEnd,
          onHorizontalDragCancel: () {
            if (_dragging)
              setState(() {
                _dragging = false;
                _offset = _offset.abs() > _maxOffset / 2 ? -_maxOffset : 0;
              });
          },
          child: Stack(
            children: [
              AnimatedPositioned(
                duration: _busy || _dragging
                    ? Duration.zero
                    : const Duration(milliseconds: 170),
                curve: Curves.easeOutCubic,
                left: constraints.maxWidth + _offset,
                top: 0,
                bottom: 0,
                width: _maxOffset,
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    _SwipeAction(
                      label: widget.pinned ? '取消置顶' : '置顶',
                      icon: widget.pinned
                          ? Icons.push_pin
                          : Icons.push_pin_outlined,
                      color: context.accentPrimary,
                      enabled: !_busy,
                      onTap: () => _run(widget.onPin),
                    ),
                    _SwipeAction(
                      label: '归档',
                      icon: Icons.archive_outlined,
                      color: context.warning,
                      enabled: !_busy,
                      onTap: () => _run(widget.onArchive),
                    ),
                    _SwipeAction(
                      label: '标为已读',
                      icon: Icons.mark_chat_read_outlined,
                      color: context.success,
                      enabled: !_busy && widget.readable,
                      unavailableReason: _busy ? '正在处理，请稍后重试。' : '当前对话无法标记为已读。',
                      onTap: () => _run(widget.onMarkRead),
                    ),
                  ],
                ),
              ),
              AnimatedContainer(
                width: double.infinity,
                duration: _busy || _dragging
                    ? Duration.zero
                    : const Duration(milliseconds: 170),
                curve: Curves.easeOutCubic,
                transform: Matrix4.translationValues(_offset, 0, 0),
                child: GestureDetector(
                  behavior: HitTestBehavior.opaque,
                  onTap: open ? () => setState(() => _offset = 0) : null,
                  child: AbsorbPointer(
                    absorbing: open || _busy,
                    child: ColoredBox(
                      color: context.backgroundPrimary,
                      child: widget.child,
                    ),
                  ),
                ),
              ),
              if (_busy)
                Positioned.fill(
                  child: IgnorePointer(
                    child: ColoredBox(
                      color: context.backgroundPrimary.withValues(alpha: 0.55),
                      child: Center(
                        child: SizedBox(
                          width: 22,
                          height: 22,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: context.accentPrimary,
                          ),
                        ),
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

class _SwipeAction extends StatelessWidget {
  const _SwipeAction({
    required this.label,
    required this.icon,
    required this.color,
    required this.enabled,
    required this.onTap,
    this.unavailableReason = '正在处理，请稍后重试。',
  });

  final String label;
  final IconData icon;
  final Color color;
  final bool enabled;
  final VoidCallback onTap;
  final String unavailableReason;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: _ConversationSwipeTileState._actionExtent,
      child: Material(
        color: color.withValues(alpha: enabled ? 0.13 : 0.05),
        child: InkWell(
          onTap: enabled
              ? onTap
              : () => ScaffoldMessenger.of(
                  context,
                ).showSnackBar(SnackBar(content: Text(unavailableReason))),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  icon,
                  size: 20,
                  color: enabled ? color : context.textTertiary,
                ),
                const SizedBox(height: 4),
                Text(
                  label,
                  maxLines: 1,
                  style: AppTypography.label(context).copyWith(
                    color: enabled ? color : context.textTertiary,
                    fontSize: 10,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
