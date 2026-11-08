import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';
import 'package:amitia_app/features/settings/presentation/pages/device_add_page.dart';
import 'package:amitia_app/features/settings/presentation/pages/deployment_page.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('配对页优先使用路由中的统筹中心地址', () {
    const deployment = MobileDeploymentConfig(
      mode: MobileDeploymentMode.local,
      remoteCoreUri: 'https://deployment.example:7000',
    );
    final route = Uri.parse(
      '/settings/device-add?endpoint=https%3A%2F%2Froute.example%3A7001',
    );

    expect(pairingEndpointFor(deployment, route), 'https://route.example:7001');
  });

  test('配对页在没有路由地址时回退当前远程配置', () {
    const deployment = MobileDeploymentConfig(
      mode: MobileDeploymentMode.cloud,
      remoteCoreUri: 'https://center.example:7000',
    );

    expect(
      pairingEndpointFor(deployment, Uri.parse('/settings/device-add')),
      'https://center.example:7000',
    );
  });

  test('扫码和图片配对动作不依赖当前部署模式', () {
    expect(isPairingRouteAction('address'), isTrue);
    expect(isPairingRouteAction('scan'), isTrue);
    expect(isPairingRouteAction('image'), isTrue);
    expect(isPairingRouteAction('manual'), isFalse);
    expect(isPairingRouteAction(null), isFalse);
  });

  test('远程配对路由始终以子设备身份显示', () {
    const deployment = MobileDeploymentConfig(mode: MobileDeploymentMode.local);

    expect(isDevicePairingCenter(deployment, null), isTrue);
    expect(isDevicePairingCenter(deployment, 'address'), isFalse);
    expect(isDevicePairingCenter(deployment, 'scan'), isFalse);
    expect(isDevicePairingCenter(deployment, 'image'), isFalse);
  });

  test('手动地址发现指纹后使用路由中心地址发起相同配对', () {
    const deployment = MobileDeploymentConfig(mode: MobileDeploymentMode.local);
    final fingerprint = List.filled(64, 'a').join();
    final identity = pairingIdentityFor(
      deployment,
      Uri.parse(
        '/settings/device-add?endpoint=https%3A%2F%2Fcenter.example%3A7000'
        '&fingerprint=$fingerprint&coreId=space-a',
      ),
      '',
    );
    expect(identity.endpoint, 'https://center.example:7000');
    expect(identity.fingerprint, fingerprint);
    expect(identity.coreId, 'space-a');
  });

  test('云端模式地址变化允许重新选择统筹中心', () {
    const current = MobileDeploymentConfig(
      mode: MobileDeploymentMode.cloud,
      remoteCoreUri: 'https://190.160.0.10:54796',
    );

    expect(
      deploymentSelectionRequiresChange(
        current,
        MobileDeploymentMode.cloud,
        'https://172.16.176.114:54796',
      ),
      isTrue,
    );
  });

  test('相同统筹中心地址不会重复发起配对', () {
    const current = MobileDeploymentConfig(
      mode: MobileDeploymentMode.cloud,
      remoteCoreUri: 'https://center.example:7000',
    );

    expect(
      deploymentSelectionRequiresChange(
        current,
        MobileDeploymentMode.cloud,
        'https://center.example:7000/',
      ),
      isFalse,
    );
  });

  test('本机中心与远程中心之间可以切换', () {
    const current = MobileDeploymentConfig(mode: MobileDeploymentMode.local);

    expect(
      deploymentSelectionRequiresChange(
        current,
        MobileDeploymentMode.cloud,
        'https://center.example:7000',
      ),
      isTrue,
    );
  });
}
