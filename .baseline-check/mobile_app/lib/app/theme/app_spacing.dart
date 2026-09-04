import 'package:flutter/material.dart';
import 'design_tokens.dart';

/// Backward-compatible layout facade backed by the active ui.tokens provider.
class AppSpacing {
  AppSpacing._();

  static double get xs => DesignTokenRuntime.layout.xs;
  static double get sm => DesignTokenRuntime.layout.sm;
  static double get md => DesignTokenRuntime.layout.md;
  static double get lg => DesignTokenRuntime.layout.lg;
  static double get xl => DesignTokenRuntime.layout.xl;
  static double get xxl => DesignTokenRuntime.layout.xxl;
  static double get xxxl => DesignTokenRuntime.layout.xxxl;

  static double get pagePadding => DesignTokenRuntime.layout.pagePadding;
  static double get cardPadding => DesignTokenRuntime.layout.cardPadding;
  static double get sectionGap => DesignTokenRuntime.layout.sectionGap;
  static double get componentGap => DesignTokenRuntime.layout.componentGap;
  static double get tightGap => DesignTokenRuntime.layout.tightGap;

  static double get listItemMinHeight => DesignTokenRuntime.layout.listItemMinHeight;
  static double get listItemMaxHeight => DesignTokenRuntime.layout.listItemMaxHeight;
  static double get buttonHeight => DesignTokenRuntime.layout.buttonHeight;
  static double get inputMinHeight => DesignTokenRuntime.layout.inputMinHeight;
}
