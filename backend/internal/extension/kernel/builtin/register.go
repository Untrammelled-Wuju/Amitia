package builtin

// AllBuiltinRegistrations 返回所有 Built-in Extension 的注册函数列表
func AllBuiltinRegistrations() []func(c *Catalog) error {
	return []func(c *Catalog) error{
		func(c *Catalog) error { return c.Register(BuildSearchExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildDeepSearchExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildBrowserExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildWorkspaceExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildUIAgentExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildTTSExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildASRExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildMediaExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildImageExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildBackgroundRemovalExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildVisionExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildMemoryExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildProfileExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildEpisodicExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildWorldBookExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildCompanionExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildGameHostExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildDesktopPetExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildWebChannelExtension("1.0.0")) },
		func(c *Catalog) error { return c.Register(BuildQQChannelExtension("1.0.0")) },
		func(c *Catalog) error {
			return c.Register(BuildWechatChannelExtension("1.0.0"))
		},
		RegisterDefaultAIModels,
	}
}

// ApplyBuiltinRegistrations 将所有 Built-in Extension 注册到 catalog
func ApplyBuiltinRegistrations(c *Catalog) error {
	for _, reg := range AllBuiltinRegistrations() {
		if err := reg(c); err != nil {
			return err
		}
	}
	return nil
}

// RegisterDefaultAIModels 注册所有默认 AI 提供商 Extension
func RegisterDefaultAIModels(c *Catalog) error {
	providers := []string{"openai", "anthropic", "gemini", "ollama"}
	for _, p := range providers {
		if err := c.Register(BuildAIExtension(p, "1.0.0")); err != nil {
			return err
		}
	}
	return nil
}
