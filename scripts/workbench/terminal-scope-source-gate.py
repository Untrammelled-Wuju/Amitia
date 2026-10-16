from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
TERMINAL = ROOT / "backend/internal/workbenchterminal"

class TerminalScopeGate(unittest.TestCase):
    def test_authenticated_actor_identity_is_scoped_by_all_components(self):
        source = (TERMINAL / "handler.go").read_text(encoding="utf-8")
        self.assertIn('actor.DeviceID == ""', source)
        self.assertIn('actor.PrincipalType != auth.PrincipalLocalUI', source)
        self.assertIn('!actor.IsLocalTrusted', source)
        self.assertIn('terminalOwnerKey(string(actor.SpaceID), string(actor.DeviceID), string(actor.RuntimeID), actor.SessionID, conversationID)', source)

    def test_sensitive_terminal_payloads_are_not_cached(self):
        handler = (TERMINAL / "handler.go").read_text(encoding="utf-8")
        stream = (TERMINAL / "stream.go").read_text(encoding="utf-8")
        self.assertIn('c.Header("Cache-Control", "no-store, private")', handler.split('func (h *Handler) read(c *gin.Context) {', 1)[1].split('func (h *Handler) input', 1)[0])
        self.assertIn('c.Header("Cache-Control", "no-store, private")', stream.split('func (h *Handler) streamTicket(c *gin.Context) {', 1)[1].split('func (h *Handler) stream(c *gin.Context) {', 1)[0])

    def test_stream_cursor_is_checked_before_consuming_ticket_and_upgrading(self):
        source = (TERMINAL / "stream.go").read_text(encoding="utf-8")
        entry = source.split('func (h *Handler) stream(c *gin.Context) {', 1)[1]
        parsed = entry.index('parseTerminalStreamCursor(')
        consumed = entry.index('h.tickets.consume(')
        upgraded = entry.index('upgrade.Upgrade(')
        self.assertLess(parsed, consumed)
        self.assertLess(consumed, upgraded)
        self.assertIn('h.respondError(c, ErrInvalid)', entry[parsed:consumed])

if __name__ == '__main__':
    unittest.main(verbosity=2)
