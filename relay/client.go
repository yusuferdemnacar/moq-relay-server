package main

import (
	"fmt"
	"path/filepath"
)

func runClient(playlistPath string, updatePlaylist bool) error {

	if updatePlaylist {
		err := updatePlaylistFiles(playlistPath)
		if err != nil {
			return err
		}
	}

	playlistDirPath := filepath.Dir(playlistPath)

	availableChannelNames, _ := getAvailableChannelNames(playlistDirPath)
	availableChannels := getAvailableChannelsFromFile(availableChannelNames, playlistDirPath)

	randomChannel := randomChannel(availableChannels)

	fmt.Println("Random channel name:", randomChannel.Name)

	randomMediaURL := randomMediaURL(randomChannel)

	fmt.Println("Random media URL:", randomMediaURL)

	return nil

}
