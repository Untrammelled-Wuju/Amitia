package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/devicemesh/business"
)

func ownedRecoveryRuntime(status string)*conversationstream.ActiveTurnState{return &conversationstream.ActiveTurnState{Blocks:[]conversationstream.BlockState{{BlockSequence:1,Type:"tool_call",CallID:"completed-call",Status:status,Payload:map[string]any{"toolName":"workspace.read","arguments":`{"path":"a.go"}`,"result":"source already read"}}}}}

func TestOwnedToolRecoveryRestoresCompletedResultWithoutRepeatingEffect(t *testing.T){
	runtime:=&ownedLoopTestRuntime{definitions:[]tool.Tool{{Function:tool.Function{Name:"workspace.read"}}}}
	s:=&service{ownedToolRuntime:runtime}
	called:=false
	s.llmWithTools=func(_ context.Context,_ *ModelConfig,messages []map[string]interface{},definitions []tool.Tool)(string,string,[]map[string]interface{},int,error){
		called=true
		if len(messages)!=3 || messages[1]["role"]!="assistant" || messages[2]["role"]!="tool" || messages[2]["content"]!="source already read" {t.Fatalf("confirmed transcript missing: %+v",messages)}
		if strings.Contains(definitions[0].Function.Name,"."){t.Fatal("resumed tool alias invalid")}
		return "resume complete","",nil,7,nil
	}
	inference:=business.Inference{RecoveredRuntime:ownedRecoveryRuntime("completed")}
	result,err:=s.generateOwnedWithTools(t.Context(),inference,&ModelConfig{ContextWindow:500000},[]map[string]interface{}{{"role":"system","content":"run task"}},&ownedReplySink{inference:inference})
	if err!=nil || result.Text!="resume complete" || !called {t.Fatalf("resume failed: %+v %v",result,err)}
}

func TestOwnedToolRecoveryRejectsUnresolvedMalformedAndDuplicateHistory(t *testing.T){
	for _,scenario:=range []string{"running","invalid_arguments","missing_result","duplicate","unlisted"}{t.Run(scenario,func(t *testing.T){
		runtime:=ownedRecoveryRuntime("completed")
		switch scenario {case "running":runtime.Blocks[0].Status="running";case "invalid_arguments":runtime.Blocks[0].Payload["arguments"]="{broken";case "missing_result":delete(runtime.Blocks[0].Payload,"result");case "duplicate":runtime.Blocks=append(runtime.Blocks,runtime.Blocks[0]);case "unlisted":runtime.Blocks[0].Payload["toolName"]="forbidden_action"}
		s:=&service{ownedToolRuntime:&ownedLoopTestRuntime{definitions:[]tool.Tool{{Function:tool.Function{Name:"workspace.read"}}}}}
		called:=false;s.llmWithTools=func(context.Context,*ModelConfig,[]map[string]interface{},[]tool.Tool)(string,string,[]map[string]interface{},int,error){called=true;return "unsafe","",nil,0,nil}
		inference:=business.Inference{RecoveredRuntime:runtime}
		_,err:=s.generateOwnedWithTools(t.Context(),inference,&ModelConfig{},nil,&ownedReplySink{})
		if err==nil || called {t.Fatalf("unconfirmed history reached provider: called=%v err=%v",called,err)}
	})}
}
