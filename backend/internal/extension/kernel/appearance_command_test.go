package kernel

import (
 "context"
 "strings"
 "testing"

 "github.com/u-ai/backend/internal/auth"
 "github.com/u-ai/backend/internal/runtimeidentity"
)

func TestAppearanceOriginRejectsMissingAndAdministrativeIdentity(t *testing.T) {
 cases:=[]struct{name string;actor *auth.ActorContext}{
 {"missing",nil},
 {"admin", &auth.ActorContext{PrincipalType:auth.PrincipalSystemWorker,SpaceID:runtimeidentity.SpaceID("space-a"),DeviceID:runtimeidentity.DeviceID("device-a"),SessionID:"session"}},
 {"automation", &auth.ActorContext{PrincipalType:auth.PrincipalAutomation,SpaceID:runtimeidentity.SpaceID("space-a"),DeviceID:runtimeidentity.DeviceID("device-a"),SessionID:"session"}},
 {"missing_device",&auth.ActorContext{PrincipalType:auth.PrincipalTrustedDevice,SpaceID:runtimeidentity.SpaceID("space-a"),SessionID:"session"}},
 {"missing_session",&auth.ActorContext{PrincipalType:auth.PrincipalTrustedDevice,SpaceID:runtimeidentity.SpaceID("space-a"),DeviceID:runtimeidentity.DeviceID("device-a")}},
 }
 for _,tc:=range cases {t.Run(tc.name,func(t *testing.T){ctx:=context.Background();if tc.actor!=nil{ctx=auth.WithActor(ctx,tc.actor)};if _,err:=appearanceOrigin(ctx);err==nil{t.Fatal("expected origin rejection")}})}
}

func TestAppearanceOriginAcceptsVerifiedLocalDevice(t *testing.T){
 actor:=&auth.ActorContext{PrincipalType:auth.PrincipalTrustedDevice,SpaceID:runtimeidentity.SpaceID("space-a"),DeviceID:runtimeidentity.DeviceID("device-a"),SessionID:"session"}
 resolved,err:=appearanceOrigin(auth.WithActor(context.Background(),actor))
 if err!=nil || resolved!=actor {t.Fatalf("origin rejected: %v",err)}
}

func TestAppearanceFailsClosedWithoutAuthenticatedOrigin(t *testing.T){
 notifier:=NewSSEUIHostNotifier(nil)
 _,err:=notifier.ExecuteAppearanceCommand(context.Background(),"reset",nil)
 if err==nil || !strings.Contains(err.Error(),"authenticated device"){t.Fatalf("expected unauthorized rejection: %v",err)}
}
