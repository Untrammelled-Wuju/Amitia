import 'package:flutter/material.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import '../../app/theme/design_tokens.dart';
import '../ui_runtime/mobile_style_runtime.dart';
import 'amitia_button.dart';

double _componentNumber(
  BuildContext context,
  String component,
  String key,
  double fallback,
) {
  final value = context.uiComponentVariant(component)[key];
  return value is num ? value.toDouble() : fallback;
}

Future<bool?> showAmitiaConfirmDialog(
  BuildContext context, {
  required String title,
  required String message,
  String confirmLabel = '确认',
  String cancelLabel = '取消',
  bool isDestructive = false,
}) {
  return showDialog<bool>(
    context: context,
    builder: (ctx) {
      return _AmitiaAlertDialog(
        title: title,
        message: message,
        actionsBuilder: (dialogContext, style) {
          return [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: Text(
                cancelLabel,
                style: style.applyTextStyle(
                  TextStyle(color: dialogContext.textSecondary),
                ),
              ),
            ),
            AmitiaButton(
              label: confirmLabel,
              isDestructive: isDestructive,
              isSecondary: !isDestructive,
              height: 40,
              onPressed: () => Navigator.pop(dialogContext, true),
            ),
          ];
        },
      );
    },
  );
}

Future<void> showAmitiaInfoDialog(
  BuildContext context, {
  required String title,
  required String message,
  String okLabel = '知道了',
}) {
  return showDialog<void>(
    context: context,
    builder: (ctx) {
      return _AmitiaAlertDialog(
        title: title,
        message: message,
        actionsBuilder: (dialogContext, _) {
          return [
            AmitiaButton(
              label: okLabel,
              height: 40,
              onPressed: () => Navigator.pop(dialogContext),
            ),
          ];
        },
      );
    },
  );
}

Future<T?> showAmitiaActionSheet<T>(
  BuildContext context, {
  required String title,
  required List<AmitiaActionSheetItem<T>> actions,
}) {
  final style = context.amitiaStyleFor('component.bottom_sheet');
  final defaultRadius = BorderRadius.vertical(
    top: Radius.circular(
      _componentNumber(context, 'bottomSheet', 'radius', 20),
    ),
  );
  final radius = style.borderRadius('borderRadius') ?? defaultRadius;
  final borderSide = style.borderSide();
  final boxShadow = style.boxShadow();
  return showModalBottomSheet<T>(
    context: context,
    backgroundColor: Colors.transparent,
    elevation: style.number('elevation'),
    shape: RoundedRectangleBorder(
      borderRadius: radius,
      side: borderSide ?? BorderSide.none,
    ),
    builder: (sheetCtx) {
      return DecoratedBox(
        decoration: BoxDecoration(
          color:
              style.color('backgroundColor') ??
              style.color('background') ??
              sheetCtx.surfacePrimary,
          borderRadius: radius,
          border: borderSide == null ? null : Border.fromBorderSide(borderSide),
          boxShadow: boxShadow,
        ),
        child: SafeArea(
          child: Padding(
            padding:
                style.edgeInsets('padding') ??
                const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: style.applyTextStyle(
                    AppTypography.pageTitle(sheetCtx),
                  ),
                ),
                const SizedBox(height: 16),
                ...actions.map((item) {
                  return _ActionSheetTile(
                    icon: item.icon,
                    label: item.label,
                    iconColor: item.isDestructive
                        ? sheetCtx.error
                        : sheetCtx.accentPrimary,
                    onTap: () {
                      Navigator.pop(sheetCtx, item.value);
                    },
                  );
                }),
                const SizedBox(height: 8),
              ],
            ),
          ),
        ),
      );
    },
  );
}

class AmitiaActionSheetItem<T> {
  final IconData icon;
  final String label;
  final T value;
  final bool isDestructive;

  const AmitiaActionSheetItem({
    required this.icon,
    required this.label,
    required this.value,
    this.isDestructive = false,
  });
}

class _ActionSheetTile extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color iconColor;
  final VoidCallback onTap;

  const _ActionSheetTile({
    required this.icon,
    required this.label,
    required this.iconColor,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final sheetStyle = context.amitiaStyleFor('component.bottom_sheet');
    final itemStyle = context.amitiaStyleFor('component.list_item');
    final radius =
        itemStyle.borderRadius('borderRadius') ??
        BorderRadius.circular(AppRadius.small);
    final borderSide = itemStyle.borderSide();
    final boxShadow = itemStyle.boxShadow();
    final textStyle = itemStyle.applyTextStyle(
      sheetStyle.applyTextStyle(AppTypography.body(context)),
    );
    final effectiveIconColor =
        itemStyle.color('iconColor') ??
        itemStyle.color('foregroundColor') ??
        iconColor;
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: DecoratedBox(
        decoration: BoxDecoration(
          color:
              itemStyle.color('backgroundColor') ??
              itemStyle.color('background'),
          borderRadius: radius,
          border: borderSide == null ? null : Border.fromBorderSide(borderSide),
          boxShadow: boxShadow,
        ),
        child: Padding(
          padding:
              itemStyle.edgeInsets('padding') ??
              const EdgeInsets.symmetric(vertical: 12),
          child: Row(
            children: [
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(
                  color: effectiveIconColor.withValues(alpha: 0.12),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(icon, size: 20, color: effectiveIconColor),
              ),
              const SizedBox(width: 12),
              Expanded(child: Text(label, style: textStyle)),
            ],
          ),
        ),
      ),
    );
  }
}

class _AmitiaAlertDialog extends StatelessWidget {
  final String title;
  final String message;
  final List<Widget> Function(BuildContext context, AmitiaResolvedStyle style)
  actionsBuilder;

  const _AmitiaAlertDialog({
    required this.title,
    required this.message,
    required this.actionsBuilder,
  });

  @override
  Widget build(BuildContext context) {
    return AmitiaStyleTarget(
      target: 'component.dialog',
      child: const SizedBox.shrink(),
      builder: (dialogContext, style) {
        final radius =
            style.borderRadius('borderRadius') ??
            BorderRadius.circular(
              _componentNumber(
                dialogContext,
                'dialog',
                'radius',
                AppRadius.medium,
              ),
            );
        final borderSide = style.borderSide();
        final padding = style.edgeInsets('padding');
        final alert = AlertDialog(
          backgroundColor:
              style.color('backgroundColor') ??
              style.color('background') ??
              dialogContext.surfacePrimary,
          shadowColor: style.color('shadowColor'),
          elevation: style.number('elevation'),
          shape: RoundedRectangleBorder(
            borderRadius: radius,
            side: borderSide ?? BorderSide.none,
          ),
          titlePadding: padding,
          contentPadding: padding,
          actionsPadding: padding,
          title: Text(
            title,
            style: style.applyTextStyle(AppTypography.cardTitle(dialogContext)),
          ),
          content: Text(
            message,
            style: style.applyTextStyle(AppTypography.body(dialogContext)),
          ),
          actions: actionsBuilder(dialogContext, style),
        );
        final boxShadow = style.boxShadow();
        if (boxShadow == null) return alert;
        return DecoratedBox(
          decoration: BoxDecoration(borderRadius: radius, boxShadow: boxShadow),
          child: alert,
        );
      },
    );
  }
}

void amitiaSnackBar(BuildContext context, String message) {
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      content: Text(message),
      duration: const Duration(seconds: 2),
      behavior: SnackBarBehavior.floating,
    ),
  );
}
