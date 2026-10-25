//go:build production

package dir

// defaultBaseDirName is the home-relative data directory of release builds.
var defaultBaseDirName = ".fire-poker"

// legacyBaseDirNames lists pre-rebrand directories that release builds keep using in place.
var legacyBaseDirNames = []string{".lemontea"}

// BundleIdentifier is the release app identity, shared by the macOS bundle, WebKit storage and the single-instance lock.
const BundleIdentifier = "com.reatang.fire-poker"
