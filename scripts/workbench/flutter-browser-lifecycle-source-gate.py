from pathlib import Path
import unittest

source = Path(__file__).resolve().parents[2] / 'mobile_app/lib/features/workbench/presentation/pages/workbench_remote_browser_view.dart'
content = source.read_text(encoding='utf-8')

def section(start, end):
    return content.split(start, 1)[1].split(end, 1)[0]

class FlutterBrowserLifecycleSourceGate(unittest.TestCase):
    def test_old_resource_release_is_explicit(self):
        block = section('void didUpdateWidget(', 'void didChangeAppLifecycleState(')
        self.assertIn('_releaseControlFor(oldWidget.conversationId, oldWidget.tabId, oldWidget.targetDeviceId, leaseId: _leaseId)', block)
        self.assertIn('_generation++', block)
        self.assertIn('_domRequestGeneration++', block)
        self.assertIn('_controlRequestGeneration++', block)
        self.assertIn('_observed = false', block)
        self.assertIn("_leaseId = ''", block)
        self.assertIn('_frame = null', block)

    def test_dom_response_is_scoped_to_request(self):
        block = section('Future<void> _readDom()', 'void _click(')
        self.assertIn('final request = ++_domRequestGeneration', block)
        self.assertIn('sequence != _generation', block)
        self.assertIn('request != _domRequestGeneration', block)
        self.assertIn('!widget.active', block)
        self.assertIn('content is! String', block)

    def test_stale_control_acquire_releases_original_target(self):
        block = section('Future<void> _control(', 'void _reconnect(')
        self.assertIn('final api = ref.read(backendServiceProvider)', block)
        self.assertIn('final conversationId = widget.conversationId', block)
        self.assertIn('final tabId = widget.tabId', block)
        self.assertIn('final targetDeviceId = widget.targetDeviceId', block)
        self.assertIn('_releaseControlFor(conversationId, tabId, targetDeviceId, leaseId: responseLeaseId, api: api)', block)
        self.assertIn('responseLeaseId.isNotEmpty', block)
        self.assertIn('_leaseId != requestedLeaseId', block)
        self.assertIn("if (action != 'acquire') 'leaseId': requestedLeaseId", block)
        self.assertIn("'leaseId': leaseId", block)
        self.assertIn('if (request == _controlRequestGeneration) _controlPending = false', block)
        release = section('void _releaseControlFor(', 'void _reconnect(')
        self.assertNotIn('_canControl = false', release)

    def test_observed_mutations_forward_current_lease(self):
        navigation = section('void _navigate()', 'void _action(')
        action = section('void _action(', 'void _inputAction(')
        user_input = section('void _inputAction(', 'Future<void> _readDom()')
        for body in (navigation, action, user_input):
            self.assertIn('final token = _observed ? _leaseId : null', body)
            self.assertIn("if (token != null) 'leaseId': token", body)
            self.assertIn('expectedLeaseId: token', body)
        gate = section('Future<void> _execute(', 'void _navigate()')
        self.assertIn('expectedLeaseId != null && _leaseId != expectedLeaseId', gate)

    def test_remote_browser_does_not_enable_mutations(self):
        for start, end in [('void _navigate()', 'void _action('), ('void _action(', 'void _inputAction('), ('void _inputAction(', 'Future<void> _readDom()')]:
            self.assertIn('if (_isRemote) return;', section(start, end))

if __name__ == '__main__':
    unittest.main(verbosity=2)
