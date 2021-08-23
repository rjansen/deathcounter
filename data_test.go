package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeathData(t *testing.T) {
	data := NewDeathData("Abyss Watchers", 0)

	assert.NotZero(t, data.Version)
	assert.NotZero(t, data.Name)
	assert.Zero(t, data.Deaths)
	assert.NotZero(t, data.CreatedAt)
	assert.NotZero(t, data.String())
}

func TestGameData(t *testing.T) {
	data := NewGameData("Dark Souls 3")

	assert.NotZero(t, data.Version)
	assert.NotZero(t, data.Name)
	assert.NotZero(t, data.DeathsData)
	assert.Len(t, data.DeathsData, 0)
	assert.Zero(t, data.DeathsData["deathDataKey"])
	assert.NotZero(t, data.CreatedAt)
	assert.NotZero(t, data.String())
}

func TestDecodeData(t *testing.T) {
	fielda, fieldb, version := "fielda", "fieldb", 19
	reader := bytes.NewBufferString(
		fmt.Sprintf(
			`{"fielda": "%s", "fieldb": "%s", "version": %d}`,
			fielda, fieldb, version,
		),
	)

	var data map[string]interface{}
	err := FromJSON(reader, &data)
	require.NoError(t, err)

	assert.Len(t, data, 3)
	assert.Equal(t, fielda, data["fielda"])
	assert.Equal(t, fieldb, data["fieldb"])
	assert.EqualValues(t, version, data["version"])
}

func TestEncodeData(t *testing.T) {
	fielda, fieldb, version := "fielda", "fieldb", 19
	data := map[string]interface{}{
		"fielda":  fielda,
		"fieldb":  fieldb,
		"version": version,
	}

	var b bytes.Buffer
	err := ToJSON(&b, data)
	require.NoError(t, err)

	assert.JSONEq(
		t,
		fmt.Sprintf(
			`{"fielda": "%s", "fieldb": "%s", "version": %d}`,
			fielda, fieldb, version,
		),
		b.String(),
	)
}

func TestSlugify(t *testing.T) {
	text := "bla bla bla my !@#$jfladkl fucking text"
	result := Slugify(text)

	assert.Equal(t, "bla-bla-bla-my-jfladkl-fucking-text", result)
}
