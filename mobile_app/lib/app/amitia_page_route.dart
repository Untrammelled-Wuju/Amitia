import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show PredictiveBackEvent;

const double amitiaSlideRatio = 1;
const Duration _transitionDuration = Duration(milliseconds: 420);
const Duration _reverseDuration = Duration(milliseconds: 380);
const Curve _pageCurve = Curves.easeInOutCubic;
const double _edgeDragWidth = 24;
const double _flingVelocity = 700;

class AmitiaSlidePage<T> extends Page<T> {
  const AmitiaSlidePage({
    required this.child,
    this.rootLevel = false,
    super.key,
  });

  final Widget child;
  final bool rootLevel;

  @override
  Route<T> createRoute(BuildContext context) {
    return AmitiaSlideRoute<T>(
      builder: (_) => child,
      rootLevel: rootLevel,
      settingsOverride: this,
    );
  }
}

AmitiaSlideRoute<T> amitiaSlideRoute<T>({
  required WidgetBuilder builder,
  RouteSettings? settings,
  bool rootLevel = false,
}) {
  return AmitiaSlideRoute<T>(
    builder: builder,
    rootLevel: rootLevel,
    settingsOverride: settings,
  );
}

class AmitiaSlideRoute<T> extends PageRoute<T> {
  AmitiaSlideRoute({
    required WidgetBuilder builder,
    bool rootLevel = false,
    RouteSettings? settingsOverride,
  }) : _builder = builder,
       _defaultRootLevel = rootLevel,
       super(settings: settingsOverride);

  final WidgetBuilder _builder;
  final bool _defaultRootLevel;

  bool get isRootLevel {
    final current = settings;
    return current is AmitiaSlidePage ? current.rootLevel : _defaultRootLevel;
  }

  @override
  bool get opaque => true;

  @override
  Color? get barrierColor => null;

  @override
  String? get barrierLabel => null;

  @override
  bool get maintainState => true;

  @override
  Duration get transitionDuration =>
      isRootLevel ? Duration.zero : _transitionDuration;

  @override
  Duration get reverseTransitionDuration =>
      isRootLevel ? Duration.zero : _reverseDuration;

  @override
  bool get popGestureEnabled {
    if (isFirst) return false;
    if (willHandlePopInternally) return false;
    if (popDisposition == RoutePopDisposition.doNotPop) return false;
    if (fullscreenDialog) return false;
    if (controller?.isAnimating ?? false) return false;
    if (secondaryAnimation?.isDismissed == false) return false;
    return true;
  }

  bool _backGestureActive = false;
  bool get backGestureActive => _backGestureActive;
  double get backGestureProgress => controller?.value ?? 1;

  @override
  void handleStartBackGesture({double progress = 0.0}) {
    final animation = controller;
    final navigatorState = navigator;
    if (animation == null || navigatorState == null) return;
    if (_backGestureActive) {
      animation.value = progress.clamp(0.0, 1.0);
      return;
    }
    if (!popGestureEnabled) return;
    _backGestureActive = true;
    animation.value = progress.clamp(0.0, 1.0);
    navigatorState.didStartUserGesture();
  }

  @override
  void handleUpdateBackGestureProgress({required double progress}) {
    if (!_backGestureActive) return;
    final animation = controller;
    if (animation == null) return;
    animation.value = progress.clamp(0.0, 1.0);
  }

  @override
  void handleCommitBackGesture() {
    if (!_backGestureActive) return;
    _settleBackGesture(pop: true);
  }

  @override
  void handleCancelBackGesture() {
    if (!_backGestureActive) return;
    _settleBackGesture(pop: false);
  }

  void beginBackGesture() {
    if (_backGestureActive || !popGestureEnabled) return;
    final navigatorState = navigator;
    _backGestureActive = true;
    controller?.stop();
    navigatorState?.didStartUserGesture();
  }

  void updateBackGesture(double delta, double width) {
    if (!_backGestureActive || width <= 0) return;
    final animation = controller;
    if (animation == null) return;
    animation.value = (animation.value - delta / width).clamp(0.0, 1.0);
  }

  void endBackGesture({required bool pop}) {
    if (!_backGestureActive) return;
    _settleBackGesture(pop: pop);
  }

  void _settleBackGesture({required bool pop}) {
    final animation = controller;
    final navigatorState = navigator;
    if (animation == null || navigatorState == null) {
      _backGestureActive = false;
      return;
    }
    if (pop) {
      if (!navigatorState.mounted) return;
      navigatorState.pop();
    }
    final distance = pop ? animation.value : 1 - animation.value;
    final milliseconds = (_reverseDuration.inMilliseconds * distance)
        .round()
        .clamp(220, _reverseDuration.inMilliseconds);
    final settling = pop
        ? animation.animateBack(
            0,
            duration: Duration(milliseconds: milliseconds),
            curve: Curves.easeOutCubic,
          )
        : animation.animateTo(
            1,
            duration: Duration(milliseconds: milliseconds),
            curve: Curves.easeOutCubic,
          );
    settling.whenCompleteOrCancel(() {
      if (!_backGestureActive) return;
      _backGestureActive = false;
      navigatorState.didStopUserGesture();
    });
  }

  @override
  void dispose() {
    if (_backGestureActive) {
      _backGestureActive = false;
      navigator?.didStopUserGesture();
    }
    super.dispose();
  }

  @override
  Widget buildPage(
    BuildContext context,
    Animation<double> animation,
    Animation<double> secondaryAnimation,
  ) {
    final current = settings;
    if (current is AmitiaSlidePage) return current.child;
    return _builder(context);
  }

  @override
  Widget buildTransitions(
    BuildContext context,
    Animation<double> animation,
    Animation<double> secondaryAnimation,
    Widget child,
  ) {
    final reduceMotion =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;
    if (reduceMotion) return child;
    final layered = ValueListenableBuilder<bool>(
      valueListenable: navigator!.userGestureInProgressNotifier,
      child: child,
      builder: (context, gestureActive, child) => SlideTransition(
        position: Tween<Offset>(
          begin: Offset.zero,
          end: const Offset(-amitiaSlideRatio, 0),
        ).animate(_transitionProgress(secondaryAnimation, gestureActive)),
        transformHitTests: false,
        child: isRootLevel
            ? child
            : SlideTransition(
                position: Tween<Offset>(
                  begin: const Offset(1, 0),
                  end: Offset.zero,
                ).animate(_transitionProgress(animation, gestureActive)),
                transformHitTests: false,
                child: child,
              ),
      ),
    );
    final forwarder = _AmitiaPredictiveBackForwarder(
      route: this,
      child: layered,
    );
    if (!popGestureEnabled || !_supportsInAppBackGesture(context)) {
      return forwarder;
    }
    return _AmitiaBackGestureDetector(route: this, child: forwarder);
  }

  bool _supportsInAppBackGesture(BuildContext context) {
    final platform = Theme.of(context).platform;
    return platform == TargetPlatform.iOS || platform == TargetPlatform.macOS;
  }

  Animation<double> _transitionProgress(
    Animation<double> parent,
    bool gestureActive,
  ) {
    return gestureActive ? parent : parent.drive(CurveTween(curve: _pageCurve));
  }
}

class _AmitiaPredictiveBackForwarder extends StatefulWidget {
  const _AmitiaPredictiveBackForwarder({
    required this.route,
    required this.child,
  });

  final AmitiaSlideRoute<dynamic> route;
  final Widget child;

  @override
  State<_AmitiaPredictiveBackForwarder> createState() =>
      _AmitiaPredictiveBackForwarderState();
}

class _AmitiaPredictiveBackForwarderState
    extends State<_AmitiaPredictiveBackForwarder>
    with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  bool handleStartBackGesture(PredictiveBackEvent backEvent) {
    final route = widget.route;
    if (!route.isCurrent || !route.popGestureEnabled) return false;
    if (backEvent.isButtonEvent) {
      route.navigator?.maybePop();
      return true;
    }
    route.handleStartBackGesture(progress: 1 - backEvent.progress);
    return true;
  }

  @override
  void handleUpdateBackGestureProgress(PredictiveBackEvent backEvent) {
    widget.route.handleUpdateBackGestureProgress(
      progress: 1 - backEvent.progress,
    );
  }

  @override
  void handleCommitBackGesture() {
    widget.route.handleCommitBackGesture();
  }

  @override
  void handleCancelBackGesture() {
    widget.route.handleCancelBackGesture();
  }

  @override
  Widget build(BuildContext context) => widget.child;
}

class _AmitiaBackGestureDetector extends StatefulWidget {
  const _AmitiaBackGestureDetector({required this.route, required this.child});

  final AmitiaSlideRoute<dynamic> route;
  final Widget child;

  @override
  State<_AmitiaBackGestureDetector> createState() =>
      _AmitiaBackGestureDetectorState();
}

class _AmitiaBackGestureDetectorState
    extends State<_AmitiaBackGestureDetector> {
  double _dragWidth = 1;
  double _edgeWidth = _edgeDragWidth;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final media = MediaQuery.maybeOf(context);
    _dragWidth = media?.size.width ?? 1;
    _edgeWidth = (media?.padding.left ?? 0) + _edgeDragWidth;
  }

  void _handleStart(DragStartDetails details) {
    final route = widget.route;
    if (!route.popGestureEnabled) return;
    route.beginBackGesture();
  }

  void _handleUpdate(DragUpdateDetails details) {
    widget.route.updateBackGesture(details.primaryDelta ?? 0, _dragWidth);
  }

  void _handleEnd(DragEndDetails details) {
    final route = widget.route;
    final velocity = details.velocity.pixelsPerSecond.dx;
    final pop =
        velocity > _flingVelocity ||
        (velocity >= -_flingVelocity && route.backGestureProgress < 0.5);
    route.endBackGesture(pop: pop);
  }

  void _handleCancel() {
    widget.route.endBackGesture(pop: false);
  }

  @override
  Widget build(BuildContext context) {
    return RawGestureDetector(
      behavior: HitTestBehavior.translucent,
      gestures: <Type, GestureRecognizerFactory>{
        _AmitiaBackGestureRecognizer:
            GestureRecognizerFactoryWithHandlers<_AmitiaBackGestureRecognizer>(
              () => _AmitiaBackGestureRecognizer(
                acceptsPointer: (position) => position.dx <= _edgeWidth,
              ),
              (instance) {
                instance
                  ..dragStartBehavior = DragStartBehavior.down
                  ..onStart = _handleStart
                  ..onUpdate = _handleUpdate
                  ..onEnd = _handleEnd
                  ..onCancel = _handleCancel;
              },
            ),
      },
      child: widget.child,
    );
  }
}

class _AmitiaBackGestureRecognizer extends HorizontalDragGestureRecognizer {
  _AmitiaBackGestureRecognizer({required this.acceptsPointer});

  final bool Function(Offset globalPosition) acceptsPointer;

  @override
  void addAllowedPointer(PointerDownEvent event) {
    if (!acceptsPointer(event.position)) return;
    super.addAllowedPointer(event);
  }
}
