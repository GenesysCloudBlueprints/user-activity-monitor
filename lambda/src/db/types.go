package db

import (
	"fmt"
	"strings"
	"time"
	"user-activity-monitor/src/apitypes"
	"user-activity-monitor/src/genesys"
	"user-activity-monitor/src/groupconfig"
)

/**
 * Design note: types written to the DB must use the dynamodbav tag; the AWS SDK ignores the json tag
 * https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue#Marshal
 */

const (
	userActivityPrefix = "ua"
)

// singleTableEntity provides the PK and SK for a single table entity
type singleTableEntity struct {
	PartitionKey string `json:"_pk" dynamodbav:"_pk"`
	SortKey      string `json:"_sk" dynamodbav:"_sk"`
	TTL          *int64 `json:"_ttl,omitempty" dynamodbav:"_ttl,omitempty"`
}

type singleTableEntityListGSI struct {
	ListItemGSIPK string `json:"_gsi_list_pk" dynamodbav:"_gsi_list_pk"`
	ListItemGSISK string `json:"_gsi_list_sk" dynamodbav:"_gsi_list_sk"`
}

// UserActivity indicates the last known activity for a user
type UserActivity struct {
	UserID              string `json:"userId" dynamodbav:"userId"`
	Presence            string `json:"presence" dynamodbav:"presence"`
	SecondaryPresenceID string `json:"secondaryPresenceId" dynamodbav:"secondaryPresenceId"`
	Conversing          bool   `json:"conversing" dynamodbav:"conversing"`
	GroupID             string `json:"groupId" dynamodbav:"groupId"`
	InactivityTTL       *int64 `json:"inactivityTTL" dynamodbav:"inactivityTTL"`
	LastUpdated         int64  `json:"lastUpdated" dynamodbav:"lastUpdated"`
}

// UserActivityEntity is an aggregate type for the DB record for a UserActivity object
type UserActivityEntity struct {
	singleTableEntity
	singleTableEntityListGSI
	UserActivity
}

func UserActivityPK(userID string) string {
	return fmt.Sprintf("%s|%s", userActivityPrefix, userID)
}

func UserActivitySK(userID string) string {
	return fmt.Sprintf("%s|%s", userActivityPrefix, userID)
}

func UserActivityListGSIPK(inactivityTTL *int64) string {
	status := "pending"
	if inactivityTTL == nil || *inactivityTTL < time.Now().UnixMilli() {
		status = "exempt"
	}

	return strings.ToLower(fmt.Sprintf("%s|%s", userActivityPrefix, status))
}

func UserActivityListGSISK(inactivityTTL *int64) string {
	if inactivityTTL == nil {
		return "0"
	}

	return fmt.Sprintf("%v", *inactivityTTL)
}

func (ua UserActivity) PK() string {
	return UserActivityPK(ua.UserID)
}

func (ua UserActivity) SK() string {
	return UserActivitySK(ua.UserID)
}

func (ua UserActivity) ListGSIPK() string {
	return UserActivityListGSIPK(ua.InactivityTTL)
}

func (ua UserActivity) ListGSISK() string {
	return UserActivityListGSISK(ua.InactivityTTL)
}

// Entity creates a DB entity from the UserActivity object
func (ua UserActivity) Entity() UserActivityEntity {
	return UserActivityEntity{
		singleTableEntity: singleTableEntity{
			PartitionKey: ua.PK(),
			SortKey:      ua.SK(),
			TTL:          &[]int64{time.Now().AddDate(0, 1, 0).UnixMilli()}[0],
		},
		singleTableEntityListGSI: singleTableEntityListGSI{
			ListItemGSIPK: ua.ListGSIPK(),
			ListItemGSISK: ua.ListGSISK(),
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
	if ua.GroupID == "" {
		ua.ClearInactivityTTL()
	} else {
		ua.SetInactivityTTL(time.Duration(groupconfig.TimeoutGroups[ua.GroupID].TimeoutMinutes) * time.Minute)
	}
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
