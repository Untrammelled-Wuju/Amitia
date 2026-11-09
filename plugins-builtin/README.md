# Amitia 固定业务插件

本目录只存放随 Amitia 分发的固定业务插件源码。

这些插件与普通插件使用完全相同的 Manifest、Adapter、权限、Service Graph、安装事务和生命周期。目录分离只表示发布归属，不形成第二套运行时。

约束：

- Manifest 必须声明 `system.defaultInstall` 和 `system.defaultEnabled`。
- 新增固定业务插件只能放入本目录。
- 普通、官方市场、示例和第三方插件不得放入本目录。
- 构建产物统一写入项目根目录 `plugin/`，不得将生成包放入源码目录。
