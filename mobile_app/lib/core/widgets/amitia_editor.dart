import 'package:flutter/material.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_typography.dart';

Future<T?> showAmitiaEditor<T>(
  BuildContext context, {
  required String title,
  required WidgetBuilder builder,
}) {
  return Navigator.of(context).push<T>(
    MaterialPageRoute<T>(
      fullscreenDialog: true,
      builder: (editorContext) => Scaffold(
        resizeToAvoidBottomInset: false,
        appBar: AppBar(
          backgroundColor: editorContext.backgroundPrimary,
          title: Text(title, style: AppTypography.pageTitle(editorContext)),
          leading: IconButton(
            tooltip: '关闭',
            onPressed: () => Navigator.of(editorContext).pop(),
            icon: const Icon(Icons.close),
          ),
        ),
        body: builder(editorContext),
      ),
    ),
  );
}
