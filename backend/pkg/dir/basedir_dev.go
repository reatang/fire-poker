//go:build !production

package dir

// defaultBaseDirName keeps development builds away from the installed app's data.
var defaultBaseDirName = ".fire-poker-dev"

// legacyBaseDirNames is empty so development builds never adopt real legacy data.
var legacyBaseDirNames []string

// BundleIdentifier lets a development build run beside the installed app without sharing its identity.
const BundleIdentifier = "com.reatang.fire-poker.dev"
