import 'package:flutter/material.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

class DevicePairScanPage extends StatefulWidget {
  const DevicePairScanPage({super.key});

  @override
  State<DevicePairScanPage> createState() => _DevicePairScanPageState();
}

class _DevicePairScanPageState extends State<DevicePairScanPage> {
  final _controller = MobileScannerController(formats: const [BarcodeFormat.qrCode]);
  bool _finished = false;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('扫描设备配对码')),
      body: Stack(
        children: [
          MobileScanner(
            controller: _controller,
            onDetect: (capture) {
              if (_finished || !mounted) return;
              for (final barcode in capture.barcodes) {
                final raw = barcode.rawValue?.trim() ?? '';
                final uri = Uri.tryParse(raw);
                if (uri?.scheme != 'amitia' || uri?.host != 'pair' || (uri?.queryParameters['offer'] ?? '').isEmpty) continue;
                _finished = true;
                _controller.stop();
                Navigator.of(context).pop(raw);
                return;
              }
            },
            errorBuilder: (context, error) => Center(
              child: Padding(padding: const EdgeInsets.all(24), child: Text('摄像头暂不可用，请检查相机权限，也可以返回后粘贴配对信息。\n${error.errorCode.name}', textAlign: TextAlign.center)),
            ),
          ),
          const Positioned(left: 24, right: 24, bottom: 40, child: Card(child: Padding(padding: EdgeInsets.all(16), child: Text('对准另一台设备的 Amitia 配对二维码。识别后会校验服务地址和设备身份。', textAlign: TextAlign.center)))),
        ],
      ),
    );
  }
}
