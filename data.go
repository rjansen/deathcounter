package main

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

type DeathData struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	Deaths    int       `json:"deaths"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewDeathData(name string, deaths int) DeathData {
	return DeathData{Name: name, Deaths: deaths, Version: 1, CreatedAt: time.Now()}
}

func (d DeathData) String() string {
	var b strings.Builder
	fmt.Fprint(&b, "DeathData")
	if d != (DeathData{}) {
		fmt.Fprintf(&b, "{Name='%s', Deaths=%d, Version=%d, CreatedAt=%s}", d.Name, d.Deaths, d.Version, d.CreatedAt.Format(time.RFC3339))
	} else {
		fmt.Fprint(&b, "{}")
	}

	return b.String()
}

type GameData struct {
	Version    int                  `json:"version"`
	Name       string               `json:"name"`
	DeathsData map[string]DeathData `json:"deathsData"`
	CreatedAt  time.Time            `json:"createdAt"`
}

func NewGameData(name string, deathsData ...DeathData) GameData {
	deathsDataMap := map[string]DeathData{}
	for _, deathData := range deathsData {
		deathCode := Slugify(deathData.Name)
		deathsDataMap[deathCode] = deathData
	}
	return GameData{Name: name, DeathsData: deathsDataMap, Version: 1, CreatedAt: time.Now()}
}

func (d GameData) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "GameData{Name='%s', DeathsData='%d items', Version=%d, CreatedAt=%s}", d.Name, len(d.DeathsData), d.Version, d.CreatedAt.Format(time.RFC3339))

	return b.String()
}

func FromJSON(reader io.Reader, data interface{}) error {
	return json.NewDecoder(reader).Decode(&data)
}

func ToJSON(writer io.Writer, data interface{}) error {
	return json.NewEncoder(writer).Encode(data)
}

var slugifyRe = regexp.MustCompile("[^a-z0-9]+")

func Slugify(text string) string {
	return strings.Trim(slugifyRe.ReplaceAllString(strings.ToLower(text), "-"), "-")
}
