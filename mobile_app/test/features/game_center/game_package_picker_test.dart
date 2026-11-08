import 'package:amitia_app/features/game_center/data/game_package_picker.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter_test/flutter_test.dart';

class _FakeFilePicker extends FilePicker {
  _FakeFilePicker(this.result);

  final FilePickerResult? result;
  FileType? requestedType;
  List<String>? requestedExtensions;
  bool? requestedMultiple;

  @override
  Future<FilePickerResult?> pickFiles({
    String? dialogTitle,
    String? initialDirectory,
    FileType type = FileType.any,
    List<String>? allowedExtensions,
    Function(FilePickerStatus)? onFileLoading,
    bool allowCompression = true,
    int compressionQuality = 30,
    bool allowMultiple = false,
    bool withData = false,
    bool withReadStream = false,
    bool lockParentWindow = false,
    bool readSequential = false,
  }) async {
    requestedType = type;
    requestedExtensions = allowedExtensions;
    requestedMultiple = allowMultiple;
    return result;
  }
}

void main() {
  test('uses Android-safe picker and accepts gamex package', () async {
    final picker = _FakeFilePicker(
      FilePickerResult([
        PlatformFile(
          name: 'world-game-plugin.gamex',
          path: '/tmp/world-game-plugin.gamex',
          size: 1,
        ),
      ]),
    );
    FilePicker.platform = picker;
    addTearDown(() => FilePicker.platform = _FakeFilePicker(null));

    expect(await pickGamePackagePath(), '/tmp/world-game-plugin.gamex');
    expect(picker.requestedType, FileType.any);
    expect(picker.requestedExtensions, isNull);
    expect(picker.requestedMultiple, isFalse);
  });

  test('rejects a non-gamex selection with the original reason', () async {
    final picker = _FakeFilePicker(
      FilePickerResult([
        PlatformFile(
          name: 'world-game-plugin.zip',
          path: '/tmp/world-game-plugin.zip',
          size: 1,
        ),
      ]),
    );
    FilePicker.platform = picker;
    addTearDown(() => FilePicker.platform = _FakeFilePicker(null));

    expect(
      pickGamePackagePath,
      throwsA(
        isA<StateError>().having(
          (error) => error.message,
          'message',
          '请选择 .gamex 游戏扩展包',
        ),
      ),
    );
  });
}
