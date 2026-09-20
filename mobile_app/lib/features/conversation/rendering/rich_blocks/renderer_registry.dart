import 'package:flutter/widgets.dart';

import '../amrp.dart';

typedef AmrpExtensionRenderer =
    Widget Function(BuildContext context, AmrpExtensionBlock block);

class AmrpRendererRegistry {
  AmrpRendererRegistry._();

  static final Map<String, AmrpExtensionRenderer> _extensions =
      <String, AmrpExtensionRenderer>{};

  static void registerExtension(
    String rendererId,
    AmrpExtensionRenderer renderer,
  ) {
    final id = rendererId.trim();
    if (id.isEmpty) return;
    _extensions[id] = renderer;
  }

  static AmrpExtensionRenderer? resolveExtension(String rendererId) {
    return _extensions[rendererId.trim()];
  }

  static List<String> get registeredExtensions =>
      _extensions.keys.toList(growable: false)..sort();
}
