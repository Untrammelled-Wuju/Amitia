import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'app_routes.dart';

abstract final class CenterNavigation {
  static void openExtensionCenter(BuildContext context) {
    final router = GoRouter.of(context);
    final current = router.routerDelegate.currentConfiguration.fullPath;
    if (current == AppRoutes.extensions) return;
    router.push(AppRoutes.extensions);
  }

  static void openGameCenter(BuildContext context) {
    final router = GoRouter.of(context);
    final current = router.routerDelegate.currentConfiguration.fullPath;
    if (current == AppRoutes.gameCenter) return;
    router.push(AppRoutes.gameCenter);
  }

  static void openDesktopPetCenter(BuildContext context) {
    final router = GoRouter.of(context);
    final current = router.routerDelegate.currentConfiguration.fullPath;
    if (current == AppRoutes.desktopPet) return;
    router.push(AppRoutes.desktopPet);
  }
}
