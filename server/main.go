package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: <program> <path to playlist>")
		return
	}

	playlistPath := os.Args[1]

	channels, err := parsePlaylist(playlistPath)
	if err != nil {
		fmt.Println("Error parsing playlist:", err)
		return
	}

	playlistDirPath := filepath.Dir(playlistPath)

	for _, channel := range channels {
		downloadPlaylistFile(channel, playlistDirPath)
		saveChannelInfo(channel, playlistDirPath)
	}
}
