import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';

const double tabletShortestSide = 600;

bool isTabletSize(Size size) => size.shortestSide >= tabletShortestSide;

List<DeviceOrientation> preferredOrientationsForSize(Size size) {
  return isTabletSize(size)
      ? DeviceOrientation.values
      : const <DeviceOrientation>[DeviceOrientation.portraitUp];
}

bool? _lastTabletState;

Future<void> applyDeviceOrientationPolicy() async {
  if (kIsWeb ||
      (defaultTargetPlatform != TargetPlatform.android &&
          defaultTargetPlatform != TargetPlatform.iOS)) {
    return;
  }
  final views = WidgetsBinding.instance.platformDispatcher.views;
  if (views.isEmpty) return;
  final view = views.first;
  final scale = view.devicePixelRatio;
  if (scale <= 0) return;
  final size = view.physicalSize / scale;
  final tablet = isTabletSize(size);
  if (_lastTabletState == tablet) return;
  _lastTabletState = tablet;
  await SystemChrome.setPreferredOrientations(
    preferredOrientationsForSize(size),
  );
}
