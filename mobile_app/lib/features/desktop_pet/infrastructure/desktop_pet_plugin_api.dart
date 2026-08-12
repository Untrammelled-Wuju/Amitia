import '../../../core/backend_transport/backend_service_api.dart';
import 'desktop_pet_plugin_dto.dart';

class DesktopPetPluginApi {
  static const _basePath = '/api/extensions/desktop-pet/plugins';

  final BackendServiceApi _api;

  DesktopPetPluginApi(this._api);

  Future<DesktopPetPluginList> list({
    int page = 1,
    int pageSize = 20,
    String? search,
  }) async {
    final query = <String, String>{
      'page': '$page',
      'pageSize': '$pageSize',
    };
    if (search != null && search.isNotEmpty) {
      query['search'] = search;
    }
    final resp = await _api.get<Map<String, dynamic>>(
      _basePath,
      queryParameters: query,
    );
    if (resp == null) {
      return const DesktopPetPluginList(plugins: [], total: 0, page: 1, pageSize: 20);
    }
    return DesktopPetPluginList.fromJson(resp);
  }

  Future<DesktopPetPluginDetail> detail(String pluginId) async {
    final resp = await _api.get<Map<String, dynamic>>('$_basePath/$pluginId');
    if (resp == null) {
      return const DesktopPetPluginDetail(
        extensionId: '',
        pluginId: '',
        name: '',
        description: '',
        version: '',
        enabled: false,
        installState: '',
      );
    }
    return DesktopPetPluginDetail.fromJson(resp);
  }

  Future<DesktopPetPluginInstallResult> install(String packagePath) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/install',
      data: {'packagePath': packagePath},
    );
    if (resp == null) {
      return const DesktopPetPluginInstallResult(
        extensionId: '',
        version: '',
        installState: '',
      );
    }
    return DesktopPetPluginInstallResult.fromJson(resp);
  }

  Future<DesktopPetPluginInstallResult> update(String extensionId, String packagePath) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/$extensionId/update',
      data: {'packagePath': packagePath},
    );
    if (resp == null) {
      return const DesktopPetPluginInstallResult(
        extensionId: '',
        version: '',
        installState: '',
      );
    }
    return DesktopPetPluginInstallResult.fromJson(resp);
  }

  Future<DesktopPetPluginMutationResult> enable(String extensionId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/$extensionId/enable',
    );
    if (resp == null) {
      return const DesktopPetPluginMutationResult(extensionId: '', success: false);
    }
    return DesktopPetPluginMutationResult.fromJson(resp);
  }

  Future<DesktopPetPluginMutationResult> disable(String extensionId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/$extensionId/disable',
    );
    if (resp == null) {
      return const DesktopPetPluginMutationResult(extensionId: '', success: false);
    }
    return DesktopPetPluginMutationResult.fromJson(resp);
  }

  Future<DesktopPetPluginMutationResult> uninstall(String extensionId) async {
    await _api.delete('$_basePath/$extensionId');
    return DesktopPetPluginMutationResult(extensionId: extensionId, success: true);
  }
}
