package benchmark

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Benchmark(b *testing.B) {
	instancesJSON, err := GetInstances(instanceListGetterJSON{})
	require.NoError(b, err)

	instancesAll, err := GetInstances(instanceListGetterAllRows{})
	require.NoError(b, err)

	require.Len(b, instancesJSON, nInstancesPerProject)
	require.Len(b, instancesAll, nInstancesPerProject)
	for i := range instancesJSON {
		instAll := instancesAll[i]
		instJSON := instancesJSON[i]
		for k, v := range instAll.ExpandedConfig {
			require.Equal(b, instJSON.ExpandedConfig[k], v)
		}
	}

	correctInstanceIDs := make([]int, 0, len(instancesAll))
	for _, inst := range instancesAll {
		if inst.ExpandedConfig[lookupConfig[0]] == lookupConfig[1] {
			correctInstanceIDs = append(correctInstanceIDs, inst.ID)
		}
	}

	var queryResult []int
	b.Run("query", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			queryResult = benchmarkQuery(b)
		}
	})
	require.ElementsMatch(b, correctInstanceIDs, queryResult)

	var manualAllRowsResult []int
	b.Run("manual_all_rows", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			manualAllRowsResult = benchmarkManualAllRows(b)
		}
	})
	require.ElementsMatch(b, correctInstanceIDs, manualAllRowsResult)

	var manualJSONResult []int
	b.Run("manual_json", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			manualJSONResult = benchmarkManualJSON(b)
		}
	})
	require.ElementsMatch(b, correctInstanceIDs, manualJSONResult)

	var scriptletAllRowsResult []int
	b.Run("scriptlet_all_rows", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			scriptletAllRowsResult = benchmarkScriptletAllRows(b)
		}
	})
	require.ElementsMatch(b, correctInstanceIDs, scriptletAllRowsResult)

	var scriptletJSONResult []int
	b.Run("scriptlet_json", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			scriptletJSONResult = benchmarkScriptletJSON(b)
		}
	})
	require.ElementsMatch(b, correctInstanceIDs, scriptletJSONResult)
}

func benchmarkQuery(b *testing.B) []int {
	res, err := RunQuery()
	require.NoError(b, err)
	return res
}

func benchmarkManualAllRows(b *testing.B) []int {
	res, err := RunManual(instanceListGetterAllRows{})
	require.NoError(b, err)
	return res
}

func benchmarkManualJSON(b *testing.B) []int {
	res, err := RunManual(instanceListGetterJSON{})
	require.NoError(b, err)
	return res
}

func benchmarkScriptletAllRows(b *testing.B) []int {
	res, err := RunScriptlet(instanceListGetterAllRows{})
	require.NoError(b, err)
	return res
}

func benchmarkScriptletJSON(b *testing.B) []int {
	res, err := RunScriptlet(instanceListGetterJSON{})
	require.NoError(b, err)
	return res
}
