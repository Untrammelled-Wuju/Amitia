import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../app/theme/app_colors.dart';
import '../settings/appearance_preferences.dart';
import '../settings/background_preferences.dart';
import 'background_media.dart';

class BackgroundSurfaceScope extends InheritedWidget {
  final bool active;
  const BackgroundSurfaceScope({
    super.key,
    required this.active,
    required super.child,
  });
  static bool isActive(BuildContext context) =>
      context
          .dependOnInheritedWidgetOfExactType<BackgroundSurfaceScope>()
          ?.active ??
      false;
  @override
  bool updateShouldNotify(BackgroundSurfaceScope oldWidget) =>
      active != oldWidget.active;
}

class ApplicationBackground extends ConsumerWidget {
  final Widget child;
  const ApplicationBackground({super.key, required this.child});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final preferences = ref.watch(backgroundPreferencesProvider);
    final appearance = ref.watch(appearancePreferencesProvider);
    final theme = Theme.of(context);
    final active = preferences.active;
    return BackgroundSurfaceScope(
      active: active,
      child: Stack(
        fit: StackFit.expand,
        children: [
          if (active)
            Positioned.fill(
              child: IgnorePointer(
                child: ExcludeSemantics(
                  child: BackgroundMedia(
                    preferences: preferences,
                    baseColor: context.backgroundPrimary,
                    animate:
                        appearance.dynamicEffect && !appearance.reduceAnimation,
                  ),
                ),
              ),
            ),
          Theme(
            data: active
                ? theme.copyWith(
                    scaffoldBackgroundColor: Colors.transparent,
                    appBarTheme: theme.appBarTheme.copyWith(
                      backgroundColor: context.backgroundPrimary.withValues(
                        alpha: 0.88,
                      ),
                    ),
                  )
                : theme,
            child: child,
          ),
        ],
      ),
    );
  }
}
