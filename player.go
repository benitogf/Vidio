package vidio

import (
	"errors"
)

type Player struct {
	FilePath string
	ID       string
	Video    *Video
}

var Players []Player

func findPlayer(filePath string, id string) (*Player, error) {
	for index, entry := range Players {
		if entry.FilePath == filePath && entry.ID == id {
			return &Players[index], nil
		}
	}

	return &Player{}, errors.New("failed to find player instance")
}

func GetPlayer(filePath string, id string) (*Player, error) {
	player, err := findPlayer(filePath, id)
	if err == nil {
		player.Video.Reset()
		return player, nil
	}

	newVideo, err := NewVideo(filePath)
	if err != nil {
		return player, err
	}
	newPlayer := Player{
		FilePath: filePath,
		ID:       id,
		Video:    newVideo,
	}

	Players = append(Players, newPlayer)

	return &newPlayer, nil
}
