package platform

type RuntimePlatform interface {
	Name() string
	KillExistingServer(addr string) error
	ExecutableSuffix() string
	BinarySuffix() string
	RootFSDir() string
	DefaultDataDir() string
	IsWindows() bool
	IsLinux() bool
	IsAndroid() bool
	IsAndroidEmbedded() bool
	WritePidFile(dataDir string) error
	ReadPidFile(dataDir string) (int, error)
	RemovePidFile(dataDir string) error
}

var current RuntimePlatform

func Set(p RuntimePlatform) {
	current = p
}

func Get() RuntimePlatform {
	if current == nil {
		current = Detect()
	}
	return current
}
