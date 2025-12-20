package fcm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type Client struct {
	client *messaging.Client
}

// NewClient initializes Firebase Cloud Messaging client
func NewClient() (*Client, error) {
	ctx := context.Background()

	// Check for Firebase credentials
	credPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if credPath == "" {
		// Try to read from environment variable JSON string
		credJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON")
		if credJSON == "" {
			log.Println("Warning: No Firebase credentials found. FCM disabled.")
			return &Client{client: nil}, nil
		}

		// Create temp file for credentials
		tmpFile, err := os.CreateTemp("", "firebase-credentials-*.json")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		defer tmpFile.Close()

		if _, err := tmpFile.Write([]byte(credJSON)); err != nil {
			return nil, fmt.Errorf("failed to write credentials: %w", err)
		}

		credPath = tmpFile.Name()
	}

	opt := option.WithCredentialsFile(credPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("error initializing firebase app: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting messaging client: %w", err)
	}

	log.Println("Firebase Cloud Messaging initialized successfully")
	return &Client{client: client}, nil
}

// SendNotification sends a push notification to a specific device token
func (c *Client) SendNotification(token, title, body string, data map[string]string) error {
	if c.client == nil {
		return fmt.Errorf("FCM client not initialized")
	}

	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "screener_alerts",
				Priority:  messaging.PriorityHigh,
			},
		},
	}

	ctx := context.Background()
	response, err := c.client.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	log.Printf("Successfully sent message: %s", response)
	return nil
}

// SendMulticast sends notification to multiple tokens
func (c *Client) SendMulticast(tokens []string, title, body string, data map[string]string) error {
	if c.client == nil {
		return fmt.Errorf("FCM client not initialized")
	}

	if len(tokens) == 0 {
		return nil
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "screener_alerts",
				Priority:  messaging.PriorityHigh,
			},
		},
	}

	ctx := context.Background()
	response, err := c.client.SendEachForMulticast(ctx, message)
	if err != nil {
		return fmt.Errorf("error sending multicast: %w", err)
	}

	log.Printf("Successfully sent %d messages (%d failures)", response.SuccessCount, response.FailureCount)
	return nil
}

// IsEnabled returns true if FCM client is initialized
func (c *Client) IsEnabled() bool {
	return c.client != nil
}

// DataToJSON converts data map to JSON string
func DataToJSON(data interface{}) string {
	b, _ := json.Marshal(data)
	return string(b)
}
