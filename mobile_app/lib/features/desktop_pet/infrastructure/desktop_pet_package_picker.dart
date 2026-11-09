import 'package:file_picker/file_picker.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

typedef DesktopPetPackagePicker = Future<String?> Function();

final desktopPetPackagePickerProvider = Provider<DesktopPetPackagePicker>(
  (_) => pickDesktopPetPackagePath,
);

Future<String?> pickDesktopPetPackagePath() async {
  final result = await FilePicker.platform.pickFiles(
    type: FileType.any,
    allowMultiple: false,
    withData: false,
    withReadStream: false,
  );
  final path = result?.files.singleOrNull?.path?.trim() ?? '';
  if (path.isEmpty) return null;
  if (!path.toLowerCase().endsWith('.petx')) {
    throw StateError('请选择 .petx 桌宠插件包: $path');
  }
  return path;
}
