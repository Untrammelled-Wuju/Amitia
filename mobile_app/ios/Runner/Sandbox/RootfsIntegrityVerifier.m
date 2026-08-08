#import "RootfsIntegrityVerifier.h"
#import <CommonCrypto/CommonDigest.h>

NSErrorDomain const RootfsIntegrityErrorDomain = @"com.amitia.RootfsIntegrityVerifier";

static const size_t kSHA256BufferSize = 1024 * 1024;

@implementation RootfsIntegrityVerifier

+ (BOOL)isValidSHA256Digest:(NSString *)digest {
    if (!digest || digest.length != 64) return NO;
    NSCharacterSet *hexSet = [NSCharacterSet characterSetWithCharactersInString:@"0123456789abcdef"];
    return [digest rangeOfCharacterFromSet:[hexSet invertedSet]].location == NSNotFound;
}

+ (NSString *)lowercaseHexString:(NSString *)digest {
    return digest.lowercaseString;
}

+ (RootfsIntegrityResult)verifySHA256OfFileAtURL:(NSURL *)fileURL
                                   againstExpected:(NSString *)expectedDigest
                                             error:(NSError *_Nullable *_Nullable)error {
    if (!fileURL || !fileURL.isFileURL) {
        if (error) *error = [NSError errorWithDomain:RootfsIntegrityErrorDomain code:RootfsIntegrityResultFileMissing userInfo:nil];
        return RootfsIntegrityResultFileMissing;
    }
    if (![[NSFileManager defaultManager] fileExistsAtPath:fileURL.path]) {
        if (error) *error = [NSError errorWithDomain:RootfsIntegrityErrorDomain code:RootfsIntegrityResultFileMissing userInfo:nil];
        return RootfsIntegrityResultFileMissing;
    }
    if (![self isValidSHA256Digest:expectedDigest]) {
        if (error) *error = [NSError errorWithDomain:RootfsIntegrityErrorDomain code:RootfsIntegrityResultDigestMalformed userInfo:nil];
        return RootfsIntegrityResultDigestMalformed;
    }

    NSString *actual = [self computeSHA256OfFileAtURL:fileURL error:error];
    if (!actual) return RootfsIntegrityResultIOError;

    return [[actual isEqualToString:expectedDigest.lowercaseString]] ? RootfsIntegrityResultMatch : RootfsIntegrityResultMismatch;
}

+ (RootfsIntegrityResult)verifySHA256OfData:(NSData *)data
                              againstExpected:(NSString *)expectedDigest {
    if (!data || data.length == 0) return RootfsIntegrityResultIOError;
    if (![self isValidSHA256Digest:expectedDigest]) return RootfsIntegrityResultDigestMalformed;
    NSString *actual = [self sha256OfData:data];
    return [[actual isEqualToString:expectedDigest.lowercaseString]] ? RootfsIntegrityResultMatch : RootfsIntegrityResultMismatch;
}

+ (NSString *_Nullable)computeSHA256OfFileAtURL:(NSURL *)fileURL
                                          error:(NSError *_Nullable *_Nullable)error {
    if (!fileURL || !fileURL.isFileURL) {
        if (error) *error = [NSError errorWithDomain:RootfsIntegrityErrorDomain code:RootfsIntegrityResultFileMissing userInfo:nil];
        return nil;
    }

    return [self computeSHA256Streaming:fileURL error:error];
}

+ (NSString *)computeSHA256Streaming:(NSURL *)fileURL error:(NSError **)error {
    NSInputStream *stream = [NSInputStream inputStreamWithURL:fileURL];
    [stream open];
    if (stream.streamError) {
        if (error) *error = stream.streamError;
        return nil;
    }

    CC_SHA256_CTX ctx;
    CC_SHA256_Init(&ctx);

    uint8_t buffer[kSHA256BufferSize];
    BOOL readAny = NO;

    while (stream.hasBytesAvailable) {
        NSInteger n = [stream read:buffer maxLength:sizeof(buffer)];
        if (n < 0) {
            [stream close];
            if (error) *error = stream.streamError ?: [NSError errorWithDomain:RootfsIntegrityErrorDomain code:RootfsIntegrityResultIOFailure userInfo:nil];
            return nil;
        }
        if (n == 0) break;
        readAny = YES;
        CC_SHA256_Update(&ctx, buffer, (CC_LONG)n);
    }
    [stream close];

    if (!readAny) {
        if (error) *error = [NSError errorWithDomain:RootfsIntegrityErrorDomain code:RootfsIntegrityResultIOFailure userInfo:nil];
        return nil;
    }

    unsigned char digest[CC_SHA256_DIGEST_LENGTH];
    CC_SHA256_Final(digest, &ctx);

    NSMutableString *out = [NSMutableString stringWithCapacity:CC_SHA256_DIGEST_LENGTH * 2];
    for (int i = 0; i < CC_SHA256_DIGEST_LENGTH; i++) {
        [out appendFormat:@"%02x", digest[i]];
    }
    return [out copy];
}

+ (NSString *)sha256OfData:(NSData *)data {
    unsigned char digest[CC_SHA256_DIGEST_LENGTH];
    CC_SHA256(data.bytes, (CC_LONG)data.length, digest);
    NSMutableString *out = [NSMutableString stringWithCapacity:CC_SHA256_DIGEST_LENGTH * 2];
    for (int i = 0; i < CC_SHA256_DIGEST_LENGTH; i++) {
        [out appendFormat:@"%02x", digest[i]];
    }
    return [out copy];
}

@end
