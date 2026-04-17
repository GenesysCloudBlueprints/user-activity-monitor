# User Activity Monitor

The [user-activity-monitor](https://github.com/GenesysCloudBlueprints/user-activity-monitor/) blueprint is an EventBridge Serverless Lambda integration that monitors user presence and conversation activity to identify idle users and log them out of Genesys Cloud.

## Solution Overview

The solution utilizes the [Genesys Cloud EventBridge](https://developer.genesys.cloud/notificationsalerts/notifications/event-bridge) integration to receive the [v2.users.{id}.conversationsummary](https://developer.genesys.cloud/notificationsalerts/notifications/available-topics#v2-users--id--conversationsummary) and [v2.users.{id}.presence](https://developer.genesys.cloud/notificationsalerts/notifications/available-topics#v2-users--id--presence) events. The data in the events is inspected to determine if the user should be considered inactive and scheduled for being logged out. This is determined by checking to see if the user is not in an exempt presence (offline or the ACD on queue and idle presences) and not on any conversation or ACW. If neither of these are true, the user's configuration is checked to determine their timeout period:

- If no group is matched, the user is not subject to being logged out and may remain idle indefinitely
- If one group is matched, that group's timeout is used
- If multiple groups are matched, the longest timeout is used

Any time a user's presence or conversations are updated, their eligibility for being logged out is recalculated and the TTL is recalculated based on the current time or removed.

The activity reaper lambda function runs every 5 minutes to log out inactive users. It queries the database cache of event data to find records with an inactivity TTL that has passed. It uses [DELETE /api/v2/tokens/{userId}](https://developer.genesys.cloud/devapps/api-explorer-standalone#delete-api-v2-tokens--userId-) to log those users out. When a user is logged out this way, they will see the Genesys Cloud app redirect to the login screen to prompt them to log in again.

### Activity Report

The solution provides some observability via an activity report web page. This is a simple app that:

- loads a static HTML page via API Gateway and lambda
- uses Genesys Cloud **Code Authorization with PKCE** OAuth grant to get an auth token to verify the user's identity
- loads activity data via an authorized API Gateway route; the lambda function validates that the auth token is valid and the user belongs to the org
- displays the activity data
- can be served within Genesys Cloud as a client app

The following screenshot shows data from a demo org to illustrate what a supervisor/admin will see when viewing the report in their org. The report shows all users known to the integration in the last 30 days, with users that have a pending inactivity TTL shown first. The report shows the current presence, when the record was last updated, the group their timeout came from (and what it is), and finally the timestamp when they will be subject to being logged out by the activity reaper lambda function.

![user activity](https://developer.genesys.cloud/blueprints/user-activity-monitor/activity.png)

### Architecture

The solution is deployed to AWS using a serverless template. It contains the following components:

- **EventBridge Event Bus** - associates the [Genesys Cloud partner event](https://developer.genesys.cloud/notificationsalerts/notifications/event-bridge#manage-your-amazon-eventbridge-partner-source) with this integration
- **Lambda Functions**
  - MonitorUserActivity - handles EventBridge events
  - ActivityReaper - timed function that uses [DELETE /api/v2/tokens/{userId}](https://developer.genesys.cloud/devapps/api-explorer-standalone#delete-api-v2-tokens--userId-) to log out users that have surpassed their inactivity TTL
  - GenerateReport - serves the activity report page and data, access controlled via Genesys Cloud authorization
  - **OAuthCallback** - handles OAuth Code Authorization callback and token exchange with PKCE
- **Dynamo DB** - caches user activity from EventBridge events
- **Secrets Manager** - securely stores Genesys Cloud client credentials used for interacting with the Genesys Cloud Platform API
- **API Gateway** - only used as the public gateway for the GenerateReport and OAuthCallback functions to serve the web page and data; not required for the EventBridge portion of the integration

![architecture diagram](https://developer.genesys.cloud/blueprints/user-activity-monitor/arch.png)

### OAuth Implementation

The application uses **two separate OAuth clients** in Genesys Cloud:

1. **Client Credentials OAuth Client** (Backend)
   - Grant Type: Client Credentials
   - Purpose: Backend Lambda functions (MonitorUserActivity, ActivityReaper) for automated tasks
   - Stored in: AWS Secrets Manager

2. **Code Authorization OAuth Client** (Frontend)
   - Grant Type: Code Authorization with PKCE
   - Purpose: User-facing Activity Report authentication
   - Security: PKCE (Proof Key for Code Exchange) prevents authorization code interception attacks

**Why Two Clients?**
Genesys Cloud does not allow both grant types on the same OAuth client. The backend functions require Client Credentials for automated API access without user interaction, while the frontend requires Code Authorization for secure user login.

**Migration from Implicit Grant:**
This implementation uses Code Authorization with PKCE instead of the deprecated Implicit Grant flow. Code Authorization is more secure as it:
- Exchanges authorization codes for tokens on the backend
- Uses PKCE to prevent code interception attacks
- Keeps tokens out of browser history
- Follows OAuth 2.0 best practices

## Building and Deployment

### Prerequisites

- Go 1.23
- NodeJS 22
- [NVM](https://github.com/nvm-sh/nvm) (node version manager, optional)

### Genesys Cloud Setup

#### Create Role

[Create a new role](https://help.genesys.cloud/articles/add-roles/) for assigning permissions to the client credentials. Give this role the `OAuth > Token > Delete` permission.

Assign this role to yourself for all divisions so you can grant it to the client credentials later.

#### Create Client Credentials OAuth Client (Backend)

[Create an OAuth client](https://help.genesys.cloud/articles/create-an-oauth-client/) using the **client credentials grant**. Assign it the role created above for all divisions.

Store the client secret in your organization's approved password management solution. You will need to retrieve it later to set in AWS Secrets Manager.

Take note of the **Client ID** and **Client Secret** for later.

#### Create Code Authorization OAuth Client (Frontend)

[Create an OAuth client](https://help.genesys.cloud/articles/create-an-oauth-client/) using the **code authorization grant**. Configure:

- **Grant Type**: Code Authorization
- **Token Duration**: 3600 seconds (1 hour)
- **Redirect URIs**: (Add after deployment - see "Update Code Authorization Redirect URIs" section)

Take note of the **Client ID** for later. The client secret is not needed for Code Authorization with PKCE.

#### Install Event Bridge Integration

[Add an integration](https://help.genesys.cloud/articles/add-an-integration/), specifically the _Amazon EventBridge Source_. See [Configure the Amazon EventBridge integration](https://help.genesys.cloud/articles/configure-the-amazon-eventbridge-integration/) for instructions.

The _Event Source Suffix_ configured here must match the value set for `genesysCloudEventSourceSuffix` in _serverless.yml_. This is `user-activity-monitor`, unless you changed it.

For _Topic Filtering_, select [v2.users.{id}.conversationsummary](https://developer.genesys.cloud/notificationsalerts/notifications/available-topics#v2-users--id--conversationsummary) and [v2.users.{id}.presence](https://developer.genesys.cloud/notificationsalerts/notifications/available-topics#v2-users--id--presence).

Save the configuration and activate the integration.

#### Create Groups

A group's purpose is to assign an inactivity timeout duration to a selection of users. Create as many groups as are necessary for your timeouts. The name can be anything you prefer. Assign users as members of the group, setting the "owners as members" setting to your liking. See [Create a general group](https://help.genesys.cloud/articles/create-group/).

Take note of the group GUIDs in the URL when viewing the group or look them up using [GET /api/v2/groups](https://developer.genesys.cloud/devapps/api-explorer-standalone#get-api-v2-groups). You'll use them when setting the timeouts in `groups.go`.

### Local Setup

After cloning [user-activity-monitor](https://github.com/GenesysCloudBlueprints/user-activity-monitor/), run these commands in the root of the repo to prepare the environment:

```shell
nvm use
npm i -g serverless
```

### Required Configuration

There are a few things that must be updated in the codebase prior to building or deployment.

#### Update `serverless.yml`

Near the top of the serverless config file, update these variables. Other variables can remain as-is.

- `genesysCloudOrgId` - your organization ID (e.g. from [Organization Settings](https://apps.mypurecloud.com/directory/#/admin/organization))
- `clientCredentialsClientId` - the **backend** Client Credentials OAuth client ID
- `codeAuthClientId` - the **frontend** Code Authorization OAuth client ID

```yaml
# Variables
custom:
  genesysCloud:
    # Genesys Cloud details https://help.genesys.cloud/articles/configure-the-amazon-eventbridge-integration/
    genesysCloudOrgId: YOUR_GENESYS_ORG_ID_HERE
    genesysCloudEventSourceSuffix: user-activity-monitor
    # Partner event source https://developer.genesys.cloud/notificationsalerts/notifications/event-bridge#manage-your-amazon-eventbridge-partner-source
    eventSource: "aws.partner/genesys.com/cloud/${self:custom.genesysCloud.genesysCloudOrgId}/${self:custom.genesysCloud.genesysCloudEventSourceSuffix}"
    # Client Credentials OAuth Client ID (for backend Lambda functions)
    clientCredentialsClientId: YOUR_BACKEND_CLIENT_ID_HERE
    # Code Authorization OAuth Client ID (for frontend user login)
    codeAuthClientId: YOUR_FRONTEND_CLIENT_ID_HERE

provider:
  environment:
    GENESYS_API_DOMAIN: YOUR_GENESYS_DOMAIN_HERE  # e.g., mypurecloud.com, mypurecloud.ie, mypurecloud.com.au, etc.
```

**Genesys Domain by Region:**
- Americas (US East): `mypurecloud.com`
- Americas (US West): `usw2.pure.cloud`
- Americas (Canada): `cac1.pure.cloud`
- Americas (São Paulo): `sae1.pure.cloud`
- EMEA (Ireland): `mypurecloud.ie`
- EMEA (London): `euw2.pure.cloud`
- EMEA (Frankfurt): `mypurecloud.de`
- APAC (Sydney): `mypurecloud.com.au`
- APAC (Tokyo): `mypurecloud.jp`
- APAC (Mumbai): `inindca.com`
- APAC (Seoul): `apne2.pure.cloud`

#### Update `lambda/src/groupconfig/groups.go`

Update `TimeoutGroups`, the configuration mapping for groups to timeouts. The map's key is the group's GUID, noted when creating it or looking it up via the API. Each value in the map should provide the name of the group (for display purposes) and the inactivity timeout for users in that group.

```go
var TimeoutGroups = map[string]TimeoutGroup{
    "e613e69c-a2d4-40fc-aba5-a9a5eb43eeef": {
        Name:           "Timeout Group - agents",
        TimeoutMinutes: 15,
    },
    "f42fd8d0-3c9b-4db4-b389-c845fcef92c9": {
        Name:           "Timeout Group - supervisors",
        TimeoutMinutes: 60,
    },
}
```

The `IsPresenceTTLExempt` function determines if a user is exempt from the inactivity TTL if they are offline, ACD idle, or ACD on queue. This list of exempt system presences can be changed by altering the return expression.

```go
// IsPresenceTTLExempt checks if the presence is exempt from the TTL (i.e. offline or ACD)
func IsPresenceTTLExempt(systemPresence string) bool {
    p := strings.ToLower(systemPresence)
    return p == "offline" || p == "idle" || p == "on_queue"
}
```

### Building

In the `lambda` directory, run `make build` to compile the artifacts for the lambda function deployments. Each function is compiled to its own binary in its own folder and zipped individually. The build output is located in `lambda/dist`.

### Deploying to AWS

The deployment process uses the serverless node package to interface with the AWS CLI. See [Setting up AWS credentials](https://www.npmjs.com/package/serverless#setting-up-aws-credentials) in the serverless documentation to ensure your CLI is authorized before attempting a deployment.

In the root of the repo, run `./scripts/deploy.sh` to create or update the AWS stack.

#### Setting the Client Credentials in Secrets Manager

After the first successful deployment, set the **backend** client ID and secret in AWS Secrets Manager in the [AWS Console](https://us-east-1.console.aws.amazon.com/secretsmanager/secret?name=user-activity-monitor-client-credentials). This does not need to be repeated unless the stack has been deleted.

```bash
aws secretsmanager create-secret \
  --name user-activity-monitor-client-credentials \
  --secret-string '{
    "clientId":"YOUR_BACKEND_CLIENT_ID",
    "clientSecret":"YOUR_BACKEND_CLIENT_SECRET"
  }' \
  --region YOUR_AWS_REGION
```

:::{"alert":"info","title":"Handle OAuth client secrets like passwords!","collapsible":false,"autoCollapse":false}
Do not put the client credentials in the serverless template! Setting them via the console creates a manual step, but prevents your credentials from being stored insecurely at rest on your computer, or worse, checked in to your repo.
:::

### Update Code Authorization Redirect URIs

Now that the stack is deployed, you can get the API Gateway hostname needed for the redirect URIs. The outputs for the stack have this hostname in `ServiceEndpoint`. 

Add these redirect URIs to your **frontend** Code Authorization OAuth client in Genesys Cloud:

```
https://xxxxxxxxxx.execute-api.us-east-1.amazonaws.com/deploy/callback
https://xxxxxxxxxx.execute-api.us-east-1.amazonaws.com/deploy/report
```

Replace `xxxxxxxxxx` with the value from your API Gateway hostname.

**Important:** Ensure exact match - no trailing slash, no extra spaces.

### Add Genesys Cloud Client App (optional)

If you wish to view the activity report from within the Genesys Cloud app, [set up a client application integration](https://help.genesys.cloud/articles/set-custom-client-application-integration/), specifically the one named `Client Application`.

The _Application URL_ is `https://xxxxxxxxxx.execute-api.us-east-1.amazonaws.com/deploy/report`.

The _Application Type_ should be _standalone_.

Select appropriate groups to show the app to.

Save the integration and activate it. Users will have to refresh their client to see the app listed in the top "Apps" menu as the name you gave it.

## Code Overview

The `lambda/src/` folder contains the Go source code for the Lambda functions and supporting packages:

### Core Lambda Functions

- **`monitorlambdafunction/`** - Main event processing function that:
  - Listens for Genesys Cloud presence and conversation events via EventBridge
  - Processes user presence changes and conversation activity
  - Updates user activity records in DynamoDB

- **`reaperlambdafunction/`** - User logout function that:
  - Runs on a schedule to identify idle users
  - Logs out users who have exceeded their inactivity timeout
  - Updates user activity records after logout operations

- **`reportlambdafunction/`** - Web interface and reporting function that:
  - Provides a web dashboard for monitoring user activity
  - Serves HTML reports of its cached activity data
  - Integrates with Genesys Cloud API to fetch user details
  - Supports Bearer token authentication for secure access to reports

- **`callbacklambdafunction/`** - OAuth callback handler that:
  - Receives authorization code from Genesys Cloud OAuth
  - Retrieves PKCE code verifier from cookie
  - Exchanges authorization code for access token using PKCE
  - Returns access token to frontend via URL hash

### Supporting Packages

- **`apitypes/`** - Common data structures for:
  - EventBridge event formats
  - Genesys Cloud presence and conversation event bodies
  - API response types and metadata

- **`db/`** - Database operations and data models:
  - DynamoDB single-table design patterns
  - User activity entity definitions and operations
  - TTL management for session cleanup
  - Global Secondary Index (GSI) structures for efficient queries

- **`genesys/`** - Genesys Cloud API integration:
  - User management and presence operations
  - Authentication and session management
  - User logout functionality
  - API response handling and data structures

- **`groupconfig/`** - Configuration management:
  - Timeout group definitions with configurable inactivity periods
  - Group-based timeout rules (e.g., agents: 15 minutes, supervisors: 60 minutes)
  - Presence state exemption logic for offline/idle users

## OAuth Code Authorization with PKCE Implementation

### Frontend (app.html)

The activity report page implements the OAuth Code Authorization flow with PKCE:

1. **Generate PKCE Parameters:**
   - Creates a cryptographically secure code verifier (32 random bytes, base64url encoded)
   - Generates code challenge as SHA-256 hash of the verifier

2. **Store Code Verifier:**
   - Stores verifier in a cookie for retrieval during callback

3. **Redirect to Authorization:**
   - Redirects user to Genesys Cloud OAuth authorization endpoint
   - Includes code challenge and PKCE method in the request

4. **Handle Callback:**
   - Extracts access token from URL hash after OAuth callback
   - Stores token in localStorage
   - Uses Bearer token for API requests

**Session Management:**
- Access tokens stored in `localStorage` (not cookies due to API Gateway limitations)
- Tokens sent via `Authorization: Bearer` header for API requests
- Tokens expire after 1 hour (3600 seconds)

### Backend (callbacklambdafunction)

The OAuth callback Lambda handles the token exchange:

1. Receives authorization code from Genesys Cloud
2. Retrieves PKCE code verifier from cookie
3. Exchanges authorization code for access token using PKCE
4. Returns access token to frontend via URL hash redirect

### Report Lambda (reportlambdafunction)

The report Lambda validates authentication:

1. Reads `Authorization` header from requests
2. Validates Bearer token format
3. Verifies token with Genesys Cloud API
4. Checks organization ID matches expected value

## Troubleshooting

### "Authentication Required" after login

**Cause:** Token not stored in localStorage

**Solution:**
1. Check browser console for JavaScript errors
2. Verify URL hash contains `#access_token=...` after callback
3. Clear localStorage and try again: `localStorage.clear()`

### "Missing authorization code" error

**Cause:** Redirect URI mismatch in Genesys Cloud

**Solution:**
1. Verify redirect URI in Genesys Cloud OAuth client exactly matches:
   `https://<API_GATEWAY_URL>/deploy/callback`
2. No trailing slash, no extra spaces
3. Ensure Code Authorization grant type is enabled

### 401 Unauthorized on data API

**Cause:** Token expired or invalid

**Solution:**
1. Clear localStorage: `localStorage.clear()`
2. Log in again to get fresh token
3. Check token expiration (tokens expire after 1 hour)

### Backend Lambda functions failing

**Cause:** Client Credentials OAuth not configured

**Solution:**
1. Verify AWS Secrets Manager secret exists: `user-activity-monitor-client-credentials`
2. Verify secret contains correct `clientId` and `clientSecret`
3. Ensure Client Credentials grant type is enabled on backend OAuth client
4. Check Lambda has IAM permissions to read from Secrets Manager

## Security Best Practices

### ✅ DO:
- Store backend credentials in AWS Secrets Manager
- Use environment-specific OAuth clients (dev, prod)
- Rotate client secrets regularly
- Use HTTPS for all communications
- Limit OAuth client permissions to minimum required

### ❌ DON'T:
- Commit credentials to git
- Share client secrets in plain text
- Use production credentials in development
- Expose client secrets in logs or error messages

## References

- [Genesys Cloud OAuth Documentation](https://developer.genesys.cloud/authorization/platform-auth/)
- [OAuth 2.0 PKCE Specification (RFC 7636)](https://tools.ietf.org/html/rfc7636)
- [AWS Lambda Proxy Integration](https://docs.aws.amazon.com/apigateway/latest/developerguide/set-up-lambda-proxy-integrations.html)
- [AWS Secrets Manager](https://docs.aws.amazon.com/secretsmanager/)
