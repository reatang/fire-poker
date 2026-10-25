package dir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// isolateTests keeps test binaries away from the real data directory unless a test opts back in.
var isolateTests = testing.Testing()

var (
	testDataDirOnce sync.Once
	testDataDirPath string
	testDataDirErr  error
)

// testDataDir returns one throwaway data directory shared by the current test process.
func testDataDir() (string, error) {
	testDataDirOnce.Do(func() {
		testDataDirPath, testDataDirErr = os.MkdirTemp("", "fire-poker-test-")
	})
	return testDataDirPath, testDataDirErr
}

// ConfigFileName defines the name of the configuration file.
const ConfigFileName = "config.json"

// DataBaseFileName defines the name of the database file.
const DataBaseFileName = "data.db"

type dataDirLocator struct {
	DataDir string `json:"data_dir"`
}

// GetDefaultBaseDir returns the stable home-based metadata directory used by Fire Poker.
func GetDefaultBaseDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dataPath := filepath.Join(homeDir, defaultBaseDirName)
	// Reuse an existing installation in place: config and plugin paths may be absolute.
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		for _, name := range legacyBaseDirNames {
			legacy := filepath.Join(homeDir, name)
			if info, legacyErr := os.Stat(legacy); legacyErr == nil && info.IsDir() {
				return legacy, nil
			}
		}
	}
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return "", err
	}
	return dataPath, nil
}

// GetLocatorFilePath returns the path of the custom data-directory locator file.
func GetLocatorFilePath() (string, error) {
	baseDir, err := GetDefaultBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "data_dir.json"), nil
}

// ReadLocatorDataDir reads the custom data-directory locator and returns an empty string when it does not exist.
func ReadLocatorDataDir() (string, error) {
	locatorPath, err := GetLocatorFilePath()
	if err != nil {
		return "", err
	}

	bytes, err := os.ReadFile(locatorPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var locator dataDirLocator
	if err := json.Unmarshal(bytes, &locator); err != nil {
		return "", err
	}

	return locator.DataDir, nil
}

// GetDataDir resolves the data directory: env override, then legacy locator file, then the home-based default.
func GetDataDir() (string, error) {
	configuredDir := os.Getenv("FIRE_POKER_DATA_DIR")
	if configuredDir == "" {
		configuredDir = os.Getenv("LEMONTEA_DATA_DIR") // Compatibility with existing launchers.
	}
	if configuredDir != "" {
		if err := os.MkdirAll(configuredDir, 0o755); err != nil {
			return "", err
		}
		return configuredDir, nil
	}
	if isolateTests {
		return testDataDir()
	}

	locatorDir, err := ReadLocatorDataDir()
	if err != nil {
		return "", err
	}
	if locatorDir != "" {
		if err := os.MkdirAll(locatorDir, 0o755); err != nil {
			return "", err
		}
		return locatorDir, nil
	}

	dataPath, err := GetDefaultBaseDir()
	if err != nil {
		return "", err
	}
	return dataPath, nil
}

// HomeDir get the home directory
func HomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return homeDir
}

// PluginsSubdirName is the unified parent directory that holds all extension subdirs under data_dir.
const PluginsSubdirName = "plugins"

// MCPSubdirName is the directory name for MCP bundles under the plugins root.
const MCPSubdirName = "mcp"

// PluginSubdirName is the directory name for Node.js plugins under the plugins root.
const PluginSubdirName = "plugin"

// CLISubdirName is the directory name for CLI plugin installs under the plugins root.
const CLISubdirName = "cli"

// CLIDataSubdirName is the directory name for CLI plugin persistent data under the plugins root.
const CLIDataSubdirName = "cli_data"

// ExtensionsRoot returns the unified extensions parent dir: {dataDir}/plugins.
func ExtensionsRoot(dataDir string) string {
	return filepath.Join(dataDir, PluginsSubdirName)
}

// MCPRoot returns the MCP bundles root: {dataDir}/plugins/mcp.
func MCPRoot(dataDir string) string {
	return filepath.Join(ExtensionsRoot(dataDir), MCPSubdirName)
}

// PluginRoot returns the Node.js plugins root: {dataDir}/plugins/plugin.
func PluginRoot(dataDir string) string {
	return filepath.Join(ExtensionsRoot(dataDir), PluginSubdirName)
}

// CLIRoot returns the CLI plugin install root: {dataDir}/plugins/cli.
func CLIRoot(dataDir string) string {
	return filepath.Join(ExtensionsRoot(dataDir), CLISubdirName)
}

// CLIDataRoot returns the CLI plugin persistent data root: {dataDir}/plugins/cli_data.
func CLIDataRoot(dataDir string) string {
	return filepath.Join(ExtensionsRoot(dataDir), CLIDataSubdirName)
}

// LegacyMCPRoot returns the pre-migration MCP root used by older builds.
func LegacyMCPRoot(dataDir string) string {
	return filepath.Join(dataDir, MCPSubdirName)
}

// LegacyPluginRoot returns the pre-migration Node.js plugin root used by older builds.
func LegacyPluginRoot(dataDir string) string {
	return filepath.Join(dataDir, PluginSubdirName)
}

// SkillsSubdirName is the directory name for skills under data_dir.
const SkillsSubdirName = "skills"

// SkillsRoot returns the skills root: {dataDir}/skills.
func SkillsRoot(dataDir string) string {
	return filepath.Join(dataDir, SkillsSubdirName)
}

// CacheSubdirName is the directory for re-creatable data such as the Node runtime and WebView storage.
const CacheSubdirName = "cache"

// LogsSubdirName is the directory for daily log files.
const LogsSubdirName = "logs"

// CacheRoot returns the cache root: {dataDir}/cache. Everything below it can be deleted safely.
func CacheRoot(dataDir string) string {
	return filepath.Join(dataDir, CacheSubdirName)
}

// LogsRoot returns the log directory: {dataDir}/logs.
func LogsRoot(dataDir string) string {
	return filepath.Join(dataDir, LogsSubdirName)
}

// NodeRuntimeRoot returns the bundled Node runtime directory: {dataDir}/cache/runtime/node.
func NodeRuntimeRoot(dataDir string) string {
	return filepath.Join(CacheRoot(dataDir), "runtime", "node")
}

// LegacyNodeRuntimeRoot returns the pre-cache Node runtime directory: {dataDir}/runtime/node.
func LegacyNodeRuntimeRoot(dataDir string) string {
	return filepath.Join(dataDir, "runtime", "node")
}

// WebViewDataRoot returns the Windows WebView2 user data directory: {dataDir}/cache/webview2.
func WebViewDataRoot(dataDir string) string {
	return filepath.Join(CacheRoot(dataDir), "webview2")
}

// LegacyWebViewDataRoot returns the pre-cache WebView2 user data directory: {dataDir}/webview2.
func LegacyWebViewDataRoot(dataDir string) string {
	return filepath.Join(dataDir, "webview2")
}

// TempRoot returns the temp directory for attachment copies: {os.TempDir()}/fire-poker. It is cleared on startup.
func TempRoot() string {
	return filepath.Join(os.TempDir(), "fire-poker")
}
