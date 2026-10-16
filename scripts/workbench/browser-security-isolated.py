from pathlib import Path
import os
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / 'backend/internal/workbenchbrowser'
FILES = [
    'control_lease.go', 'control_lease_test.go', 'manual_mutation_test.go',
    'agent_bridge.go', 'agent_bridge_test.go',
    'remote_read.go', 'remote_read_test.go',
]
BROWSER_STUB = '''package browser
import "context"
type BrowserSessionID string
type BrowserTabID string
type BrowserErrorCode string
const (
 ErrCodeTabNotFound BrowserErrorCode = "tab_not_found"
 ErrCodeSessionNotFound BrowserErrorCode = "session_not_found"
 ErrCodeTabStale BrowserErrorCode = "tab_stale"
 ErrCodeSessionStale BrowserErrorCode = "session_stale"
 ErrCodeTabCrashed BrowserErrorCode = "tab_crashed"
)
type BrowserError struct { Code BrowserErrorCode }
func (e *BrowserError) Error() string { if e == nil { return "" }; return string(e.Code) }
type BrowserTab struct { URL string; Title string; State string }
type BrowserDOMSnapshot struct { URL string; Title string; Content string; Truncated bool }
type BrowserScreenshotRequest struct { SessionID BrowserSessionID; TabID BrowserTabID; Format string; Quality int }
type BrowserScreenshotBytes interface { ScreenshotBytes(context.Context, BrowserScreenshotRequest) ([]byte, *BrowserError) }
type BrowserTabs interface { GetTab(context.Context, BrowserSessionID, BrowserTabID) (BrowserTab, *BrowserError) }
type BrowserObserve interface { GetDOMSnapshot(context.Context, BrowserSessionID, BrowserTabID, int) (BrowserDOMSnapshot, *BrowserError) }
type BrowserProvider interface { Tabs() BrowserTabs; Observe() BrowserObserve; Resources() any }
type DisabledProvider struct{}
func NewDisabledProvider() BrowserProvider { return DisabledProvider{} }
func (DisabledProvider) Tabs() BrowserTabs { return nil }
func (DisabledProvider) Observe() BrowserObserve { return nil }
func (DisabledProvider) Resources() any { return nil }
'''
SCOPE_STUB = '''package workbenchbrowser
import "regexp"
var validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_:.-]{1,160}$`)
const maxBrowserDOMBytes = 1024 * 1024
func boundDOMContent(content string) (string, bool) { if len(content) > maxBrowserDOMBytes { return content[:maxBrowserDOMBytes], true }; return content, false }
'''

def main():
    if any(not (SOURCE / f).is_file() for f in FILES):
        raise SystemExit('missing source files')
    go = shutil.which('go')
    if not go:
        raise SystemExit('Go binary not found')
    with tempfile.TemporaryDirectory(prefix='uai-browser-security-') as td:
        base = Path(td)
        pkg = base / 'internal/workbenchbrowser'
        pkg.mkdir(parents=True)
        stub = base / 'internal/browser'
        stub.mkdir(parents=True)
        (base / 'go.mod').write_text('module github.com/u-ai/backend\n\ngo 1.23\n')
        (stub / 'provider.go').write_text(BROWSER_STUB)
        (pkg / 'scope_stub.go').write_text(SCOPE_STUB)
        for f in FILES:
            shutil.copyfile(SOURCE / f, pkg / f)
        env = dict(os.environ, GOTOOLCHAIN='local', GOPROXY='off')
        result = subprocess.run([go, 'test', '-race', '-count=1', '-v', './internal/workbenchbrowser'], cwd=base, env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        print(result.stdout, end='')
        print('browser source files:', ', '.join(FILES))
        return result.returncode

if __name__ == '__main__':
    sys.exit(main())
