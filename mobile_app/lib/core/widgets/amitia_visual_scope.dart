import 'package:flutter/material.dart';

class AmitiaVisualScope extends InheritedWidget {
  const AmitiaVisualScope({
    super.key,
    required this.preserveComposer,
    required super.child,
  });

  final bool preserveComposer;

  static bool preservesComposer(BuildContext context) =>
      context
          .dependOnInheritedWidgetOfExactType<AmitiaVisualScope>()
          ?.preserveComposer ??
      false;

  @override
  bool updateShouldNotify(AmitiaVisualScope oldWidget) =>
      preserveComposer != oldWidget.preserveComposer;
}
