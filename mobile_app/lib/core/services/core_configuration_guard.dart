import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend_transport/backend_service_api.dart';
import '../backend_transport/core_configuration_intent.dart';
import '../backend_transport/providers/backend_transport_providers.dart';
import '../runtime/backend/mobile_backend_providers.dart';
import '../runtime/backend/mobile_deployment_mode.dart';
import 'providers.dart';

final coreConfigurationAccessProvider =
    FutureProvider.autoDispose<CoreConfigurationIntent>((ref) {
      ref.watch(mobileDeploymentConfigProvider);
      ref.watch(rawBackendServiceApiProvider);
      return CoreConfigurationGuard(
        deployment: () => ref.read(mobileDeploymentConfigProvider),
        api: () => ref.read(rawBackendServiceApiProvider),
        coordination: () => ref.read(deviceMeshServiceProvider).coordination(),
      ).capture();
    });

CoreConfigurationGuard coreConfigurationGuardFor(WidgetRef ref) =>
    CoreConfigurationGuard(
      deployment: () => ref.read(mobileDeploymentConfigProvider),
      api: () => ref.read(rawBackendServiceApiProvider),
      coordination: () => ref.read(deviceMeshServiceProvider).coordination(),
    );

class CoreConfigurationGuard {
  final MobileDeploymentConfig Function() deployment;
  final BackendServiceApi? Function() api;
  final Future<Map<String, dynamic>> Function() coordination;

  const CoreConfigurationGuard({
    required this.deployment,
    required this.api,
    required this.coordination,
  });

  Map<String, dynamic> _policyData(Map<String, dynamic> response) =>
      response['data'] is Map
      ? Map<String, dynamic>.from(response['data'] as Map)
      : response;

  String _revision(Map policy) {
    final values = [
      policy['providerEpoch'],
      policy['modeRevision'],
      policy['permissionRevision'],
    ];
    if (values.any((value) =>
        value is! int || value < 1 || value > 9007199254740991)) {
      throw StateError('云端配置权限版本无法确认，请恢复 Core 连接后重试');
    }
    return values.join(':');
  }

  Future<CoreConfigurationIntent> capture() async {
    final initialDeployment = deployment();
    final initialApi = api();
    if (initialApi == null || initialApi.generation <= 0) {
      throw StateError('无法确认模型配置归属，请恢复 Core 连接后重试');
    }
    bool stillCurrent() {
      final current = deployment();
      return current.mode == initialDeployment.mode &&
          current.remoteCoreUri == initialDeployment.remoteCoreUri &&
          identical(api(), initialApi);
    }

    String? coreId;
    String? policyRevision;
    var canConfigure = true;
    if (initialDeployment.mode == MobileDeploymentMode.cloud) {
      final data = _policyData(await coordination());
      final policy = data['policy'];
      coreId = (data['coreId'] ?? '').toString();
      if (data['coordinationAvailable'] != true ||
          coreId.isEmpty ||
          policy is! Map ||
          policy['coordinated'] is! bool ||
          data['canAdminister'] is! bool) {
        throw StateError('无法确认云端配置权限，请恢复 Core 连接后重试');
      }
      canConfigure =
          policy['coordinated'] == true && data['canAdminister'] == true;
      policyRevision = _revision(policy);
    }
    if (!stillCurrent()) {
      throw StateError('Core 已切换，请重新加载模型配置');
    }
    return CoreConfigurationIntent(
      generation: initialApi.generation,
      coreId: coreId,
      policyRevision: policyRevision,
      canConfigure: canConfigure,
      isCurrent: stillCurrent,
    );
  }

  Future<void> validate(CoreConfigurationIntent intent) async {
    intent.validateGeneration(api()?.generation ?? 0);
    if (intent.coreId != null) {
      final data = _policyData(await coordination());
      final policy = data['policy'];
      if (data['coordinationAvailable'] != true ||
          data['coreId'] != intent.coreId ||
          data['canAdminister'] != true ||
          policy is! Map ||
          policy['coordinated'] != true ||
          _revision(policy) != intent.policyRevision) {
        throw StateError('云端配置权限已失效，请重新加载模型配置');
      }
    }
    intent.validateGeneration(api()?.generation ?? 0);
  }
}
