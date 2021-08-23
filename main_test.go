package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainNewGameData(t *testing.T) {
	name, code := "Death Strading", "death-strading"
	os.Args = []string{"name.exe", name}

	main()

	dbFilename := fmt.Sprintf("%s/%s/%s.json", outDir, code, code)
	file, err := os.Open(dbFilename)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.Remove(dbFilename))
	}()

	var data map[string]interface{}
	err = json.NewDecoder(file).Decode(&data)
	require.NoError(t, err)

	assert.Equal(t, name, data["name"])
	assert.EqualValues(t, 1, data["version"])
	assert.NotZero(t, data["createdAt"])
	assert.NotZero(t, data["deathsData"])
	assert.Len(t, data["deathsData"], 0)
}

func TestMainNewDeathData(t *testing.T) {
	game, gameCode := "Dark Souls 3", "dark-souls-3"
	name, code := "Abyss Watchers", "abyss-watchers"
	os.Args = []string{"name.exe", game, name}

	main()

	dbFilename := fmt.Sprintf("%s/%s/%s.json", outDir, gameCode, gameCode)
	file, err := os.Open(dbFilename)
	require.NoError(t, err)
	// defer func() {
	// 	assert.NoError(t, os.Remove(dbFilename))
	// }()

	var data map[string]interface{}
	err = json.NewDecoder(file).Decode(&data)
	require.NoError(t, err)

	assert.Equal(t, game, data["name"])
	assert.EqualValues(t, 1, data["version"])
	assert.NotZero(t, data["createdAt"])
	assert.NotZero(t, data["deathsData"])
	assert.Len(t, data["deathsData"], 1)
	deathsData, ok := data["deathsData"].(map[string]interface{})
	require.True(t, ok)
	deathData, ok := deathsData[code].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, name, deathData["name"])
	assert.EqualValues(t, 1, deathData["version"])
	assert.NotZero(t, deathData["createdAt"])
	assert.EqualValues(t, 1, deathData["deaths"])
}
