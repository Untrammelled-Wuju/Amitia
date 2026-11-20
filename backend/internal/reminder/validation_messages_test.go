package reminder

import (
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func TestReminderValidationMessagesAreFieldSpecific(t *testing.T) {
	service, _, _, _ := newReminderTestService(t)
	caller := Caller{SpaceID: "space", DeviceID: "device"}
	cases := []struct {
		name   string
		mutate func(*CreateReminderRequest)
		want   string
	}{
		{name: "empty title", mutate: func(r *CreateReminderRequest) { r.Title = "  " }, want: "任务标题不能为空"},
		{name: "missing character", mutate: func(r *CreateReminderRequest) { r.CharacterID = "" }, want: "所属角色不能为空"},
		{name: "content too long", mutate: func(r *CreateReminderRequest) { r.Content = strings.Repeat("x", (128<<10)+1) }, want: "任务内容过长"},
		{name: "title too long", mutate: func(r *CreateReminderRequest) { r.Title = strings.Repeat("x", 513) }, want: "任务标题过长"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := newReminderRequest()
			testCase.mutate(&request)
			if _, err := service.Create(t.Context(), caller, request, "manual"); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("err=%v, want contains %q", err, testCase.want)
			}
		})
	}
}

func TestReminderGenerationMismatchIsLocalizedAndKeepsCause(t *testing.T) {
	service, _, _, _ := newReminderTestService(t)
	caller := Caller{SpaceID: "space", DeviceID: "device"}
	item, err := service.Create(t.Context(), caller, newReminderRequest(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Change(t.Context(), caller, item.ID, item.Generation+99, "pause")
	if err == nil || !strings.Contains(err.Error(), "任务版本已变化") {
		t.Fatalf("err=%v", err)
	}
	if !errors.Is(err, schedule.ErrGenerationMismatch) {
		t.Fatalf("cause lost: %v", err)
	}
	_, err = service.TriggerNow(t.Context(), caller, item.ID, item.Generation+99)
	if err == nil || !strings.Contains(err.Error(), "任务版本已变化") || !errors.Is(err, schedule.ErrGenerationMismatch) {
		t.Fatalf("trigger err=%v", err)
	}
}
