package kernel

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/u-ai/backend/internal/auth"
    "github.com/u-ai/backend/internal/extension/kernel/host_registry"
)

func appearanceOrigin(ctx context.Context) (*auth.ActorContext,error) {
    actor,ok:=auth.FromContext(ctx)
    if !ok || actor==nil {return nil,fmt.Errorf("appearance modification requires authenticated device context")}
    switch actor.PrincipalType {
    case auth.PrincipalLocalUI,auth.PrincipalTrustedDevice,auth.PrincipalDeviceRuntime:
    default: return nil,fmt.Errorf("appearance modification is forbidden for non-device principals")
    }
    if strings.TrimSpace(string(actor.SpaceID))=="" || strings.TrimSpace(string(actor.DeviceID))=="" ||
       (strings.TrimSpace(actor.SessionID)=="" && strings.TrimSpace(string(actor.RuntimeSessionID))=="") {
        return nil,fmt.Errorf("appearance modification requires authenticated originating device and session")
    }
    return actor,nil
}

func (n *SSEUIHostNotifier) ExecuteAppearanceCommand(ctx context.Context, action string, changes map[string]interface{}) (map[string]interface{},error) {
    actor,err:=appearanceOrigin(ctx)
    if err!=nil{return nil,err}
    switch action {case "inspect","apply","reset","undo":default:return nil,fmt.Errorf("unsupported appearance action %q",action)}
    if n.hostRegistry==nil || n.hub==nil{return nil,ErrUIHostUnavailable}
    hosts,err:=n.hostRegistry.ListReadyHostsString(ctx,string(actor.SpaceID),host_registry.CapUINotify)
    if err!=nil{return nil,err}
    var selected *host_registry.HostEntry
    for _,host:=range hosts {
        if host==nil || host.DeviceID!=actor.DeviceID || host.SpaceID!=actor.SpaceID ||
           !n.hub.ClientExists(host.HostClientID) {continue}
        if selected!=nil{return nil,fmt.Errorf("multiple UI host sessions exist for this device; refuse ambiguous appearance target")}
        selected=host
    }
    if selected==nil{return nil,fmt.Errorf("the originating device does not have an active UI host session")}
    commandID:="ui-appearance-"+uuid.NewString()
    pending:=&pendingClientRuntimeCommand{
        responseCh:make(chan clientRuntimeHostResponse,1),
        allowedHosts:map[string]string{selected.HostClientID:selected.HostSessionID},
        responded:make(map[string]struct{}),
    }
    n.mu.Lock()
    n.pendingClientRuntimeCommands[commandID]=pending
    n.mu.Unlock()
    defer func(){n.mu.Lock();delete(n.pendingClientRuntimeCommands,commandID);n.mu.Unlock()}()
    payload:=map[string]interface{}{
        "commandId":commandID,
        "hostClientId":selected.HostClientID,
        "hostSessionId":selected.HostSessionID,
        "action":action,
        "changes":changes,
        "platform":string(selected.Platform),
        "deviceId":string(actor.DeviceID),
    }
    envelope:=NewSSEEventEnvelope("ui_appearance_command","com.amitia.builtin.uiagent",payload,90*time.Second)
    envelopeMap:=envelope.ToMap()
    envelopeMap["hostClientId"]=selected.HostClientID
    envelopeMap["hostSessionId"]=selected.HostSessionID
    n.hub.SendToClient(selected.HostClientID,"ui_appearance_command",envelopeMap)
    wait,cancel:=context.WithTimeout(ctx,85*time.Second)
    defer cancel()
    select {
    case response:=<-pending.responseCh:
        if response.Error!=""{return nil,fmt.Errorf("appearance command failed: %s",response.Error)}
        if response.Result==nil || response.Result["ok"]!=true{return nil,fmt.Errorf("appearance command was not applied")}
        return response.Result,nil
    case <-wait.Done():return nil,fmt.Errorf("appearance command response unavailable: %w",wait.Err())
    }
}
