package com.amitia.runtime.extension.security

sealed class PackageSecurityException(message: String) : RuntimeException(message)

class InvalidArchiveException(message: String) : PackageSecurityException(message)

class PathTraversalException(message: String) : PackageSecurityException(message)

class AbsolutePathException(message: String) : PackageSecurityException(message)

class PathTooLongException(message: String) : PackageSecurityException(message)

class PathDepthExceededException(message: String) : PackageSecurityException(message)

class WindowsReservedNameException(message: String) : PackageSecurityException(message)

class SizeLimitExceededException(message: String) : PackageSecurityException(message)

class EntryCountExceededException(message: String) : PackageSecurityException(message)

class CompressionRatioExceededException(message: String) : PackageSecurityException(message)

class SymlinkNotAllowedException(message: String) : PackageSecurityException(message)

class SpecialFileNotAllowedException(message: String) : PackageSecurityException(message)

class NestedArchiveNotAllowedException(message: String) : PackageSecurityException(message)

class ForbiddenFileTypeException(message: String) : PackageSecurityException(message)

class ExecutableNotAllowedException(message: String) : PackageSecurityException(message)

class DuplicatePathException(message: String) : PackageSecurityException(message)

class UnicodeCollisionException(message: String) : PackageSecurityException(message)

class SignatureInvalidException(message: String) : PackageSecurityException(message)

class SignatureExpiredException(message: String) : PackageSecurityException(message)

class UnknownPublisherException(message: String) : PackageSecurityException(message)

class PublisherBlockedException(message: String) : PackageSecurityException(message)

class InvalidTrustLevelException(message: String) : PackageSecurityException(message)

class IntegrityMismatchException(message: String) : PackageSecurityException(message)

class SignatureMissingException(message: String) : PackageSecurityException(message)

class UnsupportedSignatureAlgorithmException(message: String) : PackageSecurityException(message)

class KeyRevokedException(message: String) : PackageSecurityException(message)

class KeyExpiredException(message: String) : PackageSecurityException(message)

class NonUtf8PathException(message: String) : PackageSecurityException(message)

class InvalidStructureException(message: String) : PackageSecurityException(message)

class ManifestMissingException(message: String) : PackageSecurityException(message)

class IntegrityMissingException(message: String) : PackageSecurityException(message)
