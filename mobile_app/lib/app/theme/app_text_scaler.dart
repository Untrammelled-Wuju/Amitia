import 'package:flutter/painting.dart';

class AppTextScaler extends TextScaler {
  const AppTextScaler(this.system, this.preference);

  final TextScaler system;
  final double preference;

  TextScaler get composer =>
      TextScaler.linear((system.scale(1) * preference).clamp(0.8, 2.0));

  @override
  double scale(double fontSize) => system.scale(fontSize) * preference;

  @override
  double get textScaleFactor => scale(14) / 14;

  @override
  bool operator ==(Object other) =>
      other is AppTextScaler &&
      other.system == system &&
      other.preference == preference;

  @override
  int get hashCode => Object.hash(system, preference);
}
