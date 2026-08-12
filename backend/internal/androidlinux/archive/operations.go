//go:build linux && !android

package archive

const (
	OperationDetect  = "archive.detect"
	OperationList    = "archive.list"
	OperationExtract = "archive.extract"
	OperationCreate  = "archive.create"
	OperationVerify  = "archive.verify"
)
