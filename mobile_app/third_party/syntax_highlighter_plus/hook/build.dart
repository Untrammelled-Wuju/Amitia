import 'dart:io';

import 'package:flutter_rust_bridge_hooks/flutter_rust_bridge_hooks.dart';

void main(List<String> args) async {
  await build(args, (input, output) async {
    if (Platform.isWindows) return;
    await const FlutterRustBridgeNativeAssetsBuilder(
      cratePath: 'rust',
    ).run(input: input, output: output);
  });
}
