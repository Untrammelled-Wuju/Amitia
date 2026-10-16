from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = (ROOT / 'backend/cmd/server/workbench_browser_mesh.go').read_text(encoding='utf-8')

class DeviceMeshBrowserScopeGate(unittest.TestCase):
    def test_cloud_response_is_rechecked_after_invocation(self):
        after_call = SOURCE.split('result, err := services.DeviceMesh.InvokeDeviceHandlerWithRuntimeType', 1)[1]
        before_response = after_call.split('if len(result.Structured) == 0', 1)[0]
        self.assertIn('RequireTrustedDevice(c.Request.Context(), space, actor.DeviceID)', before_response)
        self.assertIn('RequireTrustedDevice(c.Request.Context(), space, target)', before_response)
        self.assertIn('http.StatusForbidden', before_response)

    def test_response_is_fenced_to_original_connection(self):
        self.assertIn('originalConnection, connected := services.DeviceMesh.Hub.GetByDevice(space, target)', SOURCE)
        after = SOURCE.split('InvokeDeviceHandlerWithRuntimeType(requestContext', 1)[1]
        self.assertIn('currentConnection, connected := services.DeviceMesh.Hub.GetByDevice(space, target)', after)
        self.assertIn('currentConnection != originalConnection', after)
        self.assertIn('"device_reconnected"', after)

    def test_repaired_device_credential_cannot_replay_old_observation(self):
        after = SOURCE.split('result, err := workbenchbrowser.ReadAgentTab', 1)[1]
        before = after.split('payload, err := json.Marshal(result)', 1)[0]
        self.assertIn('latest.CredentialID != credential.CredentialID', before)
        self.assertIn('subtle.ConstantTimeCompare([]byte(latest.Credential), []byte(credential.Credential)) != 1', before)
        self.assertIn('credential.CredentialID == ""', SOURCE)
        self.assertIn('credential.Credential == ""', SOURCE)

    def test_device_credential_rechecked_after_observation(self):
        after_read = SOURCE.split('result, err := workbenchbrowser.ReadAgentTab', 1)[1]
        before_marshal = after_read.split('payload, err := json.Marshal(result)', 1)[0]
        self.assertIn('latest, err := services.DeviceMesh.LocalHandler.LoadCredential()', before_marshal)
        self.assertIn('latest.SpaceID != invocation.SpaceID', before_marshal)
        self.assertIn('latest.DeviceID != invocation.DeviceID', before_marshal)
        self.assertIn('latest.RuntimeID != invocation.RuntimeID', before_marshal)
        self.assertIn('latest.ExpiresAt', before_marshal)

if __name__ == '__main__':
    unittest.main(verbosity=2)
