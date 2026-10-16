// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package userfeedback

import (
	"fmt"
	"strings"
)

func renderPreviewText(payload map[string]any) string {
	builder := &strings.Builder{}
	field := func(label string, key string) {
		value := textValue(payload[key])
		if value == "" {
			value = "（未填写）"
		}
		fmt.Fprintf(builder, "%s：%s\n", label, value)
	}
	builder.WriteString("【提交内容】\n")
	field("反馈类型", "category")
	field("标题", "title")
	field("联系方式", "contact")
	description := textValue(payload["description"])
	if description == "" {
		description = "（未填写）"
	}
	fmt.Fprintf(builder, "问题描述：\n%s\n", description)

	builder.WriteString("\n【运行环境】\n")
	field("平台", "platform")
	field("客户端类型", "clientType")
	field("应用版本", "appVersion")
	field("Core 版本", "coreVersion")
	field("运行模式", "runtimeMode")
	field("系统信息", "osInfo")
	field("设备标识", "deviceHash")
	field("语言", "locale")
	field("时区", "timezone")

	builder.WriteString("\n【日志文件】\n")
	builder.WriteString(fmt.Sprintf("是否上传报错日志：%s\n", yesNo(payload["includeLogs"])))
	builder.WriteString(fmt.Sprintf("日志行数：%s\n", textValue(payload["logLineCount"])))
	files := sliceValue(payload["logFiles"])
	if len(files) == 0 {
		builder.WriteString("日志文件：无\n")
	} else {
		builder.WriteString("日志文件：\n")
		for _, name := range files {
			fmt.Fprintf(builder, "  - %s\n", name)
		}
	}

	errorLines := sliceValue(payload["errorLines"])
	builder.WriteString(fmt.Sprintf("\n【报错日志 %d 条】\n", len(errorLines)))
	if len(errorLines) == 0 {
		builder.WriteString("无\n")
	}
	for _, item := range errorLines {
		builder.WriteString(item + "\n")
	}

	clientErrors := sliceValue(payload["clientErrors"])
	builder.WriteString(fmt.Sprintf("\n【客户端异常 %d 条】\n", len(clientErrors)))
	if len(clientErrors) == 0 {
		builder.WriteString("无\n")
	}
	for _, item := range clientErrors {
		builder.WriteString(item + "\n")
	}

	builder.WriteString(fmt.Sprintf("\n脱敏处理：%s 处\n", textValue(payload["redactedCount"])))
	builder.WriteString(fmt.Sprintf("截图：%s\n", screenshotSummary(payload)))
	return builder.String()
}

func screenshotSummary(payload map[string]any) string {
	if !truthy(payload["hasScreenshot"]) {
		return "无"
	}
	name := textValue(payload["screenshotName"])
	if name == "" {
		name = "screenshot"
	}
	return fmt.Sprintf("%s（%s 字节）", name, textValue(payload["screenshotSize"]))
}

func yesNo(value any) string {
	if truthy(value) {
		return "是"
	}
	return "否"
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed != "" && typed != "false" && typed != "0"
	default:
		return false
	}
}

func textValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func sliceValue(value any) []string {
	items := []string{}
	switch typed := value.(type) {
	case []string:
		items = append(items, typed...)
	case []LogLine:
		for _, line := range typed {
			items = append(items, formatLogLine(line))
		}
	case []ClientError:
		for _, item := range typed {
			items = append(items, formatClientError(item))
		}
	case []any:
		for _, raw := range typed {
			switch entry := raw.(type) {
			case string:
				items = append(items, entry)
			case map[string]any:
				if line, ok := entry["line"]; ok {
					items = append(items, formatLogLine(LogLine{File: textValue(entry["file"]), Line: textValue(line)}))
					continue
				}
				items = append(items, strings.TrimSpace(textValue(entry["message"])))
			}
		}
	}
	return items
}

func formatLogLine(line LogLine) string {
	if line.File == "" {
		return line.Line
	}
	return fmt.Sprintf("[%s] %s", line.File, line.Line)
}

func formatClientError(item ClientError) string {
	head := strings.TrimSpace(item.Message)
	if item.Source != "" {
		head = strings.TrimSpace(item.Source + ": " + head)
	}
	if item.At != "" {
		head = item.At + " " + head
	}
	if strings.TrimSpace(item.Stack) == "" {
		return head
	}
	return head + "\n" + indent(item.Stack)
}

func indent(value string) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	for index := range lines {
		lines[index] = "    " + lines[index]
	}
	return strings.Join(lines, "\n")
}
