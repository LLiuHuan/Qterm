package update

import "changeme/internal/errno"

const (
	keyCheckFailed      = "update.checkFailed"
	keyRateLimited      = "update.rateLimited"
	keyNoRelease        = "update.noRelease"
	keyNoAsset          = "update.noAsset"
	keyDownloadFailed   = "update.downloadFailed"
	keyChecksumMissing  = "update.checksumMissing"
	keyChecksumMismatch = "update.checksumMismatch"
	keyInstallFailed    = "update.installFailed"
	keyUnsupported      = "update.unsupported"
	keyCancelled        = "update.cancelled"
)

func checkFailed(detail string) error {
	return errno.New(errno.CodeConnFailed, keyCheckFailed, detail)
}

func rateLimited(detail string) error {

	return errno.New(errno.CodePermissionDenied, keyRateLimited, detail)
}

func noRelease() error {
	return errno.New(errno.CodeNotFound, keyNoRelease, "")
}

func noAsset(detail string) error {
	return errno.New(errno.CodeNotFound, keyNoAsset, detail)
}

func downloadFailed(detail string) error {
	return errno.New(errno.CodeConnFailed, keyDownloadFailed, detail)
}

func checksumMissing() error {
	return errno.New(errno.CodeStoreFailed, keyChecksumMissing, "")
}

func checksumMismatch() error {
	return errno.New(errno.CodePermissionDenied, keyChecksumMismatch, "")
}

func installFailed(detail string) error {
	return errno.New(errno.CodeStoreFailed, keyInstallFailed, detail)
}

func unsupportedPlatform(detail string) error {
	return errno.New(errno.CodeInvalidArg, keyUnsupported, detail)
}
