package packages

import "time"

type PackagesPolicy struct {
	Enabled                       bool          `json:"enabled"`
	AptEnabled                    bool          `json:"aptEnabled"`
	PythonEnabled                 bool          `json:"pythonEnabled"`
	NodeEnabled                   bool          `json:"nodeEnabled"`
	DefaultInvokeTimeout          time.Duration `json:"defaultInvokeTimeout"`
	MaxInvokeTimeout              time.Duration `json:"maxInvokeTimeout"`
	DefaultInstallTimeout         time.Duration `json:"defaultInstallTimeout"`
	MaxInstallTimeout             time.Duration `json:"maxInstallTimeout"`
	StatusTimeout                 time.Duration `json:"statusTimeout"`
	DefaultOutputLimit            int64         `json:"defaultOutputLimit"`
	InstallNoRecommends           bool          `json:"installNoRecommends"`
	UseAptGet                     bool          `json:"useAptGet"`
	PythonVenvBaseDir             string        `json:"pythonVenvBaseDir"`
	NodePackageBaseDir            string        `json:"nodePackageBaseDir"`
}

func DefaultPackagesPolicy() PackagesPolicy {
	return PackagesPolicy{
		Enabled:               true,
		AptEnabled:            true,
		PythonEnabled:         true,
		NodeEnabled:           true,
		DefaultInvokeTimeout:  30 * time.Second,
		MaxInvokeTimeout:      2 * time.Minute,
		DefaultInstallTimeout: 5 * time.Minute,
		MaxInstallTimeout:     10 * time.Minute,
		StatusTimeout:         10 * time.Second,
		DefaultOutputLimit:    1 * 1024 * 1024,
		InstallNoRecommends:   false,
		UseAptGet:             true,
		PythonVenvBaseDir:     "packages/python/default",
		NodePackageBaseDir:    "packages/node/default",
	}
}
