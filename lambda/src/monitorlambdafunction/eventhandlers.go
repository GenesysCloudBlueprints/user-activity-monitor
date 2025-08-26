package main

import (
	"fmt"
	"strings"
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

	if strings.EqualFold(ua.Presence, "OFFLINE") && !strings.EqualFold(event.PresenceDefinition.SystemPresence, "OFFLINE") {
		// Refresh user's config when they come back online
		ua.RefreshUser()
	} else {
		// Set current presence
		ua.Presence = event.PresenceDefinition.SystemPresence
		ua.SecondaryPresenceID = event.PresenceDefinition.ID
	}

	// Check
	ua.CheckActivity()

	// Write to database
	if err := db.WriteUserActivity(*ua, false); err != nil {
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

	// Set current conversations
	ua.UpdateConversations(event)

	// Check
	ua.CheckActivity()

	// Write to database
	if err := db.WriteUserActivity(*ua, false); err != nil {
		fmt.Printf("failed to write user activity: %v\n", err)
	}

	return nil
}
