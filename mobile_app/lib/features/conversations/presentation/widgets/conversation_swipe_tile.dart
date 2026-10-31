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

  double get _maxOffset => _actionExtent * _actionCount;

  void _handleDragUpdate(DragUpdateDetails details) {
    if (!widget.enabled || _busy) return;
    setState(() {
      _offset = (_offset + details.delta.dx).clamp(-_maxOffset, 0);
    });
  }

  void _handleDragEnd(DragEndDetails details) {
    if (!widget.enabled || _busy) return;
    final velocity = details.primaryVelocity ?? 0;
    final shouldOpen = velocity < -250 || (_offset.abs() > _maxOffset / 3);
    setState(() => _offset = shouldOpen ? -_maxOffset : 0);
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
      child: Stack(
        children: [
          Positioned.fill(
            child: Align(
              alignment: Alignment.centerRight,
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
                    onTap: () => _run(widget.onMarkRead),
                  ),
                ],
              ),
            ),
          ),
          AnimatedContainer(
            duration: _busy ? Duration.zero : const Duration(milliseconds: 170),
            curve: Curves.easeOutCubic,
            transform: Matrix4.translationValues(_offset, 0, 0),
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onHorizontalDragUpdate: _handleDragUpdate,
              onHorizontalDragEnd: _handleDragEnd,
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
  });

  final String label;
  final IconData icon;
  final Color color;
  final bool enabled;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: _ConversationSwipeTileState._actionExtent,
      child: Material(
        color: color.withValues(alpha: enabled ? 0.13 : 0.05),
        child: InkWell(
          onTap: enabled ? onTap : null,
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
