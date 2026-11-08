import 'dart:async';

import 'package:amitia_app/core/runtime/backend/mobile_backend_lifecycle.dart';
import 'package:amitia_app/core/runtime/backend/mobile_backend_providers.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class _FakeBackendLifecycle implements MobileBackendLifecycle {
  _FakeBackendLifecycle(this.currentStatus);

  @override
  final MobileBackendStatus currentStatus;

  @override
  Stream<MobileBackendStatus> get statusStream =>
      const Stream<MobileBackendStatus>.empty();

  @override
  Future<void> reconcile(MobileDeploymentConfig config) async {}

  @override
  void pauseAutomaticReconciliation() {}

  @override
  void resumeAutomaticReconciliation() {}

  @override
  Future<void> shutdown() async {}
}

void main() {
  test('状态流立即发布当前生命周期快照', () async {
    const status = MobileBackendStatus(
      mode: MobileDeploymentMode.local,
      state: RuntimeDeploymentState.ready,
      businessCore: BackendNodeStatus(
        state: RuntimeDeploymentState.ready,
        baseUri: null,
      ),
      generation: 2,
    );
    final container = ProviderContainer(
      overrides: [
        mobileBackendLifecycleProvider.overrideWithValue(
          _FakeBackendLifecycle(status),
        ),
      ],
    );
    addTearDown(container.dispose);

    final value = await container.read(mobileBackendStatusProvider.future);

    expect(value.state, RuntimeDeploymentState.ready);
    expect(value.generation, 2);
  });

  test('统筹中心故障时显示聊天不可达横幅', () {
    const failed = MobileBackendStatus(
      mode: MobileDeploymentMode.cloud,
      state: RuntimeDeploymentState.failed,
      businessCore: BackendNodeStatus(
        state: RuntimeDeploymentState.failed,
        message: 'remote core not reachable',
      ),
      localRuntime: BackendNodeStatus(
        state: RuntimeDeploymentState.ready,
        profile: 'device-agent',
      ),
      generation: 3,
    );
    const localFailure = MobileBackendStatus(
      mode: MobileDeploymentMode.local,
      state: RuntimeDeploymentState.failed,
      businessCore: BackendNodeStatus(state: RuntimeDeploymentState.failed),
      generation: 4,
    );

    expect(shouldShowMobileBackendUnavailableBanner(failed), isTrue);
    expect(shouldShowMobileBackendUnavailableBanner(localFailure), isFalse);
    expect(shouldShowMobileBackendUnavailableBanner(null), isFalse);
  });
}
