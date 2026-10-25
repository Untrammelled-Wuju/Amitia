import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'amitia_page_route.dart';

Page<T> amitiaSlidePage<T>({
  BuildContext? context,
  required GoRouterState state,
  required Widget child,
  bool rootLevel = false,
}) {
  return AmitiaSlidePage<T>(
    key: state.pageKey,
    child: child,
    rootLevel: rootLevel,
  );
}
