import 'ui_client_info_stub.dart'
    if (dart.library.io) 'ui_client_info_io.dart' as platform_info;

class UIClientInfo {
  final String architecture;
  final String appVersion;

  const UIClientInfo({required this.architecture, required this.appVersion});

  Map<String, dynamic> toQueryParameters() => <String, dynamic>{
        if (architecture.isNotEmpty) 'architecture': architecture,
        if (appVersion.isNotEmpty) 'appVersion': appVersion,
      };
}

const _appVersion = String.fromEnvironment(
  'AMITIA_APP_VERSION',
  defaultValue: '1.0.0',
);

UIClientInfo currentUIClientInfo() => UIClientInfo(
      architecture: platform_info.currentArchitecture(),
      appVersion: _appVersion,
    );
