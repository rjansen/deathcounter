package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
)

var (
	outDir    = ".deathcounter"
	createDir = func(name string) error {
		return os.MkdirAll(name, os.ModePerm)
	}
)

func raiseError(exitCode int, errMsg string, args ...interface{}) {
	fmt.Printf(errMsg+"\n", args...)
	os.Exit(exitCode)
}

func main() {
	game := os.Args[1]
	code := Slugify(game)
	var gameData GameData

	outDir := fmt.Sprintf("%s/%s", outDir, code)
	filename := fmt.Sprintf("%s/%s.json", outDir, code)
	file, err := os.Open(filename)
	newGame := os.IsNotExist(err)
	if err != nil {
		if !os.IsNotExist(err) {
			raiseError(4, "err_opendb: filename='%s', cause='%s'", filename, err)
		}
		gameData = NewGameData(game)
	} else {
		if err := FromJSON(file, &gameData); err != nil {
			raiseError(5, "err_decodedb: filename='%s', cause='%s'", filename, err)
		}
	}

	var deathData DeathData
	if len(os.Args) > 2 {
		death := os.Args[2]
		deathCode := Slugify(death)
		if _deathData, exists := gameData.DeathsData[deathCode]; !exists {
			deathData = NewDeathData(death, 1)
		} else {
			deathData = _deathData
			deathData.Deaths++
			deathData.Version++
		}
		gameData.DeathsData[deathCode] = deathData
		if !newGame {
			gameData.Version++
		}
	}

	var buffer bytes.Buffer
	if err := ToJSON(&buffer, gameData); err != nil {
		raiseError(5, "err_encodedb: filename='%s', cause='%s'", filename, err)
	}

	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		if err := createDir(outDir); err != nil {
			raiseError(4, "err_writedb: filename='%s', cause='%s'", filename, err)
		}
	}

	if err := ioutil.WriteFile(filename, buffer.Bytes(), os.ModePerm); err != nil {
		raiseError(4, "err_updatedb: filename='%s', cause='%s'", filename, err)
	}

	fmt.Printf("%s (version/deaths: %d): %s\n", gameData.Name, gameData.Version, deathData)
}
