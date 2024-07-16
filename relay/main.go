package main

import (
	"flag"
	"fmt"
)

func main() {

	isClient := flag.Bool("client", false, "Run as client")

	playlistPath := flag.String("playlist", "", "Path to the playlist")
	outputDirPath := flag.String("output-dir", "", "Path to the output directory")
	updatePlaylistFiles := flag.Bool("update", false, "Update the playlist")

	moqrsDir := flag.String("moqrs-dir", "", "Path to the moqrs directory")

	flag.Parse()

	if *isClient {
		if *playlistPath == "" {
			fmt.Println("Usage for client: <program> --client --playlist <path to playlist>")
			return
		}
		if *outputDirPath == "" {
			fmt.Println("Usage for client: <program> --client --output-dir <path to output directory>")
			return
		}
		runClient(*playlistPath, *updatePlaylistFiles, *outputDirPath)
	} else {
		if *moqrsDir == "" {
			fmt.Println("Usage for server: <program> --moqrs-dir <path to moq-rs directory>")
			return
		}
		runServer(*moqrsDir)
	}
}
