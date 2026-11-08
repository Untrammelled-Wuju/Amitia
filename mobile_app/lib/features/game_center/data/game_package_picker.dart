import 'package:file_picker/file_picker.dart';

Future<String?> pickGamePackagePath() async {
  final result = await FilePicker.platform.pickFiles(allowMultiple: false);
  final path = result?.files.single.path;
  if (path == null || path.isEmpty) return null;
  if (!path.toLowerCase().endsWith('.gamex')) {
    throw StateError('请选择 .gamex 游戏扩展包');
  }
  return path;
}
