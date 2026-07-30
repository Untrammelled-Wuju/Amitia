import 'package:flutter/material.dart';
import 'app_colors.dart';

class AppTypography {
  AppTypography._();

  static TextStyle pageTitle(BuildContext context) {
    return TextStyle(
      fontSize: 20,
      fontWeight: FontWeight.w600,
      color: context.textPrimary,
      height: 1.3,
    );
  }

  static TextStyle pageLargeTitle(BuildContext context) {
    return TextStyle(
      fontSize: 24,
      fontWeight: FontWeight.w600,
      color: context.textPrimary,
      height: 1.3,
    );
  }

  static TextStyle sectionTitle(BuildContext context) {
    return TextStyle(
      fontSize: 17,
      fontWeight: FontWeight.w600,
      color: context.textPrimary,
      height: 1.35,
    );
  }

  static TextStyle cardTitle(BuildContext context) {
    return TextStyle(
      fontSize: 16,
      fontWeight: FontWeight.w500,
      color: context.textPrimary,
      height: 1.35,
    );
  }

  static TextStyle body(BuildContext context) {
    return TextStyle(
      fontSize: 15,
      fontWeight: FontWeight.w400,
      color: context.textPrimary,
      height: 1.5,
    );
  }

  static TextStyle bodySmall(BuildContext context) {
    return TextStyle(
      fontSize: 14,
      fontWeight: FontWeight.w400,
      color: context.textPrimary,
      height: 1.5,
    );
  }

  static TextStyle caption(BuildContext context) {
    return TextStyle(
      fontSize: 13,
      fontWeight: FontWeight.w400,
      color: context.textSecondary,
      height: 1.4,
    );
  }

  static TextStyle label(BuildContext context) {
    return TextStyle(
      fontSize: 12,
      fontWeight: FontWeight.w400,
      color: context.textTertiary,
      height: 1.4,
    );
  }

  static TextStyle statusLabel(BuildContext context) {
    return TextStyle(
      fontSize: 11,
      fontWeight: FontWeight.w500,
      height: 1.3,
    );
  }

  static TextStyle button(BuildContext context) {
    return TextStyle(
      fontSize: 15,
      fontWeight: FontWeight.w500,
      color: context.textPrimary,
      height: 1.2,
    );
  }
}
