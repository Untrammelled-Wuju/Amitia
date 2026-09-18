# Package 路径规则

## Action Config Path

Manifest 中的 Action Config 必须满足：

```
actions/{actionKey}/action.json
```

## Frame Path

Frame File 是相对于 Action JSON 目录：

```
frames/0000.webp
```

运行时解析：

```
ActionConfigPath Directory
+ Frame.File
→ Package Relative Path
→ ResourceIndex
→ amitia-pet:// URL
```

## 禁止

```
../
反斜杠
绝对路径
编码穿越
大小写别名
```

## 路径归一化规则

1. NFC Unicode 归一化
2. 禁止反斜杠和控制字符
3. 禁止盘符（如 C:）
4. 禁止绝对路径（以 / 开头）
5. filepath.Clean + ToSlash
6. 禁止 . 和空路径
7. 禁止 ../ 路径穿越
8. 逐段检查：禁止空段、. 段、.. 段
9. Windows 保留名（con/prn/aux/nul/com1-9/lpt1-9）
10. ADS 冒号检查
11. 尾部点或空格
12. 非 UTF-8 拒绝

## SecureJoinUnderRoot

先 NormalizePackagePath，再 filepath.Join，最后验证结果仍在 root 下。

## 大小写冲突检测

NormalizePackagePath 后 ToLower，用于大小写冲突检测。
