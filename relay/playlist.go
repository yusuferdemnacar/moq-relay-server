package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/grafov/m3u8"
)

type Channel struct {
	Name        string
	PlaylistURL string
	BaseURL     string
	MediaURLs   []string
	TvgID       string
	TvgLogo     string
	GroupTitle  string
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

				channelName := strings.ReplaceAll(matches[4], "/", "_")

				currentChannel := Channel{
					TvgID:      matches[1],
					TvgLogo:    matches[2],
					GroupTitle: matches[3],
					Name:       channelName,
				}

				for scanner.Scan() {
					nextLine := scanner.Text()
					if strings.HasPrefix(nextLine, "#EXTVLCOPT:") {
						continue
					}
					currentChannel.PlaylistURL = nextLine

					parsedURL, err := url.Parse(currentChannel.PlaylistURL)
					if err != nil {
						continue
					}
					parsedURL.Path = path.Dir(parsedURL.Path)
					currentChannel.BaseURL = parsedURL.String()
					channels[channelName] = currentChannel
					break
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
	parsedURL, err := url.Parse(channel.PlaylistURL)
	if err != nil {
		log.Printf("Failed to parse PlaylistURL %s: %v", channel.PlaylistURL, err)
		return err
	}
	filename := path.Base(parsedURL.Path)
	if filename == "/" {
		log.Printf("Failed to extract filename from PlaylistURL %s", channel.PlaylistURL)
		return fmt.Errorf("invalid filename extracted from PlaylistURL %s", channel.PlaylistURL)
	}

	tempDir := ""
	tempFile, err := os.CreateTemp(tempDir, filename+"*")
	if err != nil {
		log.Printf("Failed to create a temp file for %s: %v", filename, err)
		return err
	}
	defer os.Remove(tempFile.Name())

	log.Printf("Downloading %s to %s", channel.PlaylistURL, tempFile.Name())

	if err := downloadFile(channel.PlaylistURL, tempFile.Name()); err != nil {
		log.Printf("Failed to download file from %s: %v", channel.PlaylistURL, err)
		tempFile.Close()
		return err
	}
	tempFile.Close()
	channelDirPath := filepath.Join(dirPath, "channels", channel.Name)
	if err := os.MkdirAll(channelDirPath, os.ModePerm); err != nil {
		log.Printf("Failed to create directory %s: %v", channelDirPath, err)
		return err
	}

	finalPath := filepath.Join(channelDirPath, filename)

	if err := os.Rename(tempFile.Name(), finalPath); err != nil {
		log.Printf("Failed to move the downloaded file to %s: %v", finalPath, err)
		return err
	}

	log.Printf("Successfully downloaded and moved %s to %s", filename, finalPath)
	return nil
}

func saveChannelInfo(channel Channel, dirPath string) error {

	channelInfoPath := filepath.Join(dirPath, "channels", channel.Name, "channel.json")

	channelJSON, err := json.MarshalIndent(channel, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal channel info for %s: %v", channel.Name, err)
		return err
	}

	if err := os.MkdirAll(filepath.Dir(channelInfoPath), os.ModePerm); err != nil {
		log.Printf("Failed to create directory for %s: %v", channelInfoPath, err)
		return err
	}

	if err := os.WriteFile(channelInfoPath, channelJSON, 0644); err != nil {
		log.Printf("Failed to write to file %s: %v", channelInfoPath, err)
		return err
	}

	log.Printf("Successfully wrote channel info to %s", channelInfoPath)
	return nil
}

func downloadFile(url, filePath string) error {

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
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

func setMediaURLs(channel *Channel, playlistPath string) error {

	channelDir := filepath.Join(playlistPath, "channels", channel.Name)
	var playlistFilename string

	err := filepath.WalkDir(channelDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".m3u8") {
			playlistFilename = d.Name()
			return filepath.SkipDir
		}
		return nil
	})

	if err != nil {
		return err
	}

	if playlistFilename == "" {
		return nil
	}

	playlistPath = filepath.Join(channelDir, playlistFilename)
	file, err := os.Open(playlistPath)
	if err != nil {
		return err
	}
	defer file.Close()

	p, listType, err := m3u8.DecodeFrom(bufio.NewReader(file), true)
	if err != nil {
		return err
	}

	switch listType {
	case m3u8.MEDIA:
		channel.MediaURLs = append(channel.MediaURLs, channel.PlaylistURL)
	case m3u8.MASTER:
		masterPl := p.(*m3u8.MasterPlaylist)
		for _, variant := range masterPl.Variants {
			var mediaURL string
			if strings.HasPrefix(variant.URI, "http://") || strings.HasPrefix(variant.URI, "https://") {
				mediaURL = variant.URI
			} else {
				baseURL, err := url.Parse(channel.BaseURL)
				if err != nil {
					continue
				}
				relativeURL, err := baseURL.Parse(variant.URI)
				if err != nil {
					continue
				}
				mediaURL = relativeURL.String()
			}
			channel.MediaURLs = append(channel.MediaURLs, mediaURL)
		}
	}

	return nil
}

func getChannelNames(channels map[string]Channel) []string {
	channelNames := make([]string, 0, len(channels))
	for name := range channels {
		channelNames = append(channelNames, name)
	}
	return channelNames
}

func getAvailableChannelNames(playlistDirPath string) ([]string, error) {
	dirEntries, err := os.ReadDir(filepath.Join(playlistDirPath, "channels"))
	if err != nil {
		return nil, err
	}

	var channelNames []string
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			channelNames = append(channelNames, dirEntry.Name())
		}
	}

	return channelNames, nil
}

func filterAvailableChannels(channels map[string]Channel, availableChannelNames []string) map[string]Channel {
	availableChannels := make(map[string]Channel)
	for _, name := range availableChannelNames {
		if channel, ok := channels[name]; ok {
			availableChannels[name] = channel
		}
	}
	return availableChannels
}

func getAvailableChannelsFromFile(availableChannelNames []string, playlistDirPath string) map[string]Channel {

	availableChannels := make(map[string]Channel)
	for _, name := range availableChannelNames {
		channelPath := filepath.Join(playlistDirPath, "channels", name, "channel.json")
		file, err := os.Open(channelPath)
		if err != nil {
			log.Printf("Failed to open %s: %v", channelPath, err)
			continue
		}
		defer file.Close()

		var channel Channel
		if err := json.NewDecoder(file).Decode(&channel); err != nil {
			log.Printf("Failed to decode %s: %v", channelPath, err)
			continue
		}

		availableChannels[name] = channel

	}

	return availableChannels

}

func updatePlaylistFiles(playlistPath string) error {
	allChannels, err := parsePlaylist(playlistPath)
	if err != nil {
		fmt.Println("Error parsing playlist:", err)
		return err
	}

	playlistDirPath := filepath.Dir(playlistPath)

	for _, channel := range allChannels {
		downloadPlaylistFile(channel, playlistDirPath)
		fmt.Println()
	}

	availableChannelNames, _ := getAvailableChannelNames(playlistDirPath)
	availableChannels := filterAvailableChannels(allChannels, availableChannelNames)

	for _, channel := range availableChannels {
		setMediaURLs(&channel, playlistDirPath)
		saveChannelInfo(channel, playlistDirPath)
	}

	return nil

}

func randomChannel(channels map[string]Channel) Channel {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	channelNames := getChannelNames(channels)
	randomChannelName := channelNames[r.Intn(len(channelNames))]
	randomChannel := channels[randomChannelName]
	return randomChannel
}

func randomMediaURL(channel Channel) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomMediaURL := channel.MediaURLs[r.Intn(len(channel.MediaURLs))]
	return randomMediaURL
}
