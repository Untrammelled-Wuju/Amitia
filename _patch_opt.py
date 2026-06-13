import os

filepath = r"D:\桌面\跟进项目\U-Ai\backend\internal\tts\engine.go"
with open(filepath, "r", encoding="utf-8") as f:
    content = f.read()

# Make resourceId optional - only send if explicitly set
old = '	if resourceId != "" {\n\t\treq.Header.Set("X-Api-Resource-Id", resourceId)\n\t}\n\treq.Header.Set("X-Api-Request-Id", uuid.New().String())\n\n\tclient := &http.Client{Timeout: 120 * time.Second}\n\tresp, err := client.Do(req)\n\tif err != nil {\n\t\treturn nil, fmt.Errorf("音色复刻请求失败: %w", err)\n\t}\n\tdefer resp.Body.Close()\n\n\trawBody, _ := io.ReadAll(resp.Body)\n\tif resp.StatusCode != 200 {\n\t\treturn nil, fmt.Errorf("音色复刻返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 300))'

new = '	if resourceId != "" {\n\t\treq.Header.Set("X-Api-Resource-Id", resourceId)\n\t}\n\treq.Header.Set("X-Api-Request-Id", uuid.New().String())\n\n\tclient := &http.Client{Timeout: 120 * time.Second}\n\tresp, err := client.Do(req)\n\tif err != nil {\n\t\treturn nil, fmt.Errorf("音色复刻请求失败: %w", err)\n\t}\n\tdefer resp.Body.Close()\n\n\trawBody, _ := io.ReadAll(resp.Body)\n\tif resp.StatusCode != 200 {\n\t\treturn nil, fmt.Errorf("音色复刻返回 %d: %s", resp.StatusCode, truncateStr(string(rawBody), 300))'

content = content.replace(old, new)

with open(filepath, "w", encoding="utf-8") as f:
    f.write(content)

print("Resource-Id now optional for voice clone")
