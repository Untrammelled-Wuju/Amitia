import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/desktop_pet/infrastructure/desktop_pet_plugin_api.dart';
import 'package:amitia_app/features/desktop_pet/infrastructure/desktop_pet_plugin_dto.dart';
import 'package:amitia_app/features/desktop_pet/infrastructure/desktop_pet_package_picker.dart';
import 'package:amitia_app/features/desktop_pet/presentation/controllers/desktop_pet_plugin_controller_provider.dart';
import 'package:amitia_app/features/desktop_pet/presentation/sections/desktop_pet_plugin_section.dart';

class _StubPluginApi implements DesktopPetPluginApi {
  DesktopPetPluginList listResult = const DesktopPetPluginList(
    plugins: [
      DesktopPetPluginSummary(
        extensionId: 'ext-1',
        pluginId: 'plg-1',
        name: 'Pet Walker',
        description: 'Desc',
        version: '1.0.0',
        enabled: true,
        installState: 'installed',
      ),
      DesktopPetPluginSummary(
        extensionId: 'ext-2',
        pluginId: 'plg-2',
        name: 'Pet Talker',
        description: 'Desc2',
        version: '2.0.0',
        enabled: false,
        installState: 'installed',
      ),
    ],
    total: 2,
    page: 1,
    pageSize: 20,
  );
  bool failList = false;

  @override
  Future<DesktopPetPluginList> list({
    int page = 1,
    int pageSize = 20,
    String? search,
  }) async {
    if (failList) throw Exception('fail');
    return listResult;
  }

  @override
  Future<DesktopPetPluginDetail> detail(String pluginId) async {
    return DesktopPetPluginDetail(
      extensionId: 'ext-1',
      pluginId: pluginId,
      name: 'Detail',
      description: 'd',
      version: '1.0.0',
      enabled: true,
      installState: 'installed',
    );
  }

  @override
  Future<DesktopPetPluginInstallResult> install(String packagePath) async {
    return const DesktopPetPluginInstallResult(
      extensionId: 'ext-x',
      version: '1.0.0',
      installState: 'installed',
    );
  }

  @override
  Future<DesktopPetPluginInstallResult> update(
    String extensionId,
    String packagePath,
  ) async {
    return const DesktopPetPluginInstallResult(
      extensionId: 'ext-1',
      version: '1.1.0',
      installState: 'installed',
    );
  }

  @override
  Future<Map<String, dynamic>> previewPackage(
    String packagePath, {
    String expectedExtensionId = '',
  }) async {
    return const {
      'sessionId': 'preview-1',
      'id': 'com.amitia.qa/desktop-pet-plugin',
      'name': 'QA Desktop Pet',
      'version': '1.0.0',
      'managementTarget': 'pet_center',
      'compatible': true,
      'errors': <String>[],
      'warnings': <String>[],
      'highRiskCapabilities': <String>[],
      'capabilityConfirmations': <String>[],
      'signature': {'status': 'unsigned'},
      'scripts': 0,
      'assets': 1,
      'fileCount': 3,
    };
  }

  @override
  Future<String> commitPackagePreview(
    Map<String, dynamic> preview, {
    String expectedExtensionId = '',
  }) async {
    return 'operation-1';
  }

  @override
  Future<DesktopPetPluginMutationResult> enable(String extensionId) async {
    return const DesktopPetPluginMutationResult(
      extensionId: 'ext-1',
      success: true,
    );
  }

  @override
  Future<DesktopPetPluginMutationResult> disable(String extensionId) async {
    return const DesktopPetPluginMutationResult(
      extensionId: 'ext-1',
      success: true,
    );
  }

  @override
  Future<DesktopPetPluginMutationResult> uninstall(String extensionId) async {
    return const DesktopPetPluginMutationResult(
      extensionId: 'ext-1',
      success: true,
    );
  }
}

Widget _wrap(DesktopPetPluginApi api) {
  return ProviderScope(
    overrides: [
      desktopPetPluginApiProvider.overrideWithValue(api),
      desktopPetPackagePickerProvider.overrideWithValue(
        () async => '/tmp/qa.petx',
      ),
    ],
    child: const MaterialApp(
      home: Scaffold(
        body: SingleChildScrollView(child: DesktopPetPluginSection()),
      ),
    ),
  );
}

void main() {
  group('DesktopPetPluginSection widget', () {
    testWidgets('shows plugin names and versions', (tester) async {
      tester.view.physicalSize = const Size(420, 1200);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      await tester.pumpWidget(_wrap(_StubPluginApi()));
      await tester.pumpAndSettle();

      expect(find.text('Pet Walker'), findsOneWidget);
      expect(find.text('v1.0.0'), findsOneWidget);
      expect(find.text('Pet Talker'), findsOneWidget);
      expect(find.text('v2.0.0'), findsOneWidget);
    });

    testWidgets('shows enabled/disabled badges', (tester) async {
      await tester.pumpWidget(_wrap(_StubPluginApi()));
      await tester.pumpAndSettle();

      expect(find.text('已启用'), findsOneWidget);
      expect(find.text('已禁用'), findsOneWidget);
    });

    testWidgets('empty state shows install action', (tester) async {
      final api = _StubPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [],
          total: 0,
          page: 1,
          pageSize: 20,
        );
      await tester.pumpWidget(_wrap(api));
      await tester.pumpAndSettle();

      expect(find.text('尚未安装桌宠插件'), findsOneWidget);
      expect(find.text('安装插件'), findsOneWidget);
    });

    testWidgets('non-empty list has install button in header', (tester) async {
      await tester.pumpWidget(_wrap(_StubPluginApi()));
      await tester.pumpAndSettle();

      expect(find.text('安装'), findsOneWidget);
    });

    testWidgets('error state on load failure with retry', (tester) async {
      final api = _StubPluginApi()..failList = true;
      await tester.pumpWidget(_wrap(api));
      await tester.pumpAndSettle();

      expect(find.text('加载失败'), findsOneWidget);
    });

    testWidgets('tap plugin item opens detail sheet', (tester) async {
      await tester.pumpWidget(_wrap(_StubPluginApi()));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Pet Walker'));
      await tester.pumpAndSettle();

      expect(find.text('卸载插件'), findsOneWidget);
    });

    testWidgets('detail sheet has disable button for enabled plugin', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(_StubPluginApi()));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Pet Walker'));
      await tester.pumpAndSettle();

      expect(find.text('禁用'), findsOneWidget);
    });

    testWidgets('detail sheet shows enable button for disabled plugin', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(_StubPluginApi()));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Pet Talker'));
      await tester.pumpAndSettle();

      expect(find.text('启用'), findsOneWidget);
    });

    testWidgets('detail sheet has update button', (tester) async {
      await tester.pumpWidget(_wrap(_StubPluginApi()));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Pet Walker'));
      await tester.pumpAndSettle();

      expect(find.text('更新'), findsOneWidget);
    });

    testWidgets(
      'uninstall button shows confirmation dialog with correct text',
      (tester) async {
        await tester.pumpWidget(_wrap(_StubPluginApi()));
        await tester.pumpAndSettle();

        await tester.tap(find.text('Pet Walker'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('卸载插件'));
        await tester.pumpAndSettle();

        expect(find.text('确认卸载'), findsOneWidget);
        expect(find.textContaining('确定要卸载插件'), findsOneWidget);
      },
    );

    testWidgets('cancel dismisses uninstall dialog', (tester) async {
      await tester.pumpWidget(_wrap(_StubPluginApi()));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Pet Walker'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('卸载插件'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('取消'));
      await tester.pumpAndSettle();

      expect(find.text('确认卸载'), findsNothing);
    });

    testWidgets('install header button opens package confirmation', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(_StubPluginApi()));
      await tester.pumpAndSettle();

      await tester.tap(find.text('安装'));
      await tester.pumpAndSettle();

      expect(find.text('确认安装桌宠插件'), findsOneWidget);
      expect(find.text('QA Desktop Pet'), findsOneWidget);
      expect(find.text('查看完整预览信息'), findsOneWidget);
    });

    testWidgets('empty state install opens package confirmation', (
      tester,
    ) async {
      final api = _StubPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [],
          total: 0,
          page: 1,
          pageSize: 20,
        );
      await tester.pumpWidget(_wrap(api));
      await tester.pumpAndSettle();

      await tester.tap(find.text('安装插件'));
      await tester.pumpAndSettle();

      expect(find.text('确认安装桌宠插件'), findsOneWidget);
      expect(find.text('QA Desktop Pet'), findsOneWidget);
    });

    testWidgets('enable then refetch shows canonical enabled state', (
      tester,
    ) async {
      final api = _StubPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-2',
              pluginId: 'plg-2',
              name: 'Pet Talker',
              description: 'Desc2',
              version: '2.0.0',
              enabled: false,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );
      await tester.pumpWidget(_wrap(api));
      await tester.pumpAndSettle();

      expect(find.text('已禁用'), findsOneWidget);

      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'ext-2',
            pluginId: 'plg-2',
            name: 'Pet Talker',
            description: 'Desc2',
            version: '2.0.0',
            enabled: true,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      await tester.tap(find.text('Pet Talker'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('启用'));
      await tester.pumpAndSettle();

      expect(find.text('已启用'), findsOneWidget);
    });

    testWidgets('update then close sheet and refetch shows new version', (
      tester,
    ) async {
      final api = _StubPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Pet Walker',
              description: 'Desc',
              version: '1.0.0',
              enabled: true,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );
      await tester.pumpWidget(_wrap(api));
      await tester.pumpAndSettle();

      expect(find.text('v1.0.0'), findsOneWidget);

      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'ext-1',
            pluginId: 'plg-1',
            name: 'Pet Walker',
            description: 'Desc',
            version: '1.1.0',
            enabled: true,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      await tester.tap(find.text('Pet Walker'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('更新'));
      await tester.pumpAndSettle();
      expect(find.text('确认更新桌宠插件'), findsOneWidget);
      await tester.tap(find.text('我已查看签名、权限、风险和完整预览信息，并确认继续'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('确认更新'));
      await tester.pumpAndSettle();

      expect(find.text('v1.1.0'), findsOneWidget);
    });

    testWidgets('uninstall then refetch removes plugin from list', (
      tester,
    ) async {
      final api = _StubPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Pet Walker',
              description: 'Desc',
              version: '1.0.0',
              enabled: true,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );
      await tester.pumpWidget(_wrap(api));
      await tester.pumpAndSettle();

      expect(find.text('Pet Walker'), findsOneWidget);

      api.listResult = const DesktopPetPluginList(
        plugins: [],
        total: 0,
        page: 1,
        pageSize: 20,
      );

      await tester.tap(find.text('Pet Walker'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('卸载插件'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('卸载'));
      await tester.pumpAndSettle();

      expect(find.text('Pet Walker'), findsNothing);
      expect(find.text('尚未安装桌宠插件'), findsOneWidget);
    });

    testWidgets('install then refetch shows new plugin', (tester) async {
      final api = _StubPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [],
          total: 0,
          page: 1,
          pageSize: 20,
        );
      await tester.pumpWidget(_wrap(api));
      await tester.pumpAndSettle();

      expect(find.text('尚未安装桌宠插件'), findsOneWidget);

      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'ext-x',
            pluginId: 'plg-x',
            name: 'New Plugin',
            description: 'New',
            version: '1.0.0',
            enabled: false,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      await tester.tap(find.text('安装插件'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('我已查看签名、权限、风险和完整预览信息，并确认继续'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('确认安装'));
      await tester.pumpAndSettle();

      expect(find.text('New Plugin'), findsOneWidget);
      expect(find.text('尚未安装桌宠插件'), findsNothing);
    });
  });
}
