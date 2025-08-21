package db

import (
	"fmt"
	"time"
	"user-activity-monitor/src/apitypes"
	"user-activity-monitor/src/genesys"
	"user-activity-monitor/src/groupconfig"
)

/**
 * Design note: types written to the DB must use the dynamodbav tag; the AWS SDK ignores the json tag
 * https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue#Marshal
 */

const userActivityPrefix = "ua"

// singleTableEntity provides the PK and SK for a single table entity
type singleTableEntity struct {
	PartitionKey string `json:"_pk" dynamodbav:"_pk"`
	SortKey      string `json:"_sk" dynamodbav:"_sk"`
	TTL          *int64 `json:"_ttl,omitempty" dynamodbav:"_ttl,omitempty"`
}

// UserActivity indicates the last known activity for a user
type UserActivity struct {
	UserID              string `json:"userId" dynamodbav:"userId"`
	Presence            string `json:"presence" dynamodbav:"presence"`
	SecondaryPresenceID string `json:"secondaryPresenceId" dynamodbav:"secondaryPresenceId"`
	Conversing          bool   `json:"conversing" dynamodbav:"conversing"`
	GroupID             string `json:"groupId" dynamodbav:"groupId"`
	InactivityTTL       *int64 `json:"inactivityTTL" dynamodbav:"inactivityTTL"`
}

// UserActivityEntity is an aggregate type for the DB record for a UserActivity object
type UserActivityEntity struct {
	singleTableEntity
	UserActivity
}

func UserActivityPK(userID string) string {
	return fmt.Sprintf("%s|%s", userActivityPrefix, userID)
}

func UserActivitySK(userID string) string {
	return fmt.Sprintf("%s|%s", userActivityPrefix, userID)
}

func (ua UserActivity) PK() string {
	return UserActivityPK(ua.UserID)
}

func (ua UserActivity) SK() string {
	return UserActivitySK(ua.UserID)
}

// Entity creates a DB entity from the UserActivity object
func (ua UserActivity) Entity() UserActivityEntity {
	return UserActivityEntity{
		singleTableEntity: singleTableEntity{
			PartitionKey: ua.PK(),
			SortKey:      ua.SK(),
			TTL:          &[]int64{time.Now().AddDate(0, 0, 7).UnixMilli()}[0],
		},
		UserActivity: ua,
	}
}

// SetInactivityTTL sets the inactivity TTL to the current time plus the duration
func (ua *UserActivity) SetInactivityTTL(duration time.Duration) {
	ua.InactivityTTL = &[]int64{time.Now().Add(duration).UnixMilli()}[0]
}

// ClearInactivityTTL clears the inactivity TTL
func (ua *UserActivity) ClearInactivityTTL() {
	ua.InactivityTTL = nil
}

// RefreshInactivityTTL refreshes the inactivity TTL based on the assigned timeout group
func (ua *UserActivity) RefreshInactivityTTL() {
	ua.SetInactivityTTL(time.Duration(groupconfig.TimeoutGroups[ua.GroupID].TimeoutMinutes) * time.Minute)
}

// CheckActivity checks the current activity data and sets the inactivity TTL accordingly
func (ua *UserActivity) CheckActivity() {
	// Clear TTL or update it
	if ua.GroupID == "" || ua.Conversing || groupconfig.IsPresenceTTLExempt(ua.Presence) {
		ua.ClearInactivityTTL()
	} else {
		ua.SetInactivityTTL(time.Duration(groupconfig.TimeoutGroups[ua.GroupID].TimeoutMinutes) * time.Minute)
	}
}

// UpdateConversations updates the conversing flag based on the conversation summary
func (ua *UserActivity) UpdateConversations(conversationSummary apitypes.ConversationSummaryEventBody) {
	ua.Conversing = conversationSummary.Call.ContactCenter.Active > 0 ||
		conversationSummary.Call.ContactCenter.ACW > 0 ||
		conversationSummary.Call.Enterprise.Active > 0 ||
		conversationSummary.Callback.ContactCenter.Active > 0 ||
		conversationSummary.Callback.ContactCenter.ACW > 0 ||
		conversationSummary.Callback.Enterprise.Active > 0 ||
		conversationSummary.Chat.ContactCenter.Active > 0 ||
		conversationSummary.Chat.ContactCenter.ACW > 0 ||
		conversationSummary.Chat.Enterprise.Active > 0 ||
		conversationSummary.Email.ContactCenter.Active > 0 ||
		conversationSummary.Email.ContactCenter.ACW > 0 ||
		conversationSummary.Email.Enterprise.Active > 0 ||
		conversationSummary.Message.ContactCenter.Active > 0 ||
		conversationSummary.Message.ContactCenter.ACW > 0 ||
		conversationSummary.Message.Enterprise.Active > 0 ||
		conversationSummary.SocialExpression.ContactCenter.Active > 0 ||
		conversationSummary.SocialExpression.ContactCenter.ACW > 0 ||
		conversationSummary.SocialExpression.Enterprise.Active > 0 ||
		conversationSummary.Video.ContactCenter.Active > 0 ||
		conversationSummary.Video.ContactCenter.ACW > 0 ||
		conversationSummary.Video.Enterprise.Active > 0
}

// RefreshUser fetches the current Genesys user data and fully updates the UserActivity object
func (ua *UserActivity) RefreshUser() {
	// Get current user data
	genesysUser, err := genesys.GetUser(ua.UserID)
	if err != nil {
		fmt.Printf("failed to get Genesys user: %v\n", err)
	}

	// Update user activity with current data
	ua.GroupID = chooseTimeoutGroupID(genesysUser.Groups)
	ua.Presence = genesysUser.Presence.PresenceDefinition.SystemPresence
	ua.SecondaryPresenceID = genesysUser.Presence.PresenceDefinition.ID
	ua.UpdateConversations(genesysUser.ConversationSummary)

	// Check activity
	ua.CheckActivity()
}

// chooseTimeoutGroupID chooses the timeout group with the longest timeout from the list of assigned groups
func chooseTimeoutGroupID(genesysGroups []genesys.GenesysGroup) string {
	var targetGroupID string
	var targetGroup *groupconfig.TimeoutGroup

	// Choose group with longest timeout
	for _, genesysGroup := range genesysGroups {
		if _, ok := groupconfig.TimeoutGroups[genesysGroup.ID]; ok {
			timeoutGroup := groupconfig.TimeoutGroups[genesysGroup.ID]
			if targetGroup == nil || timeoutGroup.TimeoutMinutes > targetGroup.TimeoutMinutes {
				targetGroupID = genesysGroup.ID
				targetGroup = &timeoutGroup
			}
		}
	}

	if targetGroupID == "" {
		fmt.Println("No timeout group found for user")
		return ""
	}

	fmt.Printf("Using timeout group: %s (%v)\n", targetGroupID, targetGroup.TimeoutMinutes)

	return targetGroupID
}
