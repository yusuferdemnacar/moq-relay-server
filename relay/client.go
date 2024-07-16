package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"path/filepath"

	"github.com/quic-go/quic-go"
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

	err := sendMediaURLOverQUIC(randomMediaURL)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func sendMediaURLOverQUIC(url string) error {
	conn, err := quic.DialAddr(context.Background(), "localhost:4242", &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"moq-media-url-send"}}, nil)
	if err != nil {
		return err
	}
	defer conn.CloseWithError(0, "done")

	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}
	defer stream.Close()

	_, err = stream.Write([]byte(url))
	if err != nil {
		return err
	}

	buffer := make([]byte, 1024)
	n, err := stream.Read(buffer)
	if err != nil {
		return err
	}
	fmt.Printf("Received: %s\n", string(buffer[:n]))

	return nil
}
