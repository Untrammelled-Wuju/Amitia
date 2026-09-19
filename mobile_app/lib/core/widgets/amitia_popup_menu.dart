import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_motion.dart';

enum AmitiaPopupMenuHorizontalAnchor { start, center, end }

class AmitiaPopupMenuPlacement {
  const AmitiaPopupMenuPlacement({
    required this.horizontalAnchor,
    required this.opensAbove,
    required this.scaleAlignment,
  });

  final AmitiaPopupMenuHorizontalAnchor horizontalAnchor;
  final bool opensAbove;
  final Alignment scaleAlignment;

  static AmitiaPopupMenuPlacement resolve({
    required Rect anchorRect,
    required Rect overlayRect,
    required double estimatedMenuHeight,
    double gap = 6,
    double margin = 8,
  }) {
    final relativeX = overlayRect.width <= 0
        ? 0.5
        : ((anchorRect.center.dx - overlayRect.left) / overlayRect.width).clamp(
            0.0,
            1.0,
          );
    final horizontalAnchor = relativeX < 0.34
        ? AmitiaPopupMenuHorizontalAnchor.start
        : relativeX > 0.66
        ? AmitiaPopupMenuHorizontalAnchor.end
        : AmitiaPopupMenuHorizontalAnchor.center;
    final spaceBelow = overlayRect.bottom - margin - anchorRect.bottom - gap;
    final spaceAbove = anchorRect.top - gap - overlayRect.top - margin;
    final opensAbove =
        spaceBelow < estimatedMenuHeight && spaceAbove > spaceBelow;
    return AmitiaPopupMenuPlacement(
      horizontalAnchor: horizontalAnchor,
      opensAbove: opensAbove,
      scaleAlignment: Alignment(switch (horizontalAnchor) {
        AmitiaPopupMenuHorizontalAnchor.start => -1,
        AmitiaPopupMenuHorizontalAnchor.center => 0,
        AmitiaPopupMenuHorizontalAnchor.end => 1,
      }, opensAbove ? 1 : -1),
    );
  }
}

class AmitiaPopupMenuButton<T> extends StatefulWidget {
  const AmitiaPopupMenuButton({
    super.key,
    required this.itemBuilder,
    this.onSelected,
    this.initialValue,
    this.enabled = true,
    this.tooltip,
    this.icon,
    this.child,
    this.padding = const EdgeInsets.all(8),
    this.menuWidth = 200,
    this.useRootNavigator = true,
  });

  final List<PopupMenuEntry<T>> Function(BuildContext context) itemBuilder;
  final ValueChanged<T>? onSelected;
  final T? initialValue;
  final bool enabled;
  final String? tooltip;
  final Widget? icon;
  final Widget? child;
  final EdgeInsetsGeometry padding;
  final double menuWidth;
  final bool useRootNavigator;

  @override
  State<AmitiaPopupMenuButton<T>> createState() =>
      _AmitiaPopupMenuButtonState<T>();
}

class _AmitiaPopupMenuButtonState<T> extends State<AmitiaPopupMenuButton<T>> {
  final GlobalKey _anchorKey = GlobalKey();

  Future<void> _showMenu() async {
    final anchorContext = _anchorKey.currentContext;
    if (anchorContext == null) return;
    final anchorBox = anchorContext.findRenderObject();
    if (anchorBox is! RenderBox || !anchorBox.hasSize) return;
    final topLeft = anchorBox.localToGlobal(Offset.zero);
    final bottomRight = anchorBox.localToGlobal(
      anchorBox.size.bottomRight(Offset.zero),
    );
    final value = await showAmitiaPopupMenu<T>(
      context: context,
      anchorRect: Rect.fromPoints(topLeft, bottomRight),
      items: widget.itemBuilder(context),
      initialValue: widget.initialValue,
      menuWidth: widget.menuWidth,
      useRootNavigator: widget.useRootNavigator,
    );
    if (value != null && mounted) widget.onSelected?.call(value);
  }

  @override
  Widget build(BuildContext context) {
    final child = widget.child;
    final trigger = child != null
        ? InkWell(
            onTap: widget.enabled ? _showMenu : null,
            borderRadius: BorderRadius.circular(8),
            child: child,
          )
        : IconButton(
            tooltip: widget.tooltip,
            onPressed: widget.enabled ? _showMenu : null,
            padding: widget.padding,
            icon: widget.icon ?? const Icon(Icons.more_vert),
          );
    return KeyedSubtree(
      key: _anchorKey,
      child: widget.tooltip == null || child == null
          ? trigger
          : Tooltip(message: widget.tooltip!, child: trigger),
    );
  }
}

Future<T?> showAmitiaPopupMenu<T>({
  required BuildContext context,
  required Rect anchorRect,
  required List<PopupMenuEntry<T>> items,
  T? initialValue,
  double menuWidth = 200,
  double gap = 6,
  double margin = 8,
  bool useRootNavigator = true,
}) {
  final overlayState = Overlay.of(context, rootOverlay: useRootNavigator);
  final overlayBox = overlayState.context.findRenderObject();
  final overlaySize = overlayBox is RenderBox
      ? overlayBox.size
      : MediaQuery.sizeOf(context);
  final overlayOrigin = overlayBox is RenderBox
      ? overlayBox.localToGlobal(Offset.zero)
      : Offset.zero;
  final localAnchorRect = anchorRect.shift(-overlayOrigin);
  final overlayRect = Offset.zero & overlaySize;
  final estimatedMenuHeight = _estimateMenuHeight(items);
  final placement = AmitiaPopupMenuPlacement.resolve(
    anchorRect: localAnchorRect,
    overlayRect: overlayRect,
    estimatedMenuHeight: estimatedMenuHeight,
    gap: gap,
    margin: margin,
  );
  final resolvedWidth = math.min(
    menuWidth,
    math.max(0, overlaySize.width - margin * 2),
  );
  return showGeneralDialog<T>(
    context: context,
    useRootNavigator: useRootNavigator,
    barrierDismissible: true,
    barrierLabel: MaterialLocalizations.of(context).modalBarrierDismissLabel,
    barrierColor: Colors.transparent,
    transitionDuration: AppMotion.quick,
    pageBuilder: (_, _, _) => _AmitiaPopupMenuLayout(
      anchorRect: localAnchorRect,
      overlayRect: overlayRect,
      horizontalAnchor: placement.horizontalAnchor,
      opensAbove: placement.opensAbove,
      gap: gap,
      margin: margin,
      menuWidth: resolvedWidth.toDouble(),
      child: _AmitiaPopupMenuPanel<T>(items: items, initialValue: initialValue),
    ),
    transitionBuilder: (context, animation, _, child) {
      final curved = CurvedAnimation(
        parent: animation,
        curve: AppMotion.enterCurve,
        reverseCurve: AppMotion.exitCurve,
      );
      return FadeTransition(
        opacity: curved,
        child: ScaleTransition(
          alignment: placement.scaleAlignment,
          scale: Tween<double>(begin: 0.9, end: 1).animate(curved),
          child: child,
        ),
      );
    },
  );
}

double _estimateMenuHeight<T>(List<PopupMenuEntry<T>> items) {
  var height = 12.0;
  for (final entry in items) {
    height += entry.height;
  }
  return height;
}

class _AmitiaPopupMenuLayout extends StatelessWidget {
  const _AmitiaPopupMenuLayout({
    required this.anchorRect,
    required this.overlayRect,
    required this.horizontalAnchor,
    required this.opensAbove,
    required this.gap,
    required this.margin,
    required this.menuWidth,
    required this.child,
  });

  final Rect anchorRect;
  final Rect overlayRect;
  final AmitiaPopupMenuHorizontalAnchor horizontalAnchor;
  final bool opensAbove;
  final double gap;
  final double margin;
  final double menuWidth;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return CustomSingleChildLayout(
      delegate: _AmitiaPopupMenuLayoutDelegate(
        anchorRect: anchorRect,
        overlayRect: overlayRect,
        horizontalAnchor: horizontalAnchor,
        opensAbove: opensAbove,
        gap: gap,
        margin: margin,
        menuWidth: menuWidth,
      ),
      child: child,
    );
  }
}

class _AmitiaPopupMenuLayoutDelegate extends SingleChildLayoutDelegate {
  const _AmitiaPopupMenuLayoutDelegate({
    required this.anchorRect,
    required this.overlayRect,
    required this.horizontalAnchor,
    required this.opensAbove,
    required this.gap,
    required this.margin,
    required this.menuWidth,
  });

  final Rect anchorRect;
  final Rect overlayRect;
  final AmitiaPopupMenuHorizontalAnchor horizontalAnchor;
  final bool opensAbove;
  final double gap;
  final double margin;
  final double menuWidth;

  @override
  BoxConstraints getConstraintsForChild(BoxConstraints constraints) {
    return BoxConstraints(
      minWidth: menuWidth,
      maxWidth: menuWidth,
      maxHeight: math.max(0, overlayRect.height - margin * 2),
    );
  }

  @override
  Offset getPositionForChild(Size size, Size childSize) {
    final top = opensAbove
        ? anchorRect.top - gap - childSize.height
        : anchorRect.bottom + gap;
    final left = switch (horizontalAnchor) {
      AmitiaPopupMenuHorizontalAnchor.start => anchorRect.left,
      AmitiaPopupMenuHorizontalAnchor.center =>
        anchorRect.center.dx - childSize.width / 2,
      AmitiaPopupMenuHorizontalAnchor.end => anchorRect.right - childSize.width,
    };
    final maxTop = overlayRect.bottom - margin - childSize.height;
    final maxLeft = overlayRect.right - margin - childSize.width;
    return Offset(
      left.clamp(
        overlayRect.left + margin,
        math.max(overlayRect.left + margin, maxLeft),
      ),
      top.clamp(
        overlayRect.top + margin,
        math.max(overlayRect.top + margin, maxTop),
      ),
    );
  }

  @override
  bool shouldRelayout(covariant _AmitiaPopupMenuLayoutDelegate oldDelegate) {
    return anchorRect != oldDelegate.anchorRect ||
        overlayRect != oldDelegate.overlayRect ||
        horizontalAnchor != oldDelegate.horizontalAnchor ||
        opensAbove != oldDelegate.opensAbove ||
        gap != oldDelegate.gap ||
        margin != oldDelegate.margin ||
        menuWidth != oldDelegate.menuWidth;
  }
}

class _AmitiaPopupMenuPanel<T> extends StatelessWidget {
  const _AmitiaPopupMenuPanel({
    required this.items,
    required this.initialValue,
  });

  final List<PopupMenuEntry<T>> items;
  final T? initialValue;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(14),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.12),
            blurRadius: 14,
            spreadRadius: 0,
            offset: Offset.zero,
          ),
        ],
      ),
      child: Material(
        color: context.surfacePrimary,
        borderRadius: BorderRadius.circular(14),
        clipBehavior: Clip.antiAlias,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxHeight: double.infinity),
          child: SingleChildScrollView(
            padding: const EdgeInsets.symmetric(vertical: 6),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                for (var index = 0; index < items.length; index++)
                  _buildEntry(context, items[index], index),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildEntry(BuildContext context, PopupMenuEntry<T> entry, int index) {
    if (entry is PopupMenuDivider) {
      return Divider(
        key: ValueKey<int>(index),
        height: entry.height,
        thickness: entry.thickness,
        indent: entry.indent,
        endIndent: entry.endIndent,
        radius: entry.radius,
        color: entry.color ?? context.borderSecondary,
      );
    }
    if (entry is CheckedPopupMenuItem<T>) {
      return _menuItem(
        context,
        entry: entry,
        selected:
            entry.checked ||
            (initialValue != null && entry.represents(initialValue)),
        child: Row(
          children: [
            Expanded(
              child: _entryChild(
                context,
                entry,
                child: entry.child ?? const SizedBox.shrink(),
              ),
            ),
            const SizedBox(width: 10),
            AnimatedOpacity(
              opacity: entry.checked ? 1 : 0,
              duration: AppMotion.quick,
              child: Icon(Icons.check, size: 18, color: context.accentPrimary),
            ),
          ],
        ),
      );
    }
    if (entry is PopupMenuItem<T>) {
      return _menuItem(
        context,
        entry: entry,
        selected: initialValue != null && entry.represents(initialValue),
        child: _entryChild(
          context,
          entry,
          child: entry.child ?? const SizedBox.shrink(),
        ),
      );
    }
    return const SizedBox.shrink();
  }

  Widget _menuItem(
    BuildContext context, {
    required PopupMenuItem<T> entry,
    required bool selected,
    required Widget child,
  }) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 6),
      child: Material(
        color: selected ? context.accentSoft : Colors.transparent,
        borderRadius: BorderRadius.circular(9),
        child: InkWell(
          onTap: entry.enabled
              ? () {
                  Navigator.of(context).pop(entry.value);
                  entry.onTap?.call();
                }
              : null,
          borderRadius: BorderRadius.circular(9),
          child: ConstrainedBox(
            constraints: BoxConstraints(minHeight: entry.height),
            child: Padding(
              padding:
                  entry.padding ??
                  const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              child: Align(alignment: Alignment.centerLeft, child: child),
            ),
          ),
        ),
      ),
    );
  }

  Widget _entryChild(
    BuildContext context,
    PopupMenuItem<T> entry, {
    required Widget child,
  }) {
    final style =
        entry.textStyle ??
        TextStyle(
          color: entry.enabled ? context.textPrimary : context.textDisabled,
          fontSize: 14.5,
        );
    return DefaultTextStyle.merge(style: style, child: child);
  }
}
