package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestNativeDispatcherPreservesAuthorizedScopeAndParentCancellation(t *testing.T) {
	for _, chained:=range []bool{false,true} {
		d:=NewRuntimeDispatcher()
		scope:=coordination.ExecutionScope{CoreID:"core",AuthorizationRealm:"core",TargetDeviceID:"source",RoleID:"role",RoleRevision:2,RequestID:"native-request"}
		started:=make(chan coordination.ExecutionScope,1)
		d.RegisterCancellable("task.host.source-event",func(ctx context.Context,_ protocol.RuntimeInvokePayload)(*protocol.RuntimeResultPayload,error){
			actual,_:=coordination.FromContext(ctx);started<-actual
			<-ctx.Done();return nil,context.Cause(ctx)
		})
		var dispatcher RuntimeContextDispatcher=d
		if chained {dispatcher=NewChainedRuntimeDispatcher(NewRuntimeDispatcher(),d).(RuntimeContextDispatcher)}
		ctx,cancel:=context.WithCancel(coordination.WithScope(t.Context(),scope))
		owned,_:=json.Marshal(scope)
		finished:=make(chan error,1)
		go func(){_,err:=dispatcher.ResolveContext("task.host.source-event")(ctx,protocol.RuntimeInvokePayload{InvocationID:"native-context",DeadlineMs:5000,OwnedExecutionScope:owned});finished<-err}()
		if actual:=<-started;actual!=scope {cancel();t.Fatalf("authorized Native scope discarded: %+v",actual)}
		cancel()
		if err:=<-finished;!errors.Is(err,context.Canceled) {t.Fatalf("parent cancellation did not stop Native dispatcher: %v",err)}
	}
}
