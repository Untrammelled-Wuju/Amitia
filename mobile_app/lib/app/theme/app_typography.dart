import 'package:flutter/material.dart';
import 'app_colors.dart';
import 'design_tokens.dart';

class AppTypography {
  AppTypography._();

  static TextStyle _style(
    BuildContext context, {
    required double size,
    required int weight,
    required Color color,
    required double height,
  }) {
    final tokens = context.uiTypography;
    return TextStyle(
      fontFamily: tokens.fontFamily,
      fontSize: size,
      fontWeight: designFontWeight(weight),
      color: color,
      height: height,
    );
  }

  static TextStyle pageTitle(BuildContext context) {
    final t = context.uiTypography;
    return _style(
      context,
      size: t.pageTitleSize,
      weight: t.pageTitleWeight,
      color: context.textPrimary,
      height: 1.3,
    );
  }

  static TextStyle pageLargeTitle(BuildContext context) {
    final t = context.uiTypography;
    return _style(
      context,
      size: t.pageLargeTitleSize,
      weight: t.pageTitleWeight,
      color: context.textPrimary,
      height: 1.3,
    );
  }

  static TextStyle sectionTitle(BuildContext context) {
    final t = context.uiTypography;
    return _style(
      context,
      size: t.sectionTitleSize,
      weight: t.sectionTitleWeight,
      color: context.textPrimary,
      height: 1.35,
    );
  }

  static TextStyle cardTitle(BuildContext context) {
    final t = context.uiTypography;
    return _style(
      context,
      size: t.cardTitleSize,
      weight: t.cardTitleWeight,
      color: context.textPrimary,
      height: 1.35,
    );
  }

  static TextStyle body(BuildContext context) {
    final t = context.uiTypography;
    return _style(
      context,
      size: t.bodySize,
      weight: t.bodyWeight,
      color: context.textPrimary,
      height: 1.5,
    );
  }

  static TextStyle bodySmall(BuildContext context) {
    final t = context.uiTypography;
    return _style(
      context,
      size: t.bodySmallSize,
      weight: t.bodyWeight,
      color: context.textPrimary,
      height: 1.5,
    );
  }

  static TextStyle caption(BuildContext context) {
    final t = context.uiTypography;
    return _style(
      context,
      size: t.captionSize,
      weight: t.bodyWeight,
      color: context.textSecondary,
      height: 1.4,
    );
  }

  static TextStyle label(BuildContext context) {
    final t = context.uiTypography;
    return _style(
      context,
      size: t.labelSize,
      weight: t.labelWeight,
      color: context.textTertiary,
      height: 1.4,
    );
  }

  static TextStyle statusLabel(BuildContext context) {
    final t = context.uiTypography;
    return TextStyle(
      fontFamily: t.fontFamily,
      fontSize: t.statusLabelSize,
      fontWeight: designFontWeight(t.buttonWeight),
      height: 1.3,
    );
  }

  static TextStyle button(BuildContext context) {
    final t = context.uiTypography;
    return _style(
      context,
      size: t.buttonSize,
      weight: t.buttonWeight,
      color: context.textPrimary,
      height: 1.2,
    );
  }
}
