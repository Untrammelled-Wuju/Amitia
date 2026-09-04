import 'package:shared_preferences/shared_preferences.dart';
import 'mobile_deployment_mode.dart';

abstract interface class MobileDeploymentConfigRepository {
  Future<MobileDeploymentConfig> load();
  Future<void> save(MobileDeploymentConfig config);
}

class SharedPreferencesMobileDeploymentConfigRepository
    implements MobileDeploymentConfigRepository {
  static const String _keyMode = 'deployment_mode';
  static const String _keyRemoteUri = 'deployment_remote_core_uri';

  const SharedPreferencesMobileDeploymentConfigRepository();

  @override
  Future<MobileDeploymentConfig> load() async {
    final prefs = await SharedPreferences.getInstance();
    final modeRaw = prefs.getString(_keyMode);
    final remoteUri = prefs.getString(_keyRemoteUri);
    return MobileDeploymentConfig(
      mode: MobileDeploymentMode.fromStorage(modeRaw),
      remoteCoreUri: remoteUri,
    );
  }

  @override
  Future<void> save(MobileDeploymentConfig config) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_keyMode, config.mode.storageValue);
    if (config.remoteCoreUri != null && config.remoteCoreUri!.isNotEmpty) {
      await prefs.setString(_keyRemoteUri, config.remoteCoreUri!);
    } else {
      await prefs.remove(_keyRemoteUri);
    }
  }
}
