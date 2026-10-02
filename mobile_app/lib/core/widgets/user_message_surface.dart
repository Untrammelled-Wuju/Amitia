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
    return Container(
      margin: margin,
      constraints: constraints,
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
              color: decoration.color?.withValues(alpha: water ? 0.42 : 0.65),
              gradient: water
                  ? LinearGradient(
                      begin: Alignment.topLeft,
                      end: Alignment.bottomRight,
                      colors: [
                        Colors.white.withValues(alpha: 0.24),
                        Colors.white.withValues(alpha: 0),
                        Colors.white.withValues(alpha: 0.08),
                      ],
                      stops: const [0, 0.45, 1],
                    )
                  : decoration.gradient,
              border: Border.all(
                color: water
                    ? Colors.white.withValues(alpha: 0.5)
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
