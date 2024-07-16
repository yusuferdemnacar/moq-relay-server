package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	serverSpawnedProcesses = make(map[string]*os.Process)
	pub_count              = 0
)

type connectionResult struct {
	url  string
	name string
}

func runServer(moqrsDir string) {
	tlsConfig, err := generateTLSConfig()
	if err != nil {
		log.Fatal(err)
	}

	listener, err := quic.ListenAddr("localhost:4242", tlsConfig, nil)
	if err != nil {
		log.Fatal(err)
	}

	relayCmdPath := path.Join(moqrsDir, "dev", "relay")

	cmd := exec.Command(relayCmdPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start command: %s, error: %v", relayCmdPath, err)
	}
	fmt.Println("Relay command started successfully")

	serverSpawnedProcesses["relay"] = cmd.Process

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	go func() {
		<-signalCh
		fmt.Println("Terminating server...")

		if err := listener.Close(); err != nil {
			log.Println("Error closing listener:", err)
		}

		// Kill all spawned processes
		for name, proc := range serverSpawnedProcesses {
			if proc != nil {
				fmt.Printf("Killing process %s (PID: %d)\n", name, proc.Pid)
				if err := proc.Kill(); err != nil {
					log.Printf("Error killing process %s: %v\n", name, err)
				}
			}
		}

		os.Exit(0) // Exit the server
	}()

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			return
		}
		go handleConnection(conn, moqrsDir)
	}
}

func handleConnection(conn quic.Connection, moqrsDir string) {

	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			log.Println("Error accepting stream:", err)
			return
		}
		connRes := make(chan connectionResult)
		go handleStream(stream, moqrsDir, connRes)
		result := <-connRes
		go publish(result)

	}
}

func handleStream(stream quic.Stream, moqrsDir string, connRes chan connectionResult) {
	temp := connectionResult{}

	buffer := make([]byte, 4096)
	n, err := stream.Read(buffer)
	if err != nil {
		log.Println(err)
	}
	temp.url = string(buffer[:n])
	fmt.Printf("Received: %s\n", temp.url)

	name := "pub" + fmt.Sprint(pub_count)
	temp.name = name

	_, err_w := stream.Write([]byte(name))
	if err != nil {
		log.Println(err_w)
	}

	fmt.Println("sent name to client")

	pub_count += 1

	connRes <- temp

}

func publish(result connectionResult) {
	host := "localhost"
	port := 4443
	protocol := "https"
	name := result.name

	fmt.Println("Publishing to relay...")

	pubCmd := fmt.Sprintf("ffmpeg -hide_banner -v quiet -stream_loop -1 -re -i %s -f mp4 -c copy -an -movflags cmaf+separate_moof+delay_moov+skip_trailer+frag_every_frame - | ~/moq-rs/target/release/moq-pub --name %s %s",
		result.url, name, protocol+"://"+host+":"+fmt.Sprint(port))

	cmd := exec.Command("bash", "-c", pubCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Println("Error executing command:", err)
		return
	}

	serverSpawnedProcesses["pub"+fmt.Sprint(pub_count)] = cmd.Process

	fmt.Println("Published successfully")
}

func generateTLSConfig() (*tls.Config, error) {
	cert, err := generateCertificate()
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"moq-media-url-send"},
	}, nil
}

func generateCertificate() (tls.Certificate, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"MOQ"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}

	return cert, nil
}
