package git

import "errors"

var (
	ErrGitUnavailable           = errors.New("git unavailable")
	ErrRepositoryNotFound       = errors.New("git repository not found")
	ErrRepositoryUntrusted      = errors.New("git repository untrusted")
	ErrRepositoryInvalid        = errors.New("git repository invalid")

	ErrInternalMetadataDenied   = errors.New("git internal metadata denied")

	ErrHeadUnborn               = errors.New("git HEAD unborn")
	ErrDetachedHead             = errors.New("git detached HEAD")
	ErrGitDirty                 = errors.New("git dirty state")
	ErrGitConflictState         = errors.New("git conflict state")
	ErrGitDiverged              = errors.New("git branch diverged")

	ErrGitRefInvalid            = errors.New("git ref invalid")
	ErrBranchNotFound           = errors.New("git branch not found")
	ErrBranchExists             = errors.New("git branch already exists")

	ErrGitPathInvalid           = errors.New("git path invalid")
	ErrGitPathOutsideRepository = errors.New("git path outside repository")

	ErrGitStatusFailed          = errors.New("git status failed")
	ErrGitDiffFailed            = errors.New("git diff failed")
	ErrGitLogFailed             = errors.New("git log failed")

	ErrGitAddFailed             = errors.New("git add failed")
	ErrGitRestoreFailed         = errors.New("git restore failed")
	ErrGitCommitFailed          = errors.New("git commit failed")
	ErrGitCheckoutFailed        = errors.New("git checkout failed")

	ErrGitRemoteNotFound        = errors.New("git remote not found")
	ErrGitRemoteInvalid         = errors.New("git remote invalid")
	ErrGitRemoteAuthFailed      = errors.New("git remote authentication failed")
	ErrGitRemoteHostKeyChanged  = errors.New("git remote host key changed")
	ErrGitRemoteTLSFailed       = errors.New("git remote TLS failed")
	ErrGitRemoteNetworkFailed   = errors.New("git remote network failed")

	ErrGitFetchFailed           = errors.New("git fetch failed")
	ErrGitPullNotFastForward    = errors.New("git pull not fast forward")
	ErrGitPushRejected          = errors.New("git push rejected")
	ErrGitPushOutcomeUnknown    = errors.New("git push outcome unknown")

	ErrGitSubmoduleUnsupported  = errors.New("git submodule unsupported")
	ErrGitLFSUnsupported        = errors.New("git LFS unsupported")
	ErrGitSymlinkUnsupported    = errors.New("git symlink unsupported")
	ErrGitExternalFilterUnsupported = errors.New("git external filter unsupported")
	ErrGitHookExecutionDisabled = errors.New("git hook execution disabled")

	ErrGitOperationUnsupported  = errors.New("git operation unsupported")

	ErrIsolatedCreateFailed     = errors.New("isolated workspace create failed")
	ErrIsolatedTooLarge         = errors.New("isolated workspace too large")
	ErrIsolatedDirty            = errors.New("isolated workspace dirty")
	ErrIsolatedDeleteFailed     = errors.New("isolated workspace delete failed")
	ErrIsolatedCleanupPending   = errors.New("isolated workspace cleanup pending")

	ErrGitTimeout               = errors.New("git operation timeout")
	ErrGitCancelled             = errors.New("git operation cancelled")
)

type GitError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *GitError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *GitError) Unwrap() error {
	return e.Cause
}

func NewGitError(code string, message string, cause error) *GitError {
	return &GitError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
