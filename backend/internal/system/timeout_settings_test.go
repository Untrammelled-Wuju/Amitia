package system

import (
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"testing"
)

func TestOperationTimeoutPersistence(t *testing.T) {
	restore := timeoutpolicy.Configure(timeoutpolicy.Default())
	defer restore()
	svc := newSetupServiceTest(t)
	want := timeoutpolicy.Settings{Disabled: true, Seconds: 450}
	if err := svc.UpdateTimeoutSettings(want); err != nil {
		t.Fatal(err)
	}
	timeoutpolicy.Configure(timeoutpolicy.Default())
	(&service{db: svc.db}).loadTimeoutSettings()
	got, _ := timeoutpolicy.Current()
	if got != want {
		t.Fatalf("settings did not survive reload: %+v", got)
	}
	if err := svc.UpdateTimeoutSettings(timeoutpolicy.Settings{Seconds: 31}); err == nil {
		t.Fatal("invalid settings accepted")
	}
	got, _ = timeoutpolicy.Current()
	if got != want {
		t.Fatal("invalid save changed active settings")
	}
	svc.setAppSetting(timeoutSettingsKey, "invalid")
	svc.loadTimeoutSettings()
	got, _ = timeoutpolicy.Current()
	if got != timeoutpolicy.Default() {
		t.Fatal("corrupt stored settings did not recover defaults")
	}
}
