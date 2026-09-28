package boot

import (
	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/migration"
	"kwu/internal/userctx"
)

func migrateLegacyConfigForActor(actor userctx.ActorContext, root string) (*config.MigrationResult, error) {
	if actor.ProfileID != userctx.DefaultProfileID {
		return nil, nil
	}
	return config.MigrateLegacyIfNeededForRoot(root)
}

func emitMigrationNotices(sink event.Sink, migrated *config.MigrationResult, migErr error) {
	if migErr != nil {
		sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: "legacy user-data migration failed: " + migErr.Error()})
	} else if migrated != nil {
		sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: migrated.Notice()})
	}
	migration.MigrateLegacyMemorySources(sink)
	migration.MigrateLegacySessionSources(sink)
}

func emitMemoryCompilerRetirementNotice(sink event.Sink, migrated bool, err error) {
	if err != nil {
		text := "Deprecated [agent].memory_compiler is ignored and was not rewritten; Memory v5 no longer runs or writes state: "
		if migrated {
			text = "Deprecated [agent].memory_compiler was removed from writable configs and is ignored in configs that could not be rewritten; Memory v5 no longer runs or writes state: "
		}
		sink.Emit(event.Event{
			Kind:  event.Notice,
			Level: event.LevelWarn,
			Text:  text + err.Error(),
		})
		return
	}
	if migrated {
		sink.Emit(event.Event{
			Kind:  event.Notice,
			Level: event.LevelInfo,
			Text:  "Deprecated [agent].memory_compiler was removed; Memory v5 no longer runs or writes state, while old transcripts remain readable.",
		})
	}
}
