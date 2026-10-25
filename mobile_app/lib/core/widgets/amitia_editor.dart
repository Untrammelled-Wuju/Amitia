import 'package:flutter/material.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_typography.dart';
import '../ui_runtime/mobile_style_runtime.dart';
import 'amitia_icons.dart';

Future<T?> showAmitiaEditor<T>(
  BuildContext context, {
  required String title,
  required WidgetBuilder builder,
}) {
  return Navigator.of(context).push<T>(
    MaterialPageRoute<T>(
      fullscreenDialog: true,
      builder: (editorContext) => AmitiaStyleTarget(
        target: 'editor',
        builder: (context, style) {
          final background =
              style.color('backgroundColor') ??
              style.color('background') ??
              context.backgroundPrimary;
          final foreground =
              style.color('foregroundColor') ?? style.color('textColor');
          final padding = style.edgeInsets('padding');
          final body = builder(context);
          final scaffold = Scaffold(
            resizeToAvoidBottomInset: false,
            backgroundColor: background,
            appBar: AppBar(
              backgroundColor: background,
              foregroundColor: foreground,
              title: Text(
                title,
                style: style.applyTextStyle(AppTypography.pageTitle(context)),
              ),
              leading: IconButton(
                tooltip: '关闭',
                onPressed: () => Navigator.of(context).pop(),
                icon: const AmitiaIcon(AmitiaIconGlyph.close),
              ),
            ),
            body: padding == null
                ? body
                : Padding(padding: padding, child: body),
          );
          final opacity = style.number('opacity');
          return opacity == null
              ? scaffold
              : Opacity(
                  opacity: opacity.toDouble().clamp(0.0, 1.0).toDouble(),
                  child: scaffold,
                );
        },
        child: const SizedBox.shrink(),
      ),
    ),
  );
}
