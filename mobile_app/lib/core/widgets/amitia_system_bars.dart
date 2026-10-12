import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../../app/theme/app_colors.dart';

class AmitiaSystemBars extends StatelessWidget {
  const AmitiaSystemBars({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final background = context.backgroundPrimary;
    final backgroundBrightness = ThemeData.estimateBrightnessForColor(
      background,
    );
    final iconBrightness = backgroundBrightness == Brightness.dark
        ? Brightness.light
        : Brightness.dark;

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: SystemUiOverlayStyle(
        statusBarColor: Colors.transparent,
        statusBarBrightness: backgroundBrightness,
        statusBarIconBrightness: iconBrightness,
        systemStatusBarContrastEnforced: false,
        systemNavigationBarColor: Colors.transparent,
        systemNavigationBarDividerColor: Colors.transparent,
        systemNavigationBarIconBrightness: iconBrightness,
        systemNavigationBarContrastEnforced: false,
      ),
      child: ColoredBox(color: background, child: child),
    );
  }
}
