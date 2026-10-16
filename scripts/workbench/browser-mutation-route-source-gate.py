from pathlib import Path
import re
import unittest

root = Path(__file__).resolve().parents[2]
handler = (root / 'backend/internal/workbenchbrowser/handler.go').read_text(encoding='utf-8')
lease = (root / 'backend/internal/workbenchbrowser/control_lease.go').read_text(encoding='utf-8')

class BrowserMutationRouteGate(unittest.TestCase):
    def test_every_mutation_checks_lease_token(self):
        for operation, end in [('navigate', 'action'), ('action', 'screenshot'), ('input', 'isFinite')]:
            start = f'func (h *Handler) {operation}(c *gin.Context)'
            method = handler.split(start, 1)[1].split(f'func (h *Handler) {end}(', 1)[0] if operation != 'input' else handler.split(start, 1)[1].split('func isFinite(', 1)[0]
            self.assertRegex(method, r'LeaseID\s+string\s+`json:"leaseId"`')
            self.assertIn('h.beginBrowserMutation(c, record, req.LeaseID)', method)
        self.assertIn('BeginManualTabMutation(record.observerRef, key, token, record.session, record.tab)', handler)

    def test_non_observed_tabs_do_not_require_a_lease(self):
        body = handler.split('func (h *Handler) beginBrowserMutation(', 1)[1].split('func (h *Handler) control(', 1)[0]
        self.assertIn('if !record.observed {', body)
        self.assertIn('return func() {}, true', body)

    def test_current_lease_generation_required_in_atomic_write_gate(self):
        body = lease.split('func BeginManualTabMutation(', 1)[1].split('func currentAgentControl(', 1)[0]
        self.assertIn('token == ""', body)
        self.assertIn('lease.leaseID != token', body)
        self.assertIn('agentControlLeases.manualInFlight[key]', body)

if __name__ == '__main__':
    unittest.main(verbosity=2)
