import 'package:flutter/material.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import 'amitia_button.dart';

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
      return AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text(title, style: AppTypography.cardTitle(ctx)),
        content: Text(message, style: AppTypography.body(ctx)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: Text(cancelLabel, style: TextStyle(color: context.textSecondary)),
          ),
          AmitiaButton(
            label: confirmLabel,
            isDestructive: isDestructive,
            isSecondary: !isDestructive,
            height: 40,
            onPressed: () => Navigator.pop(ctx, true),
          ),
        ],
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
      return AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text(title, style: AppTypography.cardTitle(ctx)),
        content: Text(message, style: AppTypography.body(ctx)),
        actions: [
          AmitiaButton(
            label: okLabel,
            height: 40,
            onPressed: () => Navigator.pop(ctx),
          ),
        ],
      );
    },
  );
}

Future<T?> showAmitiaActionSheet<T>(
  BuildContext context, {
  required String title,
  required List<AmitiaActionSheetItem<T>> actions,
}) {
  return showModalBottomSheet<T>(
    context: context,
    backgroundColor: context.surfacePrimary,
    shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
    builder: (sheetCtx) {
      return SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const SizedBox(height: 8),
              Center(
                child: Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: context.borderPrimary,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 20),
              Text(title, style: AppTypography.pageTitle(context)),
              const SizedBox(height: 16),
              ...actions.map((item) {
                return _ActionSheetTile(
                  icon: item.icon,
                  label: item.label,
                  iconColor: item.isDestructive ? context.error : context.accentPrimary,
                  onTap: () {
                    Navigator.pop(sheetCtx, item.value);
                  },
                );
              }),
              const SizedBox(height: 8),
            ],
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
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 12),
        child: Row(
          children: [
            Container(
              width: 36,
              height: 36,
              decoration: BoxDecoration(
                color: iconColor.withValues(alpha: 0.12),
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(icon, size: 20, color: iconColor),
            ),
            const SizedBox(width: 12),
            Text(label, style: AppTypography.body(context)),
          ],
        ),
      ),
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
