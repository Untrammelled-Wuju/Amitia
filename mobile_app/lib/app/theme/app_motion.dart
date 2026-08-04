import 'package:flutter/animation.dart';

class AppMotion {
  AppMotion._();

  static const Duration quick = Duration(milliseconds: 160);
  static const Duration standard = Duration(milliseconds: 200);
  static const Duration panel = Duration(milliseconds: 280);
  static const Duration pageEnter = Duration(milliseconds: 380);
  static const Duration pageExit = Duration(milliseconds: 300);
  static const Duration extended = Duration(milliseconds: 320);

  static const Curve enterCurve = Curves.easeOutCubic;
  static const Curve exitCurve = Curves.easeInCubic;
  static const Curve standardCurve = Curves.easeOutCubic;
  static const Curve panelCurve = Curves.easeInOutCubicEmphasized;
}
