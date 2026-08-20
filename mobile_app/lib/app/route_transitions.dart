import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'theme/app_motion.dart';

Widget backTargetTransition({
  required Animation<double> secondaryAnimation,
  required Widget child,
}) {
  final transition = CurvedAnimation(
    parent: ReverseAnimation(secondaryAnimation),
    curve: AppMotion.enterCurve,
    reverseCurve: AppMotion.exitCurve,
  );
  final slide = Tween<Offset>(
    begin: const Offset(-0.08, 0),
    end: Offset.zero,
  ).animate(transition);
  return FadeTransition(
    opacity: transition,
    child: SlideTransition(position: slide, child: child),
  );
}

CustomTransitionPage<T> slideFadePage<T>({
  required BuildContext context,
  required GoRouterState state,
  required Widget child,
}) {
  return CustomTransitionPage<T>(
    key: state.pageKey,
    child: child,
    transitionDuration: AppMotion.pageEnter,
    reverseTransitionDuration: AppMotion.pageExit,
    transitionsBuilder: (context, animation, secondaryAnimation, child) {
      final incomingAnimation = CurvedAnimation(
        parent: animation,
        curve: AppMotion.enterCurve,
        reverseCurve: AppMotion.exitCurve,
      );
      final slide = Tween<Offset>(
        begin: const Offset(1.0, 0.0),
        end: Offset.zero,
      ).animate(incomingAnimation);

      final page = FadeTransition(
        opacity: incomingAnimation,
        child: SlideTransition(position: slide, child: child),
      );

      return backTargetTransition(
        secondaryAnimation: secondaryAnimation,
        child: page,
      );
    },
  );
}

CustomTransitionPage<T> drawerSlideFadePage<T>({
  required GoRouterState state,
  required Widget child,
}) {
  return CustomTransitionPage<T>(
    key: state.pageKey,
    child: child,
    transitionDuration: AppMotion.pageEnter,
    reverseTransitionDuration: AppMotion.pageExit,
    transitionsBuilder: (context, animation, secondaryAnimation, child) {
      final transition = CurvedAnimation(
        parent: animation,
        curve: AppMotion.enterCurve,
        reverseCurve: AppMotion.exitCurve,
      );
      final slide = Tween<Offset>(
        begin: const Offset(0.16, 0),
        end: Offset.zero,
      ).animate(transition);
      final page = FadeTransition(
        opacity: transition,
        child: SlideTransition(position: slide, child: child),
      );
      return backTargetTransition(
        secondaryAnimation: secondaryAnimation,
        child: page,
      );
    },
  );
}

CustomTransitionPage<T> chatRootPage<T>({
  required GoRouterState state,
  required Widget child,
}) {
  return CustomTransitionPage<T>(
    key: state.pageKey,
    child: child,
    transitionDuration: Duration.zero,
    reverseTransitionDuration: Duration.zero,
    transitionsBuilder: (context, animation, secondaryAnimation, child) {
      return backTargetTransition(
        secondaryAnimation: secondaryAnimation,
        child: child,
      );
    },
  );
}
