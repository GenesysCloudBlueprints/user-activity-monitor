package main

import (
	"encoding/json"
	"fmt"
	"user-activity-monitor/src/apitypes"
	"user-activity-monitor/src/db"
)

func processPresenceEvent(userID string, event apitypes.PresenceEventBody) error {
	fmt.Printf("Processing presence event: %v\n", event)

	// Get existing user activity
	ua, err := db.GetUserActivity(userID)
	if err != nil {
		fmt.Printf("failed to get user activity: %v\n", err)
	}
	if ua == nil {
		fmt.Printf("no user activity found, lazy initializing\n")
		// Lazy init
		ua = db.CreateUserActivity(userID)
	}

	// debug ua
	uaBytes, err := json.Marshal(ua)
	if err == nil {
		fmt.Printf("User activity (1): %s\n", string(uaBytes))
	}

	// Set current presence
	ua.Presence = event.PresenceDefinition.SystemPresence

	// Check if inactive
	ua.CheckActivity()

	// debug ua
	uaBytes, err = json.Marshal(ua)
	if err == nil {
		fmt.Printf("User activity (2): %s\n", string(uaBytes))
	}

	// Write to database
	if err := db.WriteUserActivity(*ua); err != nil {
		fmt.Printf("failed to write user activity: %v\n", err)
	}

	return nil
}

func processConversationSummaryEvent(userID string, event apitypes.ConversationSummaryEventBody) error {
	fmt.Printf("Processing conversation summary event: %v\n", event)

	// Get existing user activity
	ua, err := db.GetUserActivity(userID)
	if err != nil {
		fmt.Printf("failed to get user activity: %v\n", err)
	}
	if ua == nil {
		fmt.Printf("no user activity found, lazy initializing\n")
		// Lazy init
		ua = db.CreateUserActivity(userID)
	}

	// debug ua
	uaBytes, err := json.Marshal(ua)
	if err == nil {
		fmt.Printf("User activity (1): %s\n", string(uaBytes))
	}

	// Set current conversations
	ua.UpdateConversations(event)

	// Check if inactive
	ua.CheckActivity()

	// debug ua
	uaBytes, err = json.Marshal(ua)
	if err == nil {
		fmt.Printf("User activity (2): %s\n", string(uaBytes))
	}

	// Write to database
	if err := db.WriteUserActivity(*ua); err != nil {
		fmt.Printf("failed to write user activity: %v\n", err)
	}

	return nil
}
