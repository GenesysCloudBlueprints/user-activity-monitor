package apitypes

import "time"

type EventBridgeEvent struct {
	Version    string      `json:"version"`
	ID         string      `json:"id"`
	DetailType string      `json:"detail-type"`
	Source     string      `json:"source"`
	Account    string      `json:"account"`
	Time       time.Time   `json:"time"`
	Region     string      `json:"region"`
	Resources  []string    `json:"resources"`
	Detail     EventDetail `json:"detail"`
}

type EventDetail struct {
	TopicName string                 `json:"topicName"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	EventBody map[string]interface{} `json:"eventBody"`
	Metadata  EventMetadata          `json:"metadata"`
}

type EventMetadata struct {
	CorrelationId string `json:"CorrelationId"`
}

type PresenceEventBody struct {
	Message            string             `json:"message"`
	ModifiedDate       time.Time          `json:"modifiedDate"`
	PresenceDefinition PresenceDefinition `json:"presenceDefinition"`
	Source             string             `json:"source"`
}

type PresenceDefinition struct {
	ID             string `json:"id"`
	SystemPresence string `json:"systemPresence"`
}

type ConversationSummaryEventBody struct {
	Call             ChannelMetrics `json:"call"`
	Callback         ChannelMetrics `json:"callback"`
	Chat             ChannelMetrics `json:"chat"`
	Email            ChannelMetrics `json:"email"`
	Message          ChannelMetrics `json:"message"`
	SocialExpression ChannelMetrics `json:"socialExpression"`
	Video            ChannelMetrics `json:"video"`
}

type ChannelMetrics struct {
	ContactCenter ChannelActivity `json:"contactCenter"`
	Enterprise    ChannelActivity `json:"enterprise"`
}

type ChannelActivity struct {
	Active int `json:"active"`
	ACW    int `json:"acw"`
}
