import 'package:flutter/material.dart';

class AppRadius {
  AppRadius._();

  static const double small = 12;
  static const double medium = 16;
  static const double large = 22;
  static const double tag = 10;
  static const double extraSmall = 8;

  static const BorderRadius brSmall = BorderRadius.all(Radius.circular(small));
  static const BorderRadius brMedium = BorderRadius.all(Radius.circular(medium));
  static const BorderRadius brLarge = BorderRadius.all(Radius.circular(large));
  static const BorderRadius brTag = BorderRadius.all(Radius.circular(tag));
  static const BorderRadius brExtraSmall = BorderRadius.all(Radius.circular(extraSmall));
}
