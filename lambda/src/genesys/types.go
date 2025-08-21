package genesys

import "user-activity-monitor/src/apitypes"

type GenesysUser struct {
	ID                  string                                `json:"id"`
	Name                string                                `json:"name"`
	State               string                                `json:"state"`
	Groups              []GenesysGroup                        `json:"groups"`
	Presence            apitypes.PresenceEventBody            `json:"presence"`
	ConversationSummary apitypes.ConversationSummaryEventBody `json:"conversationSummary"`
}

type GenesysGroup struct {
	ID      string `json:"id"`
	SelfURI string `json:"selfUri"`
}
