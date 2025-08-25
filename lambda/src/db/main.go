package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var userActivityTableName = os.Getenv("DYNAMODB_TABLE")
var userActivityListGSI = os.Getenv("DYNAMODB_GSI_LIST")
var client *dynamodb.Client
var ctx = context.Background()

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}
	client = dynamodb.NewFromConfig(cfg)
}

func CreateUserActivity(userID string) *UserActivity {
	ua := UserActivity{
		UserID: userID,
	}
	ua.RefreshUser()
	return &ua
}

// WriteUserActivity writes a UserActivity object to the user activity table
func WriteUserActivity(ua UserActivity, isLogoutAction bool) error {
	if isLogoutAction {
		// Clear inactivity TTL so the user activity record is not processed by the reaper lambda function again
		ua.ClearInactivityTTL()
	} else {
		// Refresh inactivity TTL
		ua.CheckActivity()
	}

	// Update last updated timestamp
	ua.LastUpdated = time.Now().UnixMilli()

	// Convert to DynamoDB object
	av, err := attributevalue.MarshalMap(ua.Entity())
	if err != nil {
		return fmt.Errorf("failed to marshal UserActivity to DynamoDB: %v", err)
	}

	// debug av
	avBytes, err := json.Marshal(av)
	if err == nil {
		fmt.Printf("User activity DB record (1): %s\n", string(avBytes))
	}

	// Write to DynamoDB
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &userActivityTableName,
		Item:      av,
	})

	if err != nil {
		return fmt.Errorf("failed to write UserActivity to DynamoDB: %v", err)
	}

	fmt.Printf("Successfully wrote for %s\n", ua.UserID)
	return nil
}

func GetUserActivity(userID string) (*UserActivity, error) {
	// Get user activity from DynamoDB
	pk := UserActivityPK(userID)
	sk := UserActivitySK(userID)
	av, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &userActivityTableName,
		Key: map[string]types.AttributeValue{
			"_pk": &types.AttributeValueMemberS{Value: pk},
			"_sk": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get UserActivity from DynamoDB: %v", err)
	}

	// Create a new UserActivity object if it doesn't exist
	if av == nil || len(av.Item) == 0 {
		fmt.Printf("User activity not found for %s, creating new record\n", userID)
		return CreateUserActivity(userID), nil
	}

	// Unmarshal the DB record into a UserActivityEntity object
	var ua UserActivityEntity
	err = attributevalue.UnmarshalMap(av.Item, &ua)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal UserActivity from DynamoDB: %v", err)
	}

	// Refresh expired records
	if ua.UserActivity.InactivityTTL != nil && *ua.UserActivity.InactivityTTL < time.Now().UnixMilli() {
		ua.UserActivity.RefreshUser()
	}

	// Return the unpacked UserActivity object
	return &ua.UserActivity, nil
}

func ListUserActivity(pending bool, beforeTime *int64) ([]UserActivity, error) {
	status := "pending"
	if !pending {
		status = "exempt"
	}

	// Define query conditions
	keyCondition := expression.Key("_gsi_list_pk").Equal(expression.Value(fmt.Sprintf("%s|%s", userActivityPrefix, status)))

	// Add sort key condition if beforeTime is provided
	if beforeTime != nil {
		keyCondition = expression.KeyAnd(keyCondition, expression.Key("_gsi_list_sk").LessThan(expression.Value(fmt.Sprintf("%d", *beforeTime))))
	}

	expr, err := expression.NewBuilder().WithKeyCondition(keyCondition).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %v", err)
	}

	// Query the GSI to get all items with the specified status
	query := &dynamodb.QueryInput{
		TableName:                 &userActivityTableName,
		IndexName:                 aws.String(userActivityListGSI),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	// debug query
	queryBytes, err := json.Marshal(query)
	if err == nil {
		fmt.Printf("Query: %s\n", string(queryBytes))
	}

	// Collect all results across all pages
	var uaList []UserActivity
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		// Set the exclusive start key for pagination
		if lastEvaluatedKey != nil {
			query.ExclusiveStartKey = lastEvaluatedKey
		}

		result, err := client.Query(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("failed to query UserActivity from DynamoDB: %v", err)
		}

		// Process items from this page
		for _, item := range result.Items {
			var ua UserActivityEntity
			err := attributevalue.UnmarshalMap(item, &ua)
			if err != nil {
				fmt.Printf("Warning: failed to unmarshal item: %v\n", err)
				continue
			}
			uaList = append(uaList, ua.UserActivity)
		}

		// Check if there are more pages
		if result.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = result.LastEvaluatedKey
	}

	return uaList, nil
}
