import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:path_provider/path_provider.dart';

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
    throw StateError('请选择 .petx 桌宠扩展包: $path');
  }
  return stageDesktopPetPackage(path);
}

Future<String> stageDesktopPetPackage(String sourcePath) async {
  final source = File(sourcePath);
  if (!await source.exists()) {
    throw StateError('桌宠扩展包不存在: $sourcePath');
  }
  if (!Platform.isAndroid) return source.path;

  final support = await getApplicationSupportDirectory();
  final targetDirectory = Directory(
    '${support.path}/amitia/home/.amitia-imports/desktop-pet',
  );
  await targetDirectory.create(recursive: true);
  final fileName = Uri.file(sourcePath).pathSegments.last;
  final target = File(
    '${targetDirectory.path}/${DateTime.now().microsecondsSinceEpoch}-$fileName',
  );
  await source.copy(target.path);
  if (await source.length() != await target.length()) {
    throw StateError('桌宠扩展包暂存大小不一致: $sourcePath -> ${target.path}');
  }
  return '/home/amitia/.amitia-imports/desktop-pet/${Uri.file(target.path).pathSegments.last}';
}
