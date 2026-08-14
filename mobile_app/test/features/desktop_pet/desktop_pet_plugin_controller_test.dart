import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/desktop_pet/infrastructure/desktop_pet_plugin_api.dart';
import 'package:amitia_app/features/desktop_pet/infrastructure/desktop_pet_plugin_dto.dart';
import 'package:amitia_app/features/desktop_pet/presentation/controllers/desktop_pet_plugin_controller_provider.dart';

class _FakeDesktopPetPluginApi implements DesktopPetPluginApi {
  DesktopPetPluginList listResult = const DesktopPetPluginList(
    plugins: [],
    total: 0,
    page: 1,
    pageSize: 20,
  );

  int listCallCount = 0;
  int detailCallCount = 0;
  int enableCallCount = 0;
  int disableCallCount = 0;
  int uninstallCallCount = 0;
  int installCallCount = 0;
  int updateCallCount = 0;

  bool failList = false;
  bool failDetail = false;
  bool failEnable = false;
  bool failDisable = false;
  bool failUninstall = false;
  bool failInstall = false;
  bool failUpdate = false;
  Duration delay = Duration.zero;

  @override
  Future<DesktopPetPluginList> list({int page = 1, int pageSize = 20, String? search}) async {
    listCallCount++;
    if (delay > Duration.zero) {
      await Future.delayed(delay);
    }
    if (failList) {
      throw Exception('list failed');
    }
    return listResult;
  }

  @override
  Future<DesktopPetPluginDetail> detail(String pluginId) async {
    detailCallCount++;
    if (failDetail) {
      throw Exception('detail failed');
    }
    return listResult.plugins
        .firstWhere((p) => p.pluginId == pluginId,
            orElse: () => const DesktopPetPluginSummary(
                  extensionId: '',
                  pluginId: '',
                  name: '',
                  description: '',
                  version: '',
                  enabled: false,
                  installState: '',
                ))
        .let((p) => DesktopPetPluginDetail(
              extensionId: p.extensionId,
              pluginId: p.pluginId,
              name: p.name,
              description: p.description,
              version: p.version,
              enabled: p.enabled,
              installState: p.installState,
            ));
  }

  @override
  Future<DesktopPetPluginInstallResult> install(String packagePath) async {
    installCallCount++;
    if (failInstall) {
      throw Exception('install failed');
    }
    return const DesktopPetPluginInstallResult(
      extensionId: 'ext-x',
      version: '1.0.0',
      installState: 'installed',
    );
  }

  @override
  Future<DesktopPetPluginInstallResult> update(String extensionId, String packagePath) async {
    updateCallCount++;
    if (failUpdate) {
      throw Exception('update failed');
    }
    return DesktopPetPluginInstallResult(
      extensionId: extensionId,
      version: '1.1.0',
      installState: 'installed',
    );
  }

  @override
  Future<DesktopPetPluginMutationResult> enable(String extensionId) async {
    enableCallCount++;
    if (failEnable) {
      throw Exception('enable failed');
    }
    return DesktopPetPluginMutationResult(extensionId: extensionId, success: true);
  }

  @override
  Future<DesktopPetPluginMutationResult> disable(String extensionId) async {
    disableCallCount++;
    if (failDisable) {
      throw Exception('disable failed');
    }
    return DesktopPetPluginMutationResult(extensionId: extensionId, success: true);
  }

  @override
  Future<DesktopPetPluginMutationResult> uninstall(String extensionId) async {
    uninstallCallCount++;
    if (failUninstall) {
      throw Exception('uninstall failed');
    }
    return DesktopPetPluginMutationResult(extensionId: extensionId, success: true);
  }
}

extension _Let<T> on T {
  R let<R>(R Function(T) f) => f(this);
}

void main() {
  group('DesktopPetPluginController', () {
    test('load populates plugins on success', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Plugin A',
              description: 'Desc A',
              version: '1.0.0',
              enabled: true,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );

      final controller = DesktopPetPluginController(api: api);

      await controller.load();

      expect(controller.state.plugins.length, 1);
      expect(controller.state.plugins.first.name, 'Plugin A');
      expect(controller.state.loading, false);
      expect(controller.state.error, isNull);
      expect(api.listCallCount, 1);
    });

    test('load sets error on failure', () async {
      final api = _FakeDesktopPetPluginApi()..failList = true;
      final controller = DesktopPetPluginController(api: api);

      await controller.load();

      expect(controller.state.plugins, isEmpty);
      expect(controller.state.loading, false);
      expect(controller.state.error, isNotNull);
    });

    test('load sets loading flag during request', () async {
      final api = _FakeDesktopPetPluginApi()
        ..delay = const Duration(milliseconds: 50)
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Plugin',
              description: '',
              version: '1.0',
              enabled: false,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );

      final controller = DesktopPetPluginController(api: api);

      final future = controller.load();
      expect(controller.state.loading, true);
      await future;
      expect(controller.state.loading, false);
    });

    test('refresh sets refreshing flag during request', () async {
      final api = _FakeDesktopPetPluginApi()
        ..delay = const Duration(milliseconds: 50)
        ..listResult = const DesktopPetPluginList(
          plugins: [],
          total: 0,
          page: 1,
          pageSize: 20,
        );

      final controller = DesktopPetPluginController(api: api);

      final future = controller.refresh();
      expect(controller.state.refreshing, true);
      await future;
      expect(controller.state.refreshing, false);
    });

    test('stale load response is discarded after newer load', () async {
      final api = _FakeDesktopPetPluginApi()
        ..delay = const Duration(milliseconds: 100)
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'old-ext',
              pluginId: 'old-plg',
              name: 'Old Plugin',
              description: '',
              version: '1.0',
              enabled: false,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );

      final controller = DesktopPetPluginController(api: api);

      final firstLoad = controller.load();

      await Future.delayed(const Duration(milliseconds: 20));

      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'new-ext',
            pluginId: 'new-plg',
            name: 'New Plugin',
            description: '',
            version: '2.0',
            enabled: true,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      await controller.load();

      await firstLoad;

      expect(controller.state.plugins.length, 1);
      expect(controller.state.plugins.first.name, 'New Plugin');
    });

    test('stale refresh response is discarded after newer refresh', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'init',
              pluginId: 'init-plg',
              name: 'Initial',
              description: '',
              version: '1.0',
              enabled: false,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );

      final controller = DesktopPetPluginController(api: api);
      await controller.load();

      api.delay = const Duration(milliseconds: 100);
      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'old',
            pluginId: 'old',
            name: 'Old',
            description: '',
            version: '1.0',
            enabled: false,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      final firstRefresh = controller.refresh();

      await Future.delayed(const Duration(milliseconds: 20));

      api.delay = Duration.zero;
      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'new',
            pluginId: 'new',
            name: 'New',
            description: '',
            version: '2.0',
            enabled: true,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      await controller.refresh();
      await firstRefresh;

      expect(controller.state.plugins.first.name, 'New');
    });

    test('enable calls api and triggers refetch', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Plugin',
              description: '',
              version: '1.0',
              enabled: false,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );
      final controller = DesktopPetPluginController(api: api);
      await controller.load();

      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'ext-1',
            pluginId: 'plg-1',
            name: 'Plugin',
            description: '',
            version: '1.0',
            enabled: true,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      final result = await controller.enable('plg-1', 'ext-1');

      expect(result, true);
      expect(api.enableCallCount, 1);
      expect(api.listCallCount, 2);
      expect(controller.state.plugins.first.enabled, true);
    });

    test('disable calls api and triggers refetch', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Plugin',
              description: '',
              version: '1.0',
              enabled: true,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );
      final controller = DesktopPetPluginController(api: api);
      await controller.load();

      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'ext-1',
            pluginId: 'plg-1',
            name: 'Plugin',
            description: '',
            version: '1.0',
            enabled: false,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      final result = await controller.disable('plg-1', 'ext-1');

      expect(result, true);
      expect(api.disableCallCount, 1);
      expect(api.listCallCount, 2);
      expect(controller.state.plugins.first.enabled, false);
    });

    test('uninstall calls api and removes plugin via refetch', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Plugin',
              description: '',
              version: '1.0',
              enabled: true,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );
      final controller = DesktopPetPluginController(api: api);
      await controller.load();
      expect(controller.state.plugins.isNotEmpty, true);

      api.listResult = const DesktopPetPluginList(
        plugins: [],
        total: 0,
        page: 1,
        pageSize: 20,
      );

      final result = await controller.uninstall('plg-1', 'ext-1');

      expect(result, true);
      expect(api.uninstallCallCount, 1);
      expect(api.listCallCount, 2);
      expect(controller.state.plugins, isEmpty);
    });

    test('operation state is set during operation and cleared after', () async {
      final api = _FakeDesktopPetPluginApi()
        ..delay = const Duration(milliseconds: 50)
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Plugin',
              description: '',
              version: '1.0',
              enabled: false,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );

      final controller = DesktopPetPluginController(api: api);

      final future = controller.enable('plg-1', 'ext-1');

      expect(controller.hasOperation('plg-1'), true);
      expect(controller.state.operationByPluginId.contains('plg-1'), true);

      final _ = await future;

      expect(controller.hasOperation('plg-1'), false);
      expect(controller.state.operationByPluginId.contains('plg-1'), false);
    });

    test('operation state is cleared even on failure', () async {
      final api = _FakeDesktopPetPluginApi()..failEnable = true;
      final controller = DesktopPetPluginController(api: api);

      final result = await controller.enable('plg-1', 'ext-1');

      expect(result, false);
      expect(controller.hasOperation('plg-1'), false);
    });

    test('generation increments on each load', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [],
          total: 0,
          page: 1,
          pageSize: 20,
        );

      final controller = DesktopPetPluginController(api: api);
      expect(controller.state.generation, 0);

      await controller.load();
      expect(controller.state.generation, 1);

      await controller.load();
      expect(controller.state.generation, 2);
    });

    test('detail returns null on error', () async {
      final api = _FakeDesktopPetPluginApi()..failDetail = true;
      final controller = DesktopPetPluginController(api: api);

      final d = await controller.detail('plg-1');

      expect(d, isNull);
      expect(api.detailCallCount, 1);
    });

    test('error from load is cleared on subsequent successful load', () async {
      final api = _FakeDesktopPetPluginApi()
        ..failList = true;

      final controller = DesktopPetPluginController(api: api);
      await controller.load();
      expect(controller.state.error, isNotNull);

      api
        ..failList = false
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Recovered',
              description: '',
              version: '1.0',
              enabled: true,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );

      await controller.load();
      expect(controller.state.error, isNull);
      expect(controller.state.plugins.first.name, 'Recovered');
    });

    test('install calls api and triggers refetch', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [],
          total: 0,
          page: 1,
          pageSize: 20,
        );
      final controller = DesktopPetPluginController(api: api);
      await controller.load();
      expect(controller.state.plugins, isEmpty);

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

      final result = await controller.install('/pkgs/new.zip');

      expect(result, true);
      expect(api.installCallCount, 1);
      expect(api.listCallCount, 2);
      expect(controller.state.plugins.length, 1);
      expect(controller.state.plugins.first.pluginId, 'plg-x');
      expect(controller.state.installing, false);
    });

    test('install sets installing flag during operation and clears after', () async {
      final api = _FakeDesktopPetPluginApi()
        ..delay = const Duration(milliseconds: 50)
        ..listResult = const DesktopPetPluginList(
          plugins: [],
          total: 0,
          page: 1,
          pageSize: 20,
        );
      final controller = DesktopPetPluginController(api: api);

      final future = controller.install('/pkgs/foo.zip');
      expect(controller.state.installing, true);
      await future;
      expect(controller.state.installing, false);
    });

    test('install rejects empty path', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(plugins: [], total: 0, page: 1, pageSize: 20);
      final controller = DesktopPetPluginController(api: api);

      final result = await controller.install('  ');

      expect(result, false);
      expect(api.installCallCount, 0);
    });

    test('install rejects concurrent calls', () async {
      final api = _FakeDesktopPetPluginApi()
        ..delay = const Duration(milliseconds: 50)
        ..listResult = const DesktopPetPluginList(plugins: [], total: 0, page: 1, pageSize: 20);
      final controller = DesktopPetPluginController(api: api);

      final first = controller.install('/pkgs/1.zip');
      final second = controller.install('/pkgs/2.zip');

      await first;
      await second;

      expect(api.installCallCount, 1);
    });

    test('update calls api and triggers refetch', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Plugin',
              description: '',
              version: '1.0.0',
              enabled: true,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );
      final controller = DesktopPetPluginController(api: api);
      await controller.load();
      expect(controller.state.plugins.first.version, '1.0.0');

      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'ext-1',
            pluginId: 'plg-1',
            name: 'Plugin',
            description: '',
            version: '1.2.0',
            enabled: true,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      final result = await controller.update('plg-1', 'ext-1', '/pkgs/v2.zip');

      expect(result, true);
      expect(api.updateCallCount, 1);
      expect(api.listCallCount, 2);
      expect(controller.state.plugins.first.version, '1.2.0');
    });

    test('enable refetch with canonical state wins over mutation response', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Plugin',
              description: '',
              version: '1.0',
              enabled: false,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );
      final controller = DesktopPetPluginController(api: api);
      await controller.load();
      expect(controller.state.plugins.first.enabled, false);

      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'ext-1',
            pluginId: 'plg-1',
            name: 'Plugin',
            description: '',
            version: '1.0',
            enabled: false,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      final result = await controller.enable('plg-1', 'ext-1');

      expect(result, true);
      expect(controller.state.plugins.first.enabled, false);
    });

    test('uncertain failure refetch uses backend truth', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Plugin',
              description: '',
              version: '1.0',
              enabled: false,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );
      final controller = DesktopPetPluginController(api: api);
      await controller.load();

      api.failEnable = true;
      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'ext-1',
            pluginId: 'plg-1',
            name: 'Plugin',
            description: '',
            version: '1.0',
            enabled: true,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      final result = await controller.enable('plg-1', 'ext-1');

      expect(result, false);
      expect(controller.state.plugins.first.enabled, true);
    });

    test('refetch failure keeps old snapshot and sets error', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Plugin',
              description: '',
              version: '1.0',
              enabled: false,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );
      final controller = DesktopPetPluginController(api: api);
      await controller.load();
      expect(controller.state.plugins.length, 1);

      api
        ..failList = true
        ..listResult = const DesktopPetPluginList(
          plugins: [
            DesktopPetPluginSummary(
              extensionId: 'ext-1',
              pluginId: 'plg-1',
              name: 'Plugin',
              description: '',
              version: '1.0',
              enabled: true,
              installState: 'installed',
            ),
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        );

      final result = await controller.enable('plg-1', 'ext-1');

      expect(result, true);
      expect(controller.state.operationByPluginId.contains('plg-1'), false);
      expect(controller.state.error, isNotNull);
    });

    test('install mutation response does not patch local state directly', () async {
      final api = _FakeDesktopPetPluginApi()
        ..listResult = const DesktopPetPluginList(
          plugins: [],
          total: 0,
          page: 1,
          pageSize: 20,
        );
      final controller = DesktopPetPluginController(api: api);
      await controller.load();

      api.listResult = const DesktopPetPluginList(
        plugins: [
          DesktopPetPluginSummary(
            extensionId: 'ext-x',
            pluginId: 'plg-x',
            name: 'New',
            description: '',
            version: '1.0.0',
            enabled: false,
            installState: 'installed',
          ),
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      );

      await controller.install('/pkgs/foo.zip');

      expect(controller.state.plugins.length, 1);
      expect(controller.state.plugins.first.pluginId, 'plg-x');
    });
  });
}
