class BuildError(Exception):
    pass


class ValidationError(BuildError):
    pass


class PublishError(BuildError):
    pass


class HashMismatchError(BuildError):
    pass


class ArchiveError(BuildError):
    pass


class VersionConflictError(BuildError):
    pass


class TreeManifestError(BuildError):
    pass
