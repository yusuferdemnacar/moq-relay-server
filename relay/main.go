package main

import (
	"flag"
	"fmt"
)

func main() {

	isClient := flag.Bool("client", false, "Run as client")

	playlistPath := flag.String("playlist", "", "Path to the playlist")
	updatePlaylistFiles := flag.Bool("update", false, "Update the playlist")

	flag.Parse()

	if *isClient {
		if *playlistPath == "" {
			fmt.Println("Usage for client: <program> --client --playlist <path to playlist>")
			return
		}
		runClient(*playlistPath, *updatePlaylistFiles)
	} else {
		runServer()
	}
}
