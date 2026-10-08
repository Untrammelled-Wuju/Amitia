package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type taskOwnerRPCKey struct{}
type taskOwnerRPC func(context.Context, string, string, json.RawMessage) (json.RawMessage, error)

func CallTaskOwner(ctx context.Context, requestID, method string, params json.RawMessage) (json.RawMessage, error) {
	call, ok := ctx.Value(taskOwnerRPCKey{}).(taskOwnerRPC)
	if !ok || call == nil {
		return nil, fmt.Errorf("任务缺少当前 Core 所有者通信端口")
	}
	return call(ctx, requestID, method, params)
}

func (c *MeshClient) callTaskOwner(ctx context.Context, dispatch protocol.TaskDispatchPayload, requestID, method string, params json.RawMessage) (json.RawMessage, error) {
	worker := NewTaskWorker(c)
	if err := worker.validateTaskConnection(dispatch); err != nil {
		return nil, err
	}
	authority, ok := coordination.FromContext(ctx)
	var advertised coordination.ExecutionScope
	owner := authority.TargetDeviceID
	if authority.Coordinated {
		owner = authority.CoreID
	}
	if !ok || owner == "" || authority.ResourceOwnerID != owner || authority.RoleOwnerID != owner || json.Unmarshal(dispatch.OwnedExecutionScope, &advertised) != nil || advertised != authority || authority.CoreID != c.conf.SpaceID.String() || dispatch.TaskGeneration < 1 || dispatch.AuthorityCallID == "" || dispatch.LeaseID == "" || requestID == "" || len(requestID) > 256 || len(params) > 1500<<10 || !json.Valid(params) || c.conf.SignRequest == nil {
		return nil, fmt.Errorf("任务所有者调用缺少一致的数据归属授权或执行身份")
	}
	switch method {
	case "task.storage.get", "task.storage.set", "task.storage.delete", "task.artifact.saveData", "task.artifact.saveFile", "task.artifact.list", "task.checkpoint.save", "task.progress.save", "task.host.executeTool", "task.host.emitEvent":
	default:
		return nil, fmt.Errorf("任务所有者操作未获授权")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	base, err := url.Parse(c.conf.CloudBaseURL)
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || dispatch.TaskRunID == "" || len(dispatch.TaskRunID) > 256 || strings.ContainsAny(dispatch.TaskRunID, "/\\\x00") {
		return nil, fmt.Errorf("Core 所有者地址或任务编号无效")
	}
	requestBody, err := json.Marshal(protocol.TaskOwnerRPCRequest{Scope: authority, AuthorityCallID: dispatch.AuthorityCallID, TaskGeneration: dispatch.TaskGeneration, AttemptID: dispatch.AttemptID, LeaseID: dispatch.LeaseID, SessionID: dispatch.RuntimeSessionID.String(), ConnectionGeneration: dispatch.ConnectionGeneration, RequestID: requestID, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	current, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(current, http.MethodPost, strings.TrimRight(base.String(), "/")+"/api/device-mesh/v1/business/tasks/"+url.PathEscape(dispatch.TaskRunID)+"/owner-rpc", bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "AmitiaDevice "+c.conf.Credential)
	if err := c.conf.SignRequest(request, c.conf.SpaceID.String()); err != nil {
		return nil, err
	}
	tlsConfig := c.dialer.TLSClientConfig.Clone()
	tlsConfig.MinVersion = tls.VersionTLS13
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Core 任务所有者操作 %s 未确认: %w", method, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (512<<10)+4097))
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if response.StatusCode != http.StatusOK || len(body) > (512<<10)+4096 || json.Unmarshal(body, &envelope) != nil || envelope.Code != 200 || !json.Valid(envelope.Data) || len(envelope.Data) > 512<<10 {
		return nil, fmt.Errorf("Core 任务所有者未确认操作，状态码 %d", response.StatusCode)
	}
	if err := worker.validateTaskConnection(dispatch); err != nil {
		return nil, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), envelope.Data...), nil
}
