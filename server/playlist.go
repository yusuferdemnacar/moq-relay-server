package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type Channel struct {
	Name       string
	URL        string
	TvgID      string
	TvgLogo    string
	GroupTitle string
}

func parsePlaylist(filename string) (map[string]Channel, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	channels := make(map[string]Channel)
	scanner := bufio.NewScanner(file)

	extinfRegex := regexp.MustCompile(`#EXTINF:-1 tvg-id="(.*)" tvg-logo="(.*)" group-title="(.*)",(.*)`)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "#EXTINF:") {
			matches := extinfRegex.FindStringSubmatch(line)
			if len(matches) == 5 {
				// Replace slashes with underscores in the channel name
				channelName := strings.ReplaceAll(matches[4], "/", "_")

				currentChannel := Channel{
					TvgID:      matches[1],
					TvgLogo:    matches[2],
					GroupTitle: matches[3],
					Name:       channelName,
				}
				// The next line (channel URL) is expected to be the URL, so we read it in advance.
				for scanner.Scan() {
					nextLine := scanner.Text()
					// Skip lines starting with "#EXTVLCOPT:"
					if strings.HasPrefix(nextLine, "#EXTVLCOPT:") {
						continue
					}
					currentChannel.URL = nextLine
					// Use modified channel name as the key for the map.
					channels[channelName] = currentChannel
					break // Found the URL, break the loop
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return channels, nil
}

func downloadPlaylistFile(channel Channel, dirPath string) error {
	parsedURL, err := url.Parse(channel.URL)
	if err != nil {
		log.Printf("Failed to parse URL %s: %v", channel.URL, err)
		return err
	}
	filename := path.Base(parsedURL.Path)
	if filename == "/" {
		log.Printf("Failed to extract filename from URL %s", channel.URL)
		return fmt.Errorf("invalid filename extracted from URL %s", channel.URL)
	}

	// Download the file to a temporary location
	tempDir := "" // Empty string means os.TempDir will be used
	tempFile, err := os.CreateTemp(tempDir, filename+"*")
	if err != nil {
		log.Printf("Failed to create a temp file for %s: %v", filename, err)
		return err
	}
	defer os.Remove(tempFile.Name()) // Ensure cleanup of the temp file

	// Attempt to download the file
	if err := downloadFile(channel.URL, tempFile.Name()); err != nil {
		log.Printf("Failed to download file from %s: %v", channel.URL, err)
		tempFile.Close()
		return err
	}
	tempFile.Close()

	// Define the directory path for the channel
	channelDirPath := filepath.Join(dirPath, "channels", channel.Name)
	// Ensure the directory exists only after successful download
	if err := os.MkdirAll(channelDirPath, os.ModePerm); err != nil {
		log.Printf("Failed to create directory %s: %v", channelDirPath, err)
		return err
	}

	// Define the final path for the file
	finalPath := filepath.Join(channelDirPath, filename)
	// Move the file from the temporary location to the final destination
	if err := os.Rename(tempFile.Name(), finalPath); err != nil {
		log.Printf("Failed to move the downloaded file to %s: %v", finalPath, err)
		return err
	}

	log.Printf("Successfully downloaded and moved %s to %s", filename, finalPath)
	return nil
}

func saveChannelInfo(channel Channel, dirPath string) error {
	// Define the path for the channel info file
	channelInfoPath := filepath.Join(dirPath, "channels", channel.Name, "info.json")

	// Marshal the channel info to JSON
	channelJSON, err := json.Marshal(channel)
	if err != nil {
		log.Printf("Failed to marshal channel info for %s: %v", channel.Name, err)
		return err
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(channelInfoPath), os.ModePerm); err != nil {
		log.Printf("Failed to create directory for %s: %v", channelInfoPath, err)
		return err
	}

	// Write the JSON to the file
	if err := os.WriteFile(channelInfoPath, channelJSON, 0644); err != nil {
		log.Printf("Failed to write to file %s: %v", channelInfoPath, err)
		return err
	}

	log.Printf("Successfully wrote channel info to %s", channelInfoPath)
	return nil
}

func downloadFile(url, filePath string) error {
	// Download logic here, writing to filePath
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
