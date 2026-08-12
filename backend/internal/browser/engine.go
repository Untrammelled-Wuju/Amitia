package browser

type BrowserEngineFactory interface {
	Create(config BrowserConfig) (BrowserEngine, error)
}

type engineFactory struct {
	executableResolver BrowserExecutableResolver
}

func NewBrowserEngineFactory() BrowserEngineFactory {
	return &engineFactory{}
}

func (f *engineFactory) Create(config BrowserConfig) (BrowserEngine, error) {
	if f.executableResolver == nil {
		f.executableResolver = NewBrowserExecutableResolver(config)
	}
	return NewChromiumEngine(config, f.executableResolver), nil
}
