class BuildError(Exception):
    def __init__(self, message: str, code: str = "BUILD_ERROR"):
        super().__init__(message)
        self.code = code
        self.message = message


class ValidationError(BuildError):
    def __init__(self, message: str):
        super().__init__(message, "VALIDATION_ERROR")


class ArtifactNotFoundError(BuildError):
    def __init__(self, message: str):
        super().__init__(message, "ARTIFACT_NOT_FOUND")


class ChecksumMismatchError(BuildError):
    def __init__(self, message: str):
        super().__init__(message, "CHECKSUM_MISMATCH")


class VersionPolicyError(BuildError):
    def __init__(self, message: str):
        super().__init__(message, "VERSION_POLICY_ERROR")
