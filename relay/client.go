package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	clientSpawnedProcesses = make(map[string]*os.Process)
)

func runClient(playlistPath string, updatePlaylist bool, outputDirPath string) error {

	if err := os.MkdirAll(outputDirPath, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

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

	err := sendMediaURLOverQUIC(randomMediaURL, outputDirPath)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func sendMediaURLOverQUIC(url string, outputDirPath string) error {
	quicConfig := &quic.Config{
		KeepAlivePeriod: 1 * time.Second,
	}
	conn, err := quic.DialAddr(context.Background(), "localhost:4242", &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"moq-media-url-send"}}, quicConfig)
	if err != nil {
		return err
	}

	defer conn.CloseWithError(0, "Client closed")

	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}

	defer stream.Close()

	_, err = stream.Write([]byte(url))
	if err != nil {
		return err
	}

	buffer := make([]byte, 4096)
	var name string

	fmt.Println("Waiting for response...")

	n, err := stream.Read(buffer)
	if err != nil {
		log.Fatal(err)
	}
	name = string(buffer[:n])
	fmt.Printf("Received: %s\n", name)

	stream.Close()

	fmt.Println("Subscribing to channel...")

	host := "localhost"
	port := 4443
	protocol := "https"

	time.Sleep(3 * time.Second)

	outputPath := filepath.Join(outputDirPath, name+".mp4")

	subCmd := fmt.Sprintf("~/moq-rs/target/release/moq-sub --name %s %s | ffmpeg -i - -t 10 %s", name, protocol+"://"+host+":"+fmt.Sprint(port), outputPath)
	cmd := exec.Command("bash", "-c", subCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Println("Error executing command:", err)
		return err
	}

	fmt.Println("Subscribed successfully")

	clientSpawnedProcesses[name] = cmd.Process

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalCh
		fmt.Println("Terminating client...")

		// Kill all spawned processes
		for name, proc := range clientSpawnedProcesses {
			if proc != nil {
				fmt.Printf("Killing process %s (PID: %d)\n", name, proc.Pid)
				if err := proc.Kill(); err != nil {
					log.Printf("Error killing process %s: %v\n", name, err)
				}
			}
		}

		stream.Close()

		conn.CloseWithError(0, "Client terminated")

		os.Exit(0)
	}()

	for true {
		time.Sleep(1 * time.Second)
	}

	return nil
}
