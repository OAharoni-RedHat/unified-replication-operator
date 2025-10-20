/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package translation

import (
	"testing"
)

// BenchmarkStateTranslation benchmarks state translation operations
func BenchmarkStateTranslation(b *testing.B) {
	engine := NewEngine()

	b.Run("ToBackend", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.TranslateStateToBackend(BackendCeph, "source")
		}
	})

	b.Run("FromBackend", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.TranslateStateFromBackend(BackendCeph, "primary")
		}
	})

	b.Run("RoundTrip", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			backend, _ := engine.TranslateStateToBackend(BackendCeph, "source")
			_, _ = engine.TranslateStateFromBackend(BackendCeph, backend)
		}
	})
}

// BenchmarkModeTranslation benchmarks mode translation operations
func BenchmarkModeTranslation(b *testing.B) {
	engine := NewEngine()

	b.Run("ToBackend", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.TranslateModeToBackend(BackendTrident, "asynchronous")
		}
	})

	b.Run("FromBackend", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.TranslateModeFromBackend(BackendTrident, "Async")
		}
	})

	b.Run("RoundTrip", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			backend, _ := engine.TranslateModeToBackend(BackendTrident, "asynchronous")
			_, _ = engine.TranslateModeFromBackend(BackendTrident, backend)
		}
	})
}

// BenchmarkBackendSpecificFunctions benchmarks the convenience functions
func BenchmarkBackendSpecificFunctions(b *testing.B) {
	engine := NewEngine()

	b.Run("CephTranslation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = engine.TranslateUnifiedToCeph("source", "asynchronous")
		}
	})

	b.Run("TridentTranslation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = engine.TranslateUnifiedToTrident("replica", "synchronous")
		}
	})

	b.Run("PowerStoreTranslation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
		}
	})

	b.Run("CephReverse", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = engine.TranslateCephToUnified("primary", "async")
		}
	})

	b.Run("TridentReverse", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = engine.TranslateTridentToUnified("established", "Sync")
		}
	})

	b.Run("PowerStoreReverse", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = engine.TranslatePowerStoreToUnified("destination", "ASYNC")
		}
	})
}

// BenchmarkMapOperations benchmarks the underlying map operations
func BenchmarkMapOperations(b *testing.B) {
	stateMap := CephStateMap

	b.Run("MapToBackend", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = stateMap.ToBackend("source")
		}
	})

	b.Run("MapFromBackend", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = stateMap.FromBackend("primary")
		}
	})
}

// BenchmarkAllBackends benchmarks translation across all backends
func BenchmarkAllBackends(b *testing.B) {
	engine := NewEngine()
	backends := GetSupportedBackends()

	b.Run("StateTranslationAllBackends", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			backend := backends[i%len(backends)]
			_, _ = engine.TranslateStateToBackend(backend, "source")
		}
	})

	b.Run("ModeTranslationAllBackends", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			backend := backends[i%len(backends)]
			_, _ = engine.TranslateModeToBackend(backend, "asynchronous")
		}
	})
}

// BenchmarkValidation benchmarks validation operations
func BenchmarkValidation(b *testing.B) {
	engine := NewEngine()
	validator := NewValidator()

	b.Run("SingleBackendValidation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = engine.ValidateTranslation(BackendCeph)
		}
	})

	b.Run("AllMappingsValidation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = validator.ValidateAllMappings()
		}
	})

	b.Run("RoundTripValidation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = validator.ValidateTranslationRoundTrip(BackendCeph, "source", "asynchronous")
		}
	})
}

// BenchmarkErrorCases benchmarks error handling performance
func BenchmarkErrorCases(b *testing.B) {
	engine := NewEngine()

	b.Run("InvalidState", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.TranslateStateToBackend(BackendCeph, "invalid")
		}
	})

	b.Run("InvalidBackend", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.TranslateStateToBackend("invalid", "source")
		}
	})
}

// BenchmarkHighThroughput simulates high-throughput translation scenarios
func BenchmarkHighThroughput(b *testing.B) {
	engine := NewEngine()

	// Test data for realistic scenarios
	testCases := []struct {
		backend Backend
		state   string
		mode    string
	}{
		{BackendCeph, "source", "asynchronous"},
		{BackendCeph, "replica", "synchronous"},
		{BackendTrident, "promoting", "asynchronous"},
		{BackendTrident, "demoting", "synchronous"},
		{BackendPowerStore, "failed", "asynchronous"},
	}

	b.Run("ConcurrentTranslations", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				tc := testCases[i%len(testCases)]
				_, _ = engine.TranslateStateToBackend(tc.backend, tc.state)
				_, _ = engine.TranslateModeToBackend(tc.backend, tc.mode)
				i++
			}
		})
	})

	b.Run("SequentialBatch", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, tc := range testCases {
				_, _ = engine.TranslateStateToBackend(tc.backend, tc.state)
				_, _ = engine.TranslateModeToBackend(tc.backend, tc.mode)
			}
		}
	})
}

// BenchmarkDefaultEngine benchmarks the default engine instance
func BenchmarkDefaultEngine(b *testing.B) {
	b.Run("DefaultEngineStateTranslation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = DefaultEngine.TranslateStateToBackend(BackendCeph, "source")
		}
	})

	b.Run("DefaultEngineValidation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = DefaultEngine.ValidateAllTranslations()
		}
	})
}

// BenchmarkMemoryAllocation measures memory allocation during translation
func BenchmarkMemoryAllocation(b *testing.B) {
	engine := NewEngine()

	b.Run("NoAllocation", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.TranslateStateToBackend(BackendCeph, "source")
		}
	})

	b.Run("BackendInfo", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.GetBackendInfo(BackendCeph)
		}
	})
}
