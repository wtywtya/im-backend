// Run a small, reproducible local WebSocket load test against the IM service.
// Example: go run ./cmd/loadtest -senders 20 -per-sender 10
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"im-project/global"
	"im-project/models"
)

type registerResult struct {
	UID uint `json:"uid"`
}

type loginResult struct {
	Token string `json:"token"`
}

type account struct {
	UID   uint
	Token string
}

func main() {
	receiverBase := flag.String("receiver", "http://127.0.0.1:8080", "HTTP address for the receiving client")
	senderBase := flag.String("sender", "http://127.0.0.1:8080", "HTTP address for sending clients")
	senders := flag.Int("senders", 20, "number of sending clients")
	perSender := flag.Int("per-sender", 10, "messages sent by each client")
	timeout := flag.Duration("timeout", 60*time.Second, "time to wait for all messages")
	flag.Parse()

	if *senders < 1 || *senders > 500 || *perSender < 1 || *perSender > 1000 || *senders**perSender > 10000 {
		panic("senders must be 1..500, per-sender 1..1000, and total messages <= 10000")
	}
	if *timeout <= 0 {
		panic("timeout must be positive")
	}

	total := *senders * *perSender
	runID := fmt.Sprintf("lt_%d", time.Now().UnixNano())
	password := runID + "_test_only"
	client := &http.Client{Timeout: 10 * time.Second}

	fmt.Printf("Preparing %d test accounts; setup time is excluded from measurements.\n", *senders+1)
	receiverAccount, err := createAccount(client, *receiverBase, runID+"_receiver", password)
	if err != nil {
		panic(err)
	}
	receiverConn, err := dial(*receiverBase, receiverAccount.Token)
	if err != nil {
		panic(err)
	}
	defer receiverConn.Close()

	connections := make([]*websocket.Conn, 0, *senders)
	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()
	for i := 0; i < *senders; i++ {
		user, err := createAccount(client, *senderBase, fmt.Sprintf("%s_sender_%d", runID, i), password)
		if err != nil {
			panic(err)
		}
		conn, err := dial(*senderBase, user.Token)
		if err != nil {
			panic(err)
		}
		connections = append(connections, conn)
	}

	received := make(chan models.Message, total)
	readError := make(chan error, 1)
	go func() {
		for {
			var msg models.Message
			if err := receiverConn.ReadJSON(&msg); err != nil {
				readError <- err
				return
			}
			if strings.HasPrefix(msg.Text, runID+"_message_") {
				received <- msg
			}
		}
	}()

	var sentAt sync.Map
	sendErrors := make(chan error, *senders)
	startSignal := make(chan struct{})
	var writers sync.WaitGroup
	for i, conn := range connections {
		writers.Add(1)
		go func(senderIndex int, conn *websocket.Conn) {
			defer writers.Done()
			<-startSignal
			for j := 0; j < *perSender; j++ {
				text := fmt.Sprintf("%s_message_%d_%d", runID, senderIndex, j)
				sentAt.Store(text, time.Now())
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteJSON(models.Message{To: receiverAccount.UID, Text: text}); err != nil {
					sendErrors <- fmt.Errorf("sender %d, message %d: %w", senderIndex, j, err)
					return
				}
			}
		}(i, conn)
	}

	started := time.Now()
	close(startSignal)
	seen := make(map[string]bool, total)
	latencies := make([]time.Duration, 0, total)
	duplicates := 0
	deadline := time.NewTimer(*timeout)
	defer deadline.Stop()
	collecting := true
	for collecting {
		select {
		case msg := <-received:
			if seen[msg.Text] {
				duplicates++
				continue
			}
			seen[msg.Text] = true
			if value, ok := sentAt.Load(msg.Text); ok {
				latencies = append(latencies, time.Since(value.(time.Time)))
			}
			if len(seen) == total {
				collecting = false
			}
		case err := <-readError:
			fmt.Printf("Receiver read error: %v\n", err)
			collecting = false
		case <-deadline.C:
			fmt.Println("Timed out waiting for delivery.")
			collecting = false
		}
	}
	duration := time.Since(started)
	writers.Wait()
	close(sendErrors)
	writeErrorCount := 0
	for err := range sendErrors {
		writeErrorCount++
		fmt.Printf("Write error: %v\n", err)
	}

	global.InitMySQL()
	var dbCount int64
	if err := global.DB.Model(&models.Message{}).Where("content LIKE ?", runID+"_message_%").Count(&dbCount).Error; err != nil {
		panic(err)
	}
	var uniqueDBCount int64
	if err := global.DB.Model(&models.Message{}).Distinct("content").Where("content LIKE ?", runID+"_message_%").Count(&uniqueDBCount).Error; err != nil {
		panic(err)
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	fmt.Printf("\nRun ID: %s\n", runID)
	fmt.Printf("Sender clients: %d; receiver clients: 1; messages/client: %d\n", *senders, *perSender)
	fmt.Printf("Expected: %d; delivered: %d; duplicate deliveries: %d; write errors: %d\n", total, len(seen), duplicates, writeErrorCount)
	fmt.Printf("MySQL rows: %d; distinct message texts: %d\n", dbCount, uniqueDBCount)
	fmt.Printf("Measured duration: %s; delivered throughput: %.1f messages/s\n", duration.Round(time.Millisecond), float64(len(seen))/duration.Seconds())
	if len(latencies) > 0 {
		fmt.Printf("Client-observed end-to-end latency: p50=%s, p95=%s, max=%s\n", percentile(latencies, .50), percentile(latencies, .95), latencies[len(latencies)-1])
	}
	if len(seen) != total || dbCount != int64(total) || uniqueDBCount != int64(total) || duplicates != 0 || writeErrorCount != 0 {
		panic("load test verification failed")
	}
}

func createAccount(client *http.Client, base, username, password string) (account, error) {
	request := map[string]string{"username": username, "password": password}
	var registered registerResult
	if err := postJSON(client, base+"/register", request, &registered); err != nil {
		return account{}, fmt.Errorf("register %s: %w", username, err)
	}
	var loggedIn loginResult
	if err := postJSON(client, base+"/login", request, &loggedIn); err != nil {
		return account{}, fmt.Errorf("login %s: %w", username, err)
	}
	if registered.UID == 0 || loggedIn.Token == "" {
		return account{}, errors.New("register/login returned empty UID or token")
	}
	return account{UID: registered.UID, Token: loggedIn.Token}, nil
}

func postJSON(client *http.Client, endpoint string, request, result any) error {
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	response, err := client.Post(endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 300))
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, body)
	}
	return json.NewDecoder(response.Body).Decode(result)
}

func dial(base, token string) (*websocket.Conn, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = "/ws"
	query := u.Query()
	query.Set("token", token)
	u.RawQuery = query.Encode()
	conn, _, err := (&websocket.Dialer{HandshakeTimeout: 10 * time.Second}).Dial(u.String(), nil)
	return conn, err
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	return sorted[index]
}
