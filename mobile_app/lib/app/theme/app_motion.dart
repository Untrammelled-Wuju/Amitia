import 'package:flutter/animation.dart';

class AppMotion {
  AppMotion._();

  static bool _animationsEnabled = true;

  static void setAnimationsEnabled(bool enabled) {
    _animationsEnabled = enabled;
  }

  static Duration _duration(Duration value) =>
      _animationsEnabled ? value : Duration.zero;

  static Duration get quick => _duration(const Duration(milliseconds: 160));
  static Duration get standard => _duration(const Duration(milliseconds: 200));
  static Duration get panel => _duration(const Duration(milliseconds: 280));
  static Duration get pageEnter => _duration(const Duration(milliseconds: 380));
  static Duration get pageExit => _duration(const Duration(milliseconds: 300));
  static Duration get extended => _duration(const Duration(milliseconds: 320));

  static const Curve enterCurve = Curves.easeOutCubic;
  static const Curve exitCurve = Curves.easeInCubic;
  static const Curve standardCurve = Curves.easeOutCubic;
  static const Curve panelCurve = Curves.easeInOutCubicEmphasized;
}
