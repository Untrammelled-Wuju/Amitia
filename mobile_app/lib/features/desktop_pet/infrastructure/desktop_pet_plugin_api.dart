import '../../../core/backend_transport/backend_service_api.dart';
import 'desktop_pet_plugin_dto.dart';

class DesktopPetPluginApi {
  static const _basePath = '/api/extensions/desktop-pet/plugins';

  final BackendServiceApi _api;

  DesktopPetPluginApi(this._api);

  String _id(String value) => Uri.encodeComponent(value.trim());

  T _requireData<T>(T? value, String operation) {
    if (value == null) {
      throw StateError('Desktop Pet Plugin $operation returned empty data');
    }
    return value;
  }

  Future<DesktopPetPluginList> list({
    int page = 1,
    int pageSize = 20,
    String? search,
  }) async {
    final query = <String, String>{
      'page': '$page',
      'pageSize': '$pageSize',
    };
    final normalized = search?.trim();
    if (normalized != null && normalized.isNotEmpty) {
      query['search'] = normalized;
    }
    final resp = await _api.get<Map<String, dynamic>>(
      _basePath,
      queryParameters: query,
    );
    return DesktopPetPluginList.fromJson(_requireData(resp, 'list'));
  }

  Future<DesktopPetPluginDetail> detail(String pluginId) async {
    final resp = await _api.get<Map<String, dynamic>>('$_basePath/${_id(pluginId)}');
    return DesktopPetPluginDetail.fromJson(_requireData(resp, 'detail'));
  }

  Future<DesktopPetPluginInstallResult> install(String packagePath) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/install',
      data: {'packagePath': packagePath},
    );
    return DesktopPetPluginInstallResult.fromJson(_requireData(resp, 'install'));
  }

  Future<DesktopPetPluginInstallResult> update(String extensionId, String packagePath) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/${_id(extensionId)}/update',
      data: {'packagePath': packagePath},
    );
    return DesktopPetPluginInstallResult.fromJson(_requireData(resp, 'update'));
  }

  Future<DesktopPetPluginMutationResult> enable(String extensionId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/${_id(extensionId)}/enable',
    );
    return DesktopPetPluginMutationResult.fromJson(_requireData(resp, 'enable'));
  }

  Future<DesktopPetPluginMutationResult> disable(String extensionId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/${_id(extensionId)}/disable',
    );
    return DesktopPetPluginMutationResult.fromJson(_requireData(resp, 'disable'));
  }

  Future<DesktopPetPluginMutationResult> uninstall(String extensionId) async {
    final resp = await _api.deleteWithResponse<Map<String, dynamic>>(
      '$_basePath/${_id(extensionId)}',
    );
    return DesktopPetPluginMutationResult.fromJson(_requireData(resp, 'uninstall'));
  }
}
