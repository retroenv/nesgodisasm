package jumpengine

import (
	"testing"

	"github.com/retroenv/nesgodisasm/internal/arch/m6502"
	"github.com/retroenv/nesgodisasm/internal/arch/mocks"
	"github.com/retroenv/nesgodisasm/internal/assembler/ca65"
	"github.com/retroenv/retrogolib/arch/system/nes/parameter"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/log"
)

// TestScanForNewJumpEngineEntry_MultipleTerminated verifies that multiple terminated
// jump engine callers are correctly removed from the processing queue.
func TestScanForNewJumpEngineEntry_MultipleTerminated(t *testing.T) {
	logger := log.NewTestLogger(t)
	ar := m6502.New(parameter.New(ca65.ParamConfig))
	mapper := mocks.NewMapper()
	dis := mocks.NewDisasm(logger, mapper, 0x8000, 0x10000)
	je := New(ar)

	// Create multiple terminated jump engine callers that should all be removed.
	caller1 := &jumpEngineCaller{
		tableStartAddress: 0x8010,
		entries:           2,
		terminated:        true,
	}
	caller2 := &jumpEngineCaller{
		tableStartAddress: 0x8020,
		entries:           3,
		terminated:        true,
	}
	caller3 := &jumpEngineCaller{
		tableStartAddress: 0x8030,
		entries:           1,
		terminated:        true,
	}
	je.jumpEngineCallers = []*jumpEngineCaller{caller1, caller2, caller3}

	found, err := je.ScanForNewJumpEngineEntry(dis)

	assert.NoError(t, err)
	assert.False(t, found, "should not find any new entries when all are terminated")
	assert.Len(t, je.jumpEngineCallers, 0, "all terminated entries should be removed")
}

// TestScanForNewJumpEngineEntry_MixedTerminated tests the scenario where
// some entries are terminated and others are not.
func TestScanForNewJumpEngineEntry_MixedTerminated(t *testing.T) {
	logger := log.NewTestLogger(t)
	ar := m6502.New(parameter.New(ca65.ParamConfig))
	mapper := mocks.NewMapper()
	dis := mocks.NewDisasm(logger, mapper, 0x8000, 0x10000)

	// Set up memory with a valid function reference
	dis.Memory[0x8030] = 0x00 // Low byte
	dis.Memory[0x8031] = 0x80 // High byte (points to 0x8000, within code range)

	je := New(ar)

	// Mix of terminated and active entries
	caller1 := &jumpEngineCaller{
		tableStartAddress: 0x8010,
		entries:           2,
		terminated:        true,
	}
	caller2 := &jumpEngineCaller{
		tableStartAddress: 0x8020,
		entries:           0,
		terminated:        false,
	}
	caller3 := &jumpEngineCaller{
		tableStartAddress: 0x8030,
		entries:           0,
		terminated:        false,
	}
	je.jumpEngineCallers = []*jumpEngineCaller{caller1, caller2, caller3}

	found, err := je.ScanForNewJumpEngineEntry(dis)

	assert.NoError(t, err)
	// The active entries should remain
	assert.Len(t, je.jumpEngineCallers, 1)
	assert.Equal(t, je.jumpEngineCallers[0], caller3)
	// At least one of the active entries should be processed
	assert.True(t, found)
	assert.Equal(t, 2, caller1.entries)
	assert.Equal(t, 0, caller2.entries)
	assert.Equal(t, 1, caller3.entries)
}
