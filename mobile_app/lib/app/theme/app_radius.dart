import 'package:flutter/material.dart';
import 'design_tokens.dart';

class AppRadius {
  AppRadius._();

  static double get small => DesignTokenRuntime.layout.radiusSmall;
  static double get medium => DesignTokenRuntime.layout.radiusMedium;
  static double get large => DesignTokenRuntime.layout.radiusLarge;
  static double get tag => DesignTokenRuntime.layout.radiusTag;
  static double get extraSmall => DesignTokenRuntime.layout.radiusExtraSmall;

  static BorderRadius get brSmall => BorderRadius.all(Radius.circular(small));
  static BorderRadius get brMedium => BorderRadius.all(Radius.circular(medium));
  static BorderRadius get brLarge => BorderRadius.all(Radius.circular(large));
  static BorderRadius get brTag => BorderRadius.all(Radius.circular(tag));
  static BorderRadius get brExtraSmall => BorderRadius.all(Radius.circular(extraSmall));
}
