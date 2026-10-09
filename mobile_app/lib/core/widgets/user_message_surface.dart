import 'dart:ui';
import 'package:flutter/material.dart';
import '../settings/chat_appearance_preferences.dart';

class UserMessageSurface extends StatelessWidget {
  final UserMessageMaterial material;
  final BoxDecoration decoration;
  final Widget child;
  final EdgeInsetsGeometry? padding;
  final EdgeInsetsGeometry? margin;
  final BoxConstraints? constraints;
  final Clip clipBehavior;

  const UserMessageSurface({
    super.key,
    this.material = UserMessageMaterial.solid,
    required this.decoration,
    required this.child,
    this.padding,
    this.margin,
    this.constraints,
    this.clipBehavior = Clip.none,
  });

  @override
  Widget build(BuildContext context) {
    if (material == UserMessageMaterial.solid) {
      return Container(
        decoration: decoration,
        padding: padding,
        margin: margin,
        constraints: constraints,
        clipBehavior: clipBehavior,
        child: child,
      );
    }
    final water = material == UserMessageMaterial.water;
    final scheme = Theme.of(context).colorScheme;
    final dark = Theme.of(context).brightness == Brightness.dark;
    final waterColor = Color.lerp(scheme.surface, scheme.primary, 0.14)!;
    return Container(
      margin: margin,
      constraints: constraints,
      decoration: water
          ? BoxDecoration(
              borderRadius: decoration.borderRadius,
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: dark ? 0.20 : 0.09),
                  blurRadius: 12,
                  offset: const Offset(0, 3),
                ),
              ],
            )
          : null,
      child: ClipRRect(
        borderRadius: decoration.borderRadius ?? BorderRadius.zero,
        child: BackdropFilter(
          filter: ImageFilter.blur(
            sigmaX: water ? 4 : 12,
            sigmaY: water ? 4 : 12,
          ),
          child: Container(
            padding: padding,
            decoration: decoration.copyWith(
              color: water
                  ? waterColor.withValues(alpha: 0.60)
                  : decoration.color?.withValues(alpha: 0.65),
              gradient: water
                  ? LinearGradient(
                      begin: Alignment.topLeft,
                      end: Alignment.bottomRight,
                      colors: [
                        Colors.white.withValues(alpha: dark ? 0.16 : 0.36),
                        Colors.white.withValues(alpha: 0),
                        scheme.primary.withValues(alpha: dark ? 0.16 : 0.12),
                      ],
                      stops: const [0, 0.45, 1],
                    )
                  : decoration.gradient,
              border: Border.all(
                color: water
                    ? (dark ? Colors.white : scheme.primary).withValues(
                        alpha: 0.16,
                      )
                    : Theme.of(
                        context,
                      ).colorScheme.onSurface.withValues(alpha: 0.10),
                width: 0.5,
              ),
            ),
            child: child,
          ),
        ),
      ),
    );
  }
}
