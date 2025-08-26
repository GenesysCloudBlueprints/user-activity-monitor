package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"time"
	"user-activity-monitor/src/apitypes"

	"github.com/aws/aws-lambda-go/lambda"
)

var presenceUserRegex = regexp.MustCompile(`^v2\.users\.([a-z0-9]{8}-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{12})\.presence$`)
var conversationUserRegex = regexp.MustCompile(`^v2\.users\.([a-z0-9]{8}-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{12})\.conversationsummary$`)

func main() {
	lambda.Start(handleRequestLogger)
}

func handleRequestLogger(ctx context.Context, event interface{}) error {
	err := handleRequest(ctx, event)
	if err != nil {
		log.Printf("Error handling request: %v", err)
	}
	return err
}

func handleRequest(ctx context.Context, event interface{}) error {
	start := time.Now()

	// Parse the event
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %v", err)
	} else {
		// DEBUG
		// fmt.Printf("Received event: %s\n", string(eventBytes))
	}

	// Unmarshal event
	var eventBridgeEvent apitypes.EventBridgeEvent
	if err := json.Unmarshal(eventBytes, &eventBridgeEvent); err != nil {
		return fmt.Errorf("failed to unmarshal event: %v", err)
	}
	switch eventBridgeEvent.DetailType {
	case "v2.users.{id}.presence":
		{
			fmt.Printf("Received presence event: %v\n", eventBridgeEvent)

			// Parse event body
			var presenceEventBody apitypes.PresenceEventBody
			if err := parseEventBody(eventBridgeEvent.Detail.EventBody, &presenceEventBody); err != nil {
				return err
			}

			// Get user ID
			userID := extractUserIDFromPresenceTopic(eventBridgeEvent.Detail.TopicName)

			// Process event
			processPresenceEvent(userID, presenceEventBody)
		}
	case "v2.users.{id}.conversationsummary":
		{
			fmt.Printf("Received conversation summary event: %v\n", eventBridgeEvent)

			// Parse event body
			var conversationSummaryEventBody apitypes.ConversationSummaryEventBody
			if err := parseEventBody(eventBridgeEvent.Detail.EventBody, &conversationSummaryEventBody); err != nil {
				return err
			}

			// Get user ID
			userID := extractUserIDFromConversationSummaryTopic(eventBridgeEvent.Detail.TopicName)

			// Process event
			processConversationSummaryEvent(userID, conversationSummaryEventBody)
		}
	default:
		fmt.Printf("unexpected event: %v\n", eventBridgeEvent)
		return nil
	}

	// print a success message indicating how long it took to process the event
	fmt.Printf("Successfully processed event in %v\n", time.Since(start))
	return nil
}

// extractUserIDFromTopic extracts the user ID from the topic name
func extractUserIDFromPresenceTopic(topicName string) string {
	matches := presenceUserRegex.FindStringSubmatch(topicName)
	if len(matches) > 1 {
		return matches[1]
	} else {
		fmt.Printf("failed to extract user ID from topic: %s\n", topicName)
		return ""
	}
}

// extractUserIDFromConversationSummaryTopic extracts the user ID from the topic name
func extractUserIDFromConversationSummaryTopic(topicName string) string {
	matches := conversationUserRegex.FindStringSubmatch(topicName)
	if len(matches) > 1 {
		return matches[1]
	} else {
		fmt.Printf("failed to extract user ID from topic: %s\n", topicName)
		return ""
	}
}

// parseEventBody parses the event body from an interface{} and unmarshals it into the provided pointer
func parseEventBody(eventBody interface{}, result interface{}) error {
	eventBodyBytes, err := json.Marshal(eventBody)
	if err != nil {
		return fmt.Errorf("failed to marshal event body: %v", err)
	}

	if err := json.Unmarshal(eventBodyBytes, result); err != nil {
		return fmt.Errorf("failed to unmarshal event body: %v", err)
	}

	return nil
}
