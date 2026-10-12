import 'package:flutter/material.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

import '../../../../core/ui_runtime/mobile_style_runtime.dart';

class DevicePairScanPage extends StatefulWidget {
  const DevicePairScanPage({super.key});

  @override
  State<DevicePairScanPage> createState() => _DevicePairScanPageState();
}

class _DevicePairScanPageState extends State<DevicePairScanPage> {
  final _controller = MobileScannerController(
    formats: const [BarcodeFormat.qrCode],
  );
  bool _finished = false;

  @override
  void initState() {
    super.initState();
    registerDevicePairScanStyleTargets();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaStyleTarget(
      target: 'device.pair.scan.page',
      child: const SizedBox.shrink(),
      builder: (context, pageStyle) {
        final background =
            pageStyle.color('backgroundColor') ?? pageStyle.color('background');
        final foreground =
            pageStyle.color('foregroundColor') ?? pageStyle.color('textColor');
        final padding = pageStyle.edgeInsets('padding');
        final opacity = pageStyle.number('opacity');
        final page = Scaffold(
          backgroundColor: background,
          appBar: AppBar(
            backgroundColor: background,
            foregroundColor: foreground,
            title: Text(
              '扫描设备配对码',
              style: pageStyle.applyTextStyle(const TextStyle()),
            ),
          ),
          body: Padding(
            padding: padding ?? EdgeInsets.zero,
            child: Stack(
              children: [
                AmitiaStyleTarget(
                  target: 'device.pair.scan.viewfinder',
                  child: const SizedBox.shrink(),
                  builder: (context, style) {
                    final radius = style.borderRadius('borderRadius');
                    final borderSide = style.borderSide();
                    final viewfinderBackground =
                        style.color('backgroundColor') ??
                        style.color('background');
                    final viewfinderPadding = style.edgeInsets('padding');
                    final viewfinderOpacity = style.number('opacity');
                    Widget scanner = MobileScanner(
                      controller: _controller,
                      onDetect: (capture) {
                        if (_finished || !mounted) return;
                        for (final barcode in capture.barcodes) {
                          final raw = barcode.rawValue?.trim() ?? '';
                          final uri = Uri.tryParse(raw);
                          if (uri?.scheme != 'amitia' ||
                              uri?.host != 'pair' ||
                              (uri?.queryParameters['offer'] ?? '').isEmpty) {
                            continue;
                          }
                          _finished = true;
                          _controller.stop();
                          Navigator.of(context).pop(raw);
                          return;
                        }
                      },
                      errorBuilder: (context, error) => Center(
                        child: Padding(
                          padding: const EdgeInsets.all(24),
                          child: Text(
                            '摄像头暂不可用，请检查相机权限，也可以返回后粘贴配对信息。\n${error.errorCode.name}',
                            textAlign: TextAlign.center,
                          ),
                        ),
                      ),
                    );
                    if (viewfinderBackground != null) {
                      scanner = Stack(
                        fit: StackFit.expand,
                        children: [
                          ColoredBox(color: viewfinderBackground),
                          scanner,
                        ],
                      );
                    }
                    if (radius != null) {
                      scanner = ClipRRect(borderRadius: radius, child: scanner);
                    }
                    if (borderSide != null || radius != null) {
                      scanner = DecoratedBox(
                        decoration: BoxDecoration(
                          borderRadius: radius,
                          border: borderSide == null
                              ? null
                              : Border.fromBorderSide(borderSide),
                        ),
                        child: scanner,
                      );
                    }
                    if (viewfinderPadding != null) {
                      scanner = Padding(
                        padding: viewfinderPadding,
                        child: scanner,
                      );
                    }
                    return viewfinderOpacity == null
                        ? scanner
                        : Opacity(
                            opacity: viewfinderOpacity
                                .toDouble()
                                .clamp(0.0, 1.0)
                                .toDouble(),
                            child: scanner,
                          );
                  },
                ),
                Positioned(
                  left: 24,
                  right: 24,
                  bottom: 40,
                  child: AmitiaStyleTarget(
                    target: 'device.pair.scan.hint',
                    child: const SizedBox.shrink(),
                    builder: (context, style) {
                      final radius = style.borderRadius('borderRadius');
                      final borderSide = style.borderSide();
                      final padding =
                          style.edgeInsets('padding') ??
                          const EdgeInsets.all(16);
                      final margin = style.edgeInsets('margin');
                      final opacity = style.number('opacity');
                      final shadow = style.boxShadow();
                      final text = Text(
                        '对准另一台设备的 Amitia 配对二维码。识别后会校验服务地址和设备身份。',
                        textAlign: TextAlign.center,
                        style: style.applyTextStyle(const TextStyle()),
                      );
                      Widget card = Card(
                        color:
                            style.color('backgroundColor') ??
                            style.color('background'),
                        shadowColor: style.color('shadowColor'),
                        elevation: style.number('elevation'),
                        margin: margin,
                        shape: radius == null && borderSide == null
                            ? null
                            : RoundedRectangleBorder(
                                borderRadius:
                                    radius ?? BorderRadius.circular(12),
                                side: borderSide ?? BorderSide.none,
                              ),
                        child: Padding(padding: padding, child: text),
                      );
                      if (shadow != null) {
                        card = DecoratedBox(
                          decoration: BoxDecoration(
                            borderRadius: radius,
                            boxShadow: shadow,
                          ),
                          child: card,
                        );
                      }
                      return opacity == null
                          ? card
                          : Opacity(
                              opacity: opacity
                                  .toDouble()
                                  .clamp(0.0, 1.0)
                                  .toDouble(),
                              child: card,
                            );
                    },
                  ),
                ),
              ],
            ),
          ),
        );
        return opacity == null
            ? page
            : Opacity(
                opacity: opacity.toDouble().clamp(0.0, 1.0).toDouble(),
                child: page,
              );
      },
    );
  }
}

void registerDevicePairScanStyleTargets() {
  AmitiaStyleTargetRegistry.register(
    const AmitiaStyleTargetDescriptor(
      target: 'device.pair.scan.page',
      properties: <String, String>{
        'backgroundColor': 'color',
        'background': 'color',
        'foregroundColor': 'color',
        'textColor': 'color',
        'padding': 'insets',
        'opacity': 'number',
      },
    ),
  );
  AmitiaStyleTargetRegistry.register(
    const AmitiaStyleTargetDescriptor(
      target: 'device.pair.scan.viewfinder',
      properties: <String, String>{
        'backgroundColor': 'color',
        'background': 'color',
        'borderColor': 'color',
        'borderWidth': 'number',
        'borderStyle': 'string',
        'borderRadius': 'length',
        'padding': 'insets',
        'opacity': 'number',
      },
    ),
  );
  AmitiaStyleTargetRegistry.register(
    const AmitiaStyleTargetDescriptor(
      target: 'device.pair.scan.hint',
      properties: <String, String>{
        'backgroundColor': 'color',
        'background': 'color',
        'foregroundColor': 'color',
        'textColor': 'color',
        'borderColor': 'color',
        'borderWidth': 'number',
        'borderStyle': 'string',
        'borderRadius': 'length',
        'shadowColor': 'color',
        'boxShadow': 'shadow',
        'elevation': 'number',
        'padding': 'insets',
        'margin': 'insets',
        'fontSize': 'number',
        'fontWeight': 'number',
        'opacity': 'number',
      },
    ),
  );
}
