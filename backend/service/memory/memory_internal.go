package memory

import (
	"time"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/configfile"
	"github.com/reatang/fire-poker/backend/pkg/dir"
	"github.com/reatang/fire-poker/backend/service/memory/memory_dto"
)

const memoryTimeFormat = time.RFC3339

func toMemoryItems(memories []data_models.Memory) []memory_dto.MemoryItem {
	items := make([]memory_dto.MemoryItem, 0, len(memories))
	for _, memory := range memories {
		items = append(items, toMemoryItem(memory))
	}
	return items
}

func toMemoryItem(memory data_models.Memory) memory_dto.MemoryItem {
	return memory_dto.MemoryItem{
		ID:             memory.ID,
		Summary:        memory.Summary,
		Content:        memory.Content,
		Type:           memory.Type,
		Target:         memory.Target,
		Source:         memory.Source,
		CharCount:      memory.CharCount,
		EmbeddingID:    memory.EmbeddingID,
		IsForgotten:    memory.IsForgotten,
		RecallCount:    memory.RecallCount,
		LastRecalledAt: formatOptionalTime(memory.LastRecalledAt),
		LastUsedAt:     formatOptionalTime(memory.LastUsedAt),
		Importance:     memory.Importance,
		Confidence:     memory.Confidence,
		Pinned:         memory.Pinned,
		Created:        memory.CreatedAt.Format(memoryTimeFormat),
		Updated:        memory.UpdatedAt.Format(memoryTimeFormat),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(memoryTimeFormat)
	return &formatted
}

// loadConfig reads the shared config; configfile.Load applies the memory-enabled default.
func (m *Memory) loadConfig() (*data_models.Config, error) {
	return configfile.LoadCurrent()
}

func (m *Memory) saveConfig(config *data_models.Config) error {
	dataDir := config.DataDir
	if dataDir == "" {
		var err error
		dataDir, err = dir.GetDataDir()
		if err != nil {
			return err
		}
		config.DataDir = dataDir
	}
	return configfile.SaveFields(dataDir, config, "memory")
}
