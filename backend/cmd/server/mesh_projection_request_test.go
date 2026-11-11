package main

import (
	"os"
	"strings"
	"testing"
)

func TestProjectionHandlerBindsRequestToLocalDevice(t *testing.T) {
	raw, err := os.ReadFile("mesh_business_router.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "projectionHandler := func(c *gin.Context) {")
	if start < 0 {
		t.Fatal("未找到 projectionHandler，无法校验投影请求绑定")
	}
	block := source[start:]
	if end := strings.Index(block, `mesh.GET("/projections", projectionHandler)`); end > 0 {
		block = block[:end]
	}
	if !strings.Contains(block, "TargetDeviceID: actor.DeviceID.String()") {
		t.Fatal("投影请求未把目标设备绑定为本机设备")
	}
	if strings.Contains(block, "TargetDeviceID: coreID") {
		t.Fatal("投影请求把统筹中心标识当作目标设备")
	}
	if strings.Contains(block, "ResourceOwnerID:") {
		t.Fatal("投影请求自行指定资源所有者，与数据读取端点的归属解析不一致")
	}
}
