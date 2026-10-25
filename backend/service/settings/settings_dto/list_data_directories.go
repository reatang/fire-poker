package settings_dto

type ListDataDirectoriesInput struct {
}

// DataDirectoryItem describes one location Fire Poker reads or writes, shown read-only in data management.
type DataDirectoryItem struct {
	ID        string `json:"id"`         // stable key used by the frontend for labels
	Path      string `json:"path"`       // absolute path of the file or directory
	RevealDir string `json:"reveal_dir"` // directory opened by "show in file manager"
	IsFile    bool   `json:"is_file"`    // whether Path is a single file
	Exists    bool   `json:"exists"`     // whether Path currently exists on disk
}

type ListDataDirectoriesOutput struct {
	Items []DataDirectoryItem `json:"items"`
}
