import 'dart:io';

import 'package:amitia_app/features/desktop_pet/infrastructure/desktop_pet_package_picker.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter_test/flutter_test.dart';

class _FakeFilePicker extends FilePicker {
  _FakeFilePicker(this.result);

  final FilePickerResult? result;
  FileType? requestedType;
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
    requestedMultiple = allowMultiple;
    return result;
  }
}

void main() {
  test('selects a petx package with the generic Android-safe picker', () async {
    final directory = await Directory.systemTemp.createTemp('petx-picker-');
    addTearDown(() => directory.delete(recursive: true));
    final package = File('${directory.path}/qa.petx')
      ..writeAsBytesSync([1, 2, 3]);
    final picker = _FakeFilePicker(
      FilePickerResult([
        PlatformFile(name: 'qa.petx', path: package.path, size: 3),
      ]),
    );
    FilePicker.platform = picker;
    addTearDown(() => FilePicker.platform = _FakeFilePicker(null));

    expect(await pickDesktopPetPackagePath(), package.path);
    expect(picker.requestedType, FileType.any);
    expect(picker.requestedMultiple, isFalse);
  });

  test('rejects a non-petx package with the original path', () async {
    final picker = _FakeFilePicker(
      FilePickerResult([
        PlatformFile(name: 'qa.zip', path: '/tmp/qa.zip', size: 3),
      ]),
    );
    FilePicker.platform = picker;
    addTearDown(() => FilePicker.platform = _FakeFilePicker(null));

    expect(
      pickDesktopPetPackagePath,
      throwsA(
        isA<StateError>().having(
          (error) => error.message,
          'message',
          allOf(contains('.petx'), contains('/tmp/qa.zip')),
        ),
      ),
    );
  });
}
